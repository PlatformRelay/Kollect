// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ReasonDebounced indicates export was skipped because the payload is unchanged within the interval.
	ReasonDebounced = "Debounced"
	// ReasonPartiallySynced indicates some sinks exported while others are debounced.
	ReasonPartiallySynced = "PartiallySynced"
)

// InventorySinkRef names a KollectSink with an optional per-ref export interval override.
type InventorySinkRef struct {
	// name is the KollectSink object name in the same namespace as the inventory.
	// +required
	Name string `json:"name"`

	// exportMinInterval overrides the inventory default for this sink ref.
	// Zero means material-change only (no periodic re-export of identical payload).
	// +optional
	ExportMinInterval *metav1.Duration `json:"exportMinInterval,omitempty"`
}

// InventorySinkRefList accepts plain sink name strings or structured InventorySinkRef objects.
type InventorySinkRefList []InventorySinkRef

// UnmarshalJSON implements backward-compatible decoding for string or object sinkRefs entries.
func (l *InventorySinkRefList) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*l = nil
		return nil
	}

	var asStrings []string
	if err := json.Unmarshal(data, &asStrings); err == nil {
		out := make(InventorySinkRefList, 0, len(asStrings))
		for _, name := range asStrings {
			out = append(out, InventorySinkRef{Name: name})
		}
		*l = out
		return nil
	}

	var asObjects []InventorySinkRef
	if err := json.Unmarshal(data, &asObjects); err != nil {
		return fmt.Errorf("sinkRefs: expected string array or object array: %w", err)
	}
	*l = asObjects
	return nil
}

// MarshalJSON emits structured sink ref objects.
func (l InventorySinkRefList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("null"), nil
	}
	out := make([]InventorySinkRef, len(l))
	copy(out, l)
	return json.Marshal(out)
}

// Names returns sink names in list order.
func (l InventorySinkRefList) Names() []string {
	names := make([]string, 0, len(l))
	for _, ref := range l {
		if ref.Name != "" {
			names = append(names, ref.Name)
		}
	}
	return names
}

// NewSinkRefList builds refs from plain sink names.
func NewSinkRefList(names ...string) InventorySinkRefList {
	out := make(InventorySinkRefList, 0, len(names))
	for _, name := range names {
		out = append(out, InventorySinkRef{Name: name})
	}
	return out
}

// InventorySinkExportStatus holds per-sink export observation on an inventory.
type InventorySinkExportStatus struct {
	// name matches spec.sinkRefs[].name.
	// +required
	Name string `json:"name"`

	// lastExportTime is when this sink last accepted an export successfully.
	// +optional
	LastExportTime *metav1.Time `json:"lastExportTime,omitempty"`

	// lastChecksum is the payload fingerprint from the last successful export.
	// +optional
	LastChecksum string `json:"lastChecksum,omitempty"`

	// conditions report per-sink Synced / debounce state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
