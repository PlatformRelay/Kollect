// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"encoding/json"
	"fmt"

	"github.com/konih/kollect/internal/collect"
)

const reportAPIVersion = "kollect.dev/v1alpha1"

// InventoryRef identifies the spoke inventory resource backing a report.
type InventoryRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// SpokeReport is the spoke → hub inventory payload (ADR-0023 sketch).
type SpokeReport struct {
	APIVersion   string         `json:"apiVersion"`
	Cluster      string         `json:"cluster"`
	InventoryRef InventoryRef   `json:"inventoryRef"`
	Generation   int64          `json:"generation,omitempty"`
	Items        []collect.Item `json:"items,omitempty"`
}

// Merger applies spoke reports into a hub-side collection store.
type Merger struct {
	Store *collect.Store
}

// NewMerger returns a merger writing into store.
func NewMerger(store *collect.Store) *Merger {
	return &Merger{Store: store}
}

// Apply merges report items idempotently on (cluster, namespace, name, uid).
func (m *Merger) Apply(report SpokeReport) (int, error) {
	if m == nil || m.Store == nil {
		return 0, fmt.Errorf("hub merger: store is nil")
	}

	if report.Cluster == "" {
		return 0, fmt.Errorf("hub merger: cluster is required")
	}

	targetName := report.InventoryRef.Name
	if targetName == "" {
		targetName = "default"
	}

	applied := 0
	for _, item := range report.Items {
		item.TargetNamespace = report.Cluster
		item.TargetName = targetName
		m.Store.Upsert(item)
		applied++
	}

	return applied, nil
}

// ApplyJSON unmarshals payload and merges into the store.
func (m *Merger) ApplyJSON(payload []byte) (int, error) {
	var report SpokeReport
	if err := json.Unmarshal(payload, &report); err != nil {
		return 0, fmt.Errorf("hub merger: unmarshal report: %w", err)
	}

	if report.APIVersion == "" {
		report.APIVersion = reportAPIVersion
	}

	return m.Apply(report)
}
