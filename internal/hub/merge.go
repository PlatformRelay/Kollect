// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"encoding/json"
	"fmt"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
)

const (
	defaultInventoryName  = "default"
	defaultHubMetricLabel = "default"
)

// InventoryRef identifies the spoke inventory resource backing a report.
type InventoryRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// SpokeReport is the spoke → hub inventory payload (ADR-0502 sketch).
type SpokeReport struct {
	APIVersion    string         `json:"apiVersion"`
	SchemaVersion string         `json:"schemaVersion"`
	Cluster       string         `json:"cluster"`
	InventoryRef  InventoryRef   `json:"inventoryRef"`
	Generation    int64          `json:"generation,omitempty"`
	Items         []collect.Item `json:"items,omitempty"`
	RemovedUIDs   []string       `json:"removedUIDs,omitempty"`
}

// NormalizeReport fills default apiVersion and schemaVersion on unmarshaled reports.
func NormalizeReport(report *SpokeReport) {
	if report == nil {
		return
	}

	if report.APIVersion == "" {
		report.APIVersion = export.WireAPIVersion
	}

	report.SchemaVersion = export.NormalizeSchemaVersion(report.SchemaVersion)
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
		targetName = defaultInventoryName
	}

	applied := 0
	for _, uid := range report.RemovedUIDs {
		m.Store.Remove(report.Cluster, targetName, uid)
		applied++
	}

	for _, item := range report.Items {
		item.TargetNamespace = report.Cluster
		item.TargetName = targetName
		m.Store.Upsert(item)
		applied++
	}

	return applied, nil
}

// ApplyJSON unmarshals payload and merges into the store.
// Callers that accept untrusted wire payloads should use ReceiveReport instead.
func (m *Merger) ApplyJSON(payload []byte) (int, error) {
	var report SpokeReport
	if err := json.Unmarshal(payload, &report); err != nil {
		return 0, fmt.Errorf("hub merger: unmarshal report: %w", err)
	}

	NormalizeReport(&report)

	if err := export.ValidateSchemaVersion(report.SchemaVersion); err != nil {
		return 0, fmt.Errorf("hub merger: %w", err)
	}

	return m.Apply(report)
}
