// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package inventory

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konih/kollect/internal/collect"
)

// ListFilter holds query parameters for inventory list endpoints.
type ListFilter struct {
	Namespace string
	Inventory string
	Target    string
	Group     string
	Version   string
	Kind      string
	Name      string
	Limit     int
	Offset    int
}

// Pagination describes a page of inventory items (ADR-0408).
type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

// ExportStatus reports export health for one sink referenced by an inventory.
type ExportStatus struct {
	SinkName       string `json:"sinkName"`
	SinkNamespace  string `json:"sinkNamespace,omitempty"`
	Status         string `json:"status"`
	LastExportTime string `json:"lastExportTime,omitempty"`
	Message        string `json:"message,omitempty"`
}

// InventorySummary is the versioned Read API envelope (ADR-0405, ADR-0408).
type InventorySummary struct {
	SchemaVersion string         `json:"schemaVersion"`
	ItemCount     int            `json:"itemCount"`
	Namespace     string         `json:"namespace,omitempty"`
	Inventory     string         `json:"inventory,omitempty"`
	Items         []collect.Item `json:"items"`
	UpdatedAt     string         `json:"updatedAt"`
	Pagination    *Pagination    `json:"pagination,omitempty"`
	ExportStatus  []ExportStatus `json:"exportStatus,omitempty"`
	Checksum      string         `json:"checksum,omitempty"`
}

// ResourceStatus is a CRD status projection for UI health views (B3).
type ResourceStatus struct {
	Name               string             `json:"name"`
	Namespace          string             `json:"namespace"`
	Generation         int64              `json:"generation,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	ItemCount          int                `json:"itemCount,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	LastExportTime     string             `json:"lastExportTime,omitempty"`
}

// StatusListResponse wraps CRD status rows in the Read API envelope.
type StatusListResponse struct {
	SchemaVersion string           `json:"schemaVersion"`
	Items         []ResourceStatus `json:"items"`
}
