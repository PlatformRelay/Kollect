// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"encoding/json"
	"fmt"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// ValidateClusterWire ensures wire metadata matches the report body (ADR-0028 queue + HTTP ingest).
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

// ReceiveReport unmarshals payload, validates wire cluster metadata, and merges into the store.
func ReceiveReport(wireCluster string, payload []byte, merger *Merger) (SpokeReport, int, error) {
	if merger == nil {
		return SpokeReport{}, 0, fmt.Errorf("hub receive: merger is nil")
	}

	var report SpokeReport
	if err := json.Unmarshal(payload, &report); err != nil {
		return SpokeReport{}, 0, fmt.Errorf("hub receive: unmarshal report: %w", err)
	}

	if report.APIVersion == "" {
		report.APIVersion = reportAPIVersion
	}

	if report.Cluster == "" && strings.TrimSpace(wireCluster) != "" {
		report.Cluster = strings.TrimSpace(wireCluster)
	}

	if err := ValidateClusterWire(wireCluster, report); err != nil {
		return SpokeReport{}, 0, err
	}

	applied, err := merger.Apply(report)
	if err != nil {
		return SpokeReport{}, 0, err
	}

	return report, applied, nil
}
