// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/pathvalidate"
	"github.com/konih/kollect/internal/sink/cap"
)

const connectTimeout = 30 * time.Second

// Backend upserts inventory rows into BigQuery.
type Backend struct {
	cfg      Config
	client   *bigquery.Client
	executor queryExecutor
}

type mergeRow struct {
	InventoryNamespace string `bigquery:"inventory_namespace"`
	InventoryName      string `bigquery:"inventory_name"`
	Cluster            string `bigquery:"cluster"`
	TargetName         string `bigquery:"target_name"`
	SourceUID          string `bigquery:"source_uid"`
	ResourceNamespace  string `bigquery:"resource_namespace"`
	PayloadJSON        string `bigquery:"payload_json"`
}

type queryExecutor interface {
	Execute(ctx context.Context, statement string, params []bigquery.QueryParameter, location string) error
}

type clientQueryExecutor struct {
	client *bigquery.Client
}

// NewBackend constructs a bigquery sink backend.
func NewBackend(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	databaseSecret map[string][]byte,
) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec, databaseSecret)
	if err != nil {
		return nil, err
	}

	connectCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		connectCtx, cancel = context.WithTimeout(ctx, connectTimeout)
		defer cancel()
	}

	clientOpts, err := cfg.clientOptions(connectCtx)
	if err != nil {
		return nil, classifyError(fmt.Errorf("bigquery client options: %w", err))
	}

	client, err := bigquery.NewClient(connectCtx, cfg.Project, clientOpts...)
	if err != nil {
		return nil, classifyError(fmt.Errorf("bigquery connect: %w", err))
	}

	b := &Backend{
		cfg:      cfg,
		client:   client,
		executor: clientQueryExecutor{client: client},
	}

	// ensureTable runs once when the backend is constructed; pooled backends reuse the same
	// instance so DDL is not repeated on every export (PERF-02).
	if cfg.ProvisioningMode == kollectdevv1alpha1.ProvisioningModeExisting {
		err = b.verifyTable(connectCtx)
	} else {
		err = b.ensureTable(connectCtx)
	}
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return b, nil
}

// Type returns the sink type identifier.
func (b *Backend) Type() string {
	return typeName
}

// Capabilities reports relational upsert with delete reconciliation (ADR-0401).
func (b *Backend) Capabilities() cap.Capabilities {
	return cap.RelationalStore()
}

// Close releases the BigQuery client.
func (b *Backend) Close() {
	if b.client != nil {
		_ = b.client.Close()
	}
}

// Export upserts each inventory item keyed by inventory, target, and source UID,
// then deletes rows absent from the current snapshot (ADR-0401 delete reconciliation).
func (b *Backend) Export(ctx context.Context, payload []byte, objectPath string) error {
	items, err := collect.ItemsFromExportPayload(payload)
	if err != nil {
		return classifyError(fmt.Errorf("bigquery export: decode payload: %w", err))
	}

	invNS, invName := pathvalidate.InventoryFromObjectPath(objectPath)
	cluster := b.cfg.Cluster

	if len(items) == 0 {
		return b.runDeleteAll(ctx, invNS, invName, cluster)
	}

	rows, err := toMergeRows(items, invNS, invName, cluster)
	if err != nil {
		return classifyError(fmt.Errorf("bigquery export: build rows: %w", err))
	}

	if b.cfg.UseEmulator {
		return b.runEmulatorReplace(ctx, invNS, invName, cluster, rows)
	}

	if err := b.runMergeUpsert(ctx, rows); err != nil {
		return err
	}

	return b.runDeleteStale(ctx, invNS, invName, cluster, rows)
}

func (b *Backend) verifyTable(ctx context.Context) error {
	_, err := b.client.Dataset(b.cfg.Dataset).Table(b.cfg.Table).Metadata(ctx)
	if err != nil {
		return classifyError(fmt.Errorf(
			"bigquery verify table: %s.%s does not exist (provisioning.mode=existing): %w",
			b.cfg.Dataset,
			b.cfg.Table,
			err,
		))
	}

	return nil
}

func (b *Backend) ensureTable(ctx context.Context) error {
	table := b.client.Dataset(b.cfg.Dataset).Table(b.cfg.Table)
	metadata := &bigquery.TableMetadata{
		Schema: bigquery.Schema{
			{Name: "inventory_namespace", Type: bigquery.StringFieldType, Required: true},
			{Name: "inventory_name", Type: bigquery.StringFieldType, Required: true},
			{Name: "target_name", Type: bigquery.StringFieldType, Required: true},
			{Name: "source_uid", Type: bigquery.StringFieldType, Required: true},
			{Name: "cluster", Type: bigquery.StringFieldType, Required: true},
			{Name: "resource_namespace", Type: bigquery.StringFieldType, Required: true},
			{Name: "payload", Type: bigquery.JSONFieldType, Required: true},
			{Name: "exported_at", Type: bigquery.TimestampFieldType, Required: true},
		},
		TimePartitioning: &bigquery.TimePartitioning{
			Type:  bigquery.DayPartitioningType,
			Field: "exported_at",
		},
		Clustering: &bigquery.Clustering{
			Fields: []string{"cluster", "inventory_namespace", "inventory_name", "resource_namespace"},
		},
	}

	err := table.Create(ctx, metadata)
	if err != nil && !isDuplicateCreate(err) {
		return classifyError(fmt.Errorf("bigquery ensure table: %w", err))
	}

	return nil
}

func (b *Backend) runMergeUpsert(ctx context.Context, rows []mergeRow) error {
	sourceRows := mergeSourceRowsSQL(rows)
	statement := fmt.Sprintf(`
MERGE %s AS t
USING %s AS s
ON t.inventory_namespace = s.inventory_namespace
 AND t.inventory_name = s.inventory_name
 AND t.cluster = s.cluster
 AND t.target_name = s.target_name
 AND t.source_uid = s.source_uid
WHEN MATCHED THEN UPDATE SET
  payload = PARSE_JSON(s.payload_json),
  exported_at = @exported_at,
  resource_namespace = s.resource_namespace
WHEN NOT MATCHED BY TARGET THEN
  INSERT (
    inventory_namespace,
    inventory_name,
    target_name,
    source_uid,
    cluster,
    resource_namespace,
    payload,
    exported_at
  )
  VALUES (
    s.inventory_namespace,
    s.inventory_name,
    s.target_name,
    s.source_uid,
    s.cluster,
    s.resource_namespace,
    PARSE_JSON(s.payload_json),
    @exported_at
  )
`, qualifiedTable(b.cfg.Project, b.cfg.Dataset, b.cfg.Table), sourceRows)
	params := []bigquery.QueryParameter{
		{Name: "exported_at", Value: time.Now().UTC()},
	}
	if err := b.executeQuery(ctx, statement, params); err != nil {
		return classifyError(fmt.Errorf("bigquery merge upsert: %w", err))
	}

	return nil
}

func (b *Backend) runDeleteStale(
	ctx context.Context,
	invNS, invName, cluster string,
	rows []mergeRow,
) error {
	sourceRows := mergeSourceRowsSQL(rows)
	statement := fmt.Sprintf(`
DELETE FROM %s AS t
WHERE t.inventory_namespace = @inv_ns
  AND t.inventory_name = @inv_name
  AND t.cluster = @cluster
  AND NOT EXISTS (
    SELECT 1 FROM %s AS s
    WHERE s.target_name = t.target_name
      AND s.source_uid = t.source_uid
  )
`, qualifiedTable(b.cfg.Project, b.cfg.Dataset, b.cfg.Table), sourceRows)
	params := []bigquery.QueryParameter{
		{Name: "inv_ns", Value: invNS},
		{Name: "inv_name", Value: invName},
		{Name: "cluster", Value: cluster},
	}
	if err := b.executeQuery(ctx, statement, params); err != nil {
		return classifyError(fmt.Errorf("bigquery delete stale: %w", err))
	}

	return nil
}

func (b *Backend) runDeleteAll(ctx context.Context, invNS, invName, cluster string) error {
	statement := fmt.Sprintf(`
DELETE FROM %s
WHERE inventory_namespace = @inv_ns
  AND inventory_name = @inv_name
  AND cluster = @cluster
`, qualifiedTable(b.cfg.Project, b.cfg.Dataset, b.cfg.Table))
	params := []bigquery.QueryParameter{
		{Name: "inv_ns", Value: invNS},
		{Name: "inv_name", Value: invName},
		{Name: "cluster", Value: cluster},
	}
	if err := b.executeQuery(ctx, statement, params); err != nil {
		return classifyError(fmt.Errorf("bigquery delete: %w", err))
	}

	return nil
}

func toMergeRows(items []collect.Item, invNS, invName, cluster string) ([]mergeRow, error) {
	rows := make([]mergeRow, 0, len(items))
	for _, item := range items {
		payload, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("marshal item %s: %w", item.UID, err)
		}

		resourceNS := strings.TrimSpace(item.Namespace)
		if resourceNS == "" {
			resourceNS = invNS
		}

		rows = append(rows, mergeRow{
			InventoryNamespace: invNS,
			InventoryName:      invName,
			Cluster:            cluster,
			TargetName:         item.TargetName,
			SourceUID:          item.UID,
			ResourceNamespace:  resourceNS,
			PayloadJSON:        string(payload),
		})
	}

	return rows, nil
}

func qualifiedTable(project, dataset, table string) string {
	escape := func(v string) string {
		return strings.ReplaceAll(v, "`", "")
	}

	return fmt.Sprintf("`%s.%s.%s`", escape(project), escape(dataset), escape(table))
}

func mergeSourceRowsSQL(rows []mergeRow) string {
	selects := make([]string, 0, len(rows))
	for _, row := range rows {
		selects = append(selects, fmt.Sprintf(
			"SELECT %s AS inventory_namespace, %s AS inventory_name, %s AS cluster, %s AS target_name, %s AS source_uid, %s AS resource_namespace, %s AS payload_json",
			sqlStringLiteral(row.InventoryNamespace),
			sqlStringLiteral(row.InventoryName),
			sqlStringLiteral(row.Cluster),
			sqlStringLiteral(row.TargetName),
			sqlStringLiteral(row.SourceUID),
			sqlStringLiteral(row.ResourceNamespace),
			sqlStringLiteral(row.PayloadJSON),
		))
	}

	return "(" + strings.Join(selects, " UNION ALL ") + ")"
}

func sqlStringLiteral(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

func usingEmulator() bool {
	return strings.TrimSpace(os.Getenv("BIGQUERY_EMULATOR_HOST")) != ""
}

func (b *Backend) runEmulatorReplace(
	ctx context.Context,
	invNS, invName, cluster string,
	rows []mergeRow,
) error {
	if err := b.runDeleteAll(ctx, invNS, invName, cluster); err != nil {
		return err
	}

	sourceRows := mergeSourceRowsSQL(rows)
	statement := fmt.Sprintf(`
INSERT INTO %s (
  inventory_namespace,
  inventory_name,
  target_name,
  source_uid,
  cluster,
  resource_namespace,
  payload,
  exported_at
)
SELECT
  s.inventory_namespace,
  s.inventory_name,
  s.target_name,
  s.source_uid,
  s.cluster,
  s.resource_namespace,
  PARSE_JSON(s.payload_json),
  @exported_at
FROM %s AS s
`, qualifiedTable(b.cfg.Project, b.cfg.Dataset, b.cfg.Table), sourceRows)
	params := []bigquery.QueryParameter{
		{Name: "exported_at", Value: time.Now().UTC()},
	}
	if err := b.executeQuery(ctx, statement, params); err != nil {
		return classifyError(fmt.Errorf("bigquery emulator insert: %w", err))
	}

	return nil
}

func (b *Backend) executeQuery(ctx context.Context, statement string, params []bigquery.QueryParameter) error {
	executor := b.executor
	if executor == nil {
		executor = clientQueryExecutor{client: b.client}
	}

	return executor.Execute(ctx, statement, params, b.cfg.Location)
}

func (e clientQueryExecutor) Execute(
	ctx context.Context,
	statement string,
	params []bigquery.QueryParameter,
	location string,
) error {
	query := e.client.Query(statement)
	query.Parameters = params
	if location != "" {
		query.Location = location
	}

	job, err := query.Run(ctx)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	if statusErr := status.Err(); statusErr != nil {
		return fmt.Errorf("status: %w", statusErr)
	}

	return nil
}
