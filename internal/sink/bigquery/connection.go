// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package bigquery

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const probeTimeout = 15 * time.Second

// TestConnection checks credential resolution, dataset metadata, and dry-run job execution.
func TestConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	databaseSecret map[string][]byte,
) error {
	cfg, err := ConfigFromSpec(spec, databaseSecret)
	if err != nil {
		return err
	}

	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	clientOpts, err := cfg.clientOptions(probeCtx)
	if err != nil {
		return classifyError(fmt.Errorf("bigquery client options: %w", err))
	}

	client, err := bigquery.NewClient(probeCtx, cfg.Project, clientOpts...)
	if err != nil {
		return classifyError(fmt.Errorf("bigquery connect: %w", err))
	}
	defer func() { _ = client.Close() }()

	if _, err := client.Dataset(cfg.Dataset).Metadata(probeCtx); err != nil {
		return classifyError(fmt.Errorf("bigquery dataset metadata: %w", err))
	}

	query := client.Query("SELECT 1")
	query.DryRun = true
	if cfg.Location != "" {
		query.Location = cfg.Location
	}
	if _, err := query.Run(probeCtx); err != nil {
		return classifyError(fmt.Errorf("bigquery dry-run query: %w", err))
	}

	return nil
}
