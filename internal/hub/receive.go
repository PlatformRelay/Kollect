// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"encoding/json"
	"fmt"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
)

// ValidateClusterWire ensures wire metadata matches the report body (ADR-0503 queue + HTTP ingest).
func ValidateClusterWire(wireCluster string, report SpokeReport) error {
	bodyCluster := strings.TrimSpace(report.Cluster)
	wireCluster = strings.TrimSpace(wireCluster)

	if bodyCluster == "" {
		return fmt.Errorf("hub receive: report cluster is required")
	}

	if wireCluster == "" {
		return nil
	}

	if wireCluster != bodyCluster {
		return fmt.Errorf("hub receive: %s %q does not match report cluster %q",
			kollectdevv1alpha1.HeaderClusterID, wireCluster, bodyCluster)
	}

	return nil
}

// ReceiveReport unmarshals payload, validates wire cluster metadata and optional ACL,
// then merges into the store. prior captures the target bucket before merge so callers
// can roll back on export failure (EC-P0-03). When allowlistEnforced is false and
// allowedClusters is empty, any cluster is permitted (dev queues). When enforced,
// an empty allowlist rejects all.
func ReceiveReport(
	wireCluster string,
	payload []byte,
	merger *Merger,
	allowedClusters []string,
	allowlistEnforced bool,
) (SpokeReport, int, map[string]collect.Item, error) {
	if merger == nil {
		return SpokeReport{}, 0, nil, fmt.Errorf("hub receive: merger is nil")
	}

	var report SpokeReport
	if err := json.Unmarshal(payload, &report); err != nil {
		return SpokeReport{}, 0, nil, fmt.Errorf("hub receive: unmarshal report: %w", err)
	}

	NormalizeReport(&report)

	if err := export.ValidateSchemaVersion(report.SchemaVersion); err != nil {
		return SpokeReport{}, 0, nil, fmt.Errorf("hub receive: %w", err)
	}

	if report.Cluster == "" && strings.TrimSpace(wireCluster) != "" {
		report.Cluster = strings.TrimSpace(wireCluster)
	}

	if err := ValidateClusterWire(wireCluster, report); err != nil {
		return SpokeReport{}, 0, nil, err
	}

	if err := ValidateClusterACL(report.Cluster, allowedClusters, allowlistEnforced); err != nil {
		return SpokeReport{}, 0, nil, err
	}

	prior := merger.Store.CloneTargetItems(report.Cluster, inventoryTargetName(report.InventoryRef))

	applied, err := merger.Apply(report)
	if err != nil {
		return SpokeReport{}, 0, nil, err
	}

	return report, applied, prior, nil
}
