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
	// name is the family sink object name.
	// +required
	Name string `json:"name"`

	// namespace overrides where this sink resolves. Cluster inventory only:
	// when omitted it inherits spec.sinkNamespace. Forbidden on namespaced inventories,
	// which always resolve in the inventory's own namespace (ADR-0208).
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// exportMinInterval overrides the inventory default for this sink ref.
	// Zero means material-change only (no periodic re-export of identical payload).
	// +optional
	ExportMinInterval *metav1.Duration `json:"exportMinInterval,omitempty"`

	// maxExportBytes overrides the inventory-wide export size ceiling for this sink
	// ref (AR-01 / EC-P0-01). It may set a smaller or larger byte ceiling than
	// spec.maxExportBytes; when omitted the inventory-wide ceiling (or the operator
	// global cap) applies. The webhook rejects values above the operator global cap
	// (ADR-0103). Payloads exceeding this ceiling are split into multiple export parts.
	// +optional
	MaxExportBytes *int64 `json:"maxExportBytes,omitempty"`
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
	// name matches spec.sinkRefs[].name (keyed by family/name).
	// +required
	Name string `json:"name"`

	// namespace is the resolved namespace the sink was exported to (ADR-0208).
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// lastExportTime is when this sink last accepted an export successfully.
	// +optional
	LastExportTime *metav1.Time `json:"lastExportTime,omitempty"`

	// lastChecksum is the payload fingerprint from the last successful export.
	// +optional
	LastChecksum string `json:"lastChecksum,omitempty"`

	// lastExportPaths are the sink-relative paths the last successful export to
	// this sink actually wrote (document path, layout sidecars, projected
	// per-resource files). The deletion-time cleanup path reads them as
	// retraction evidence (K-28): recorded paths the cleanup attempt cannot
	// address — the sink's pathTemplate or serialization format changed, or the
	// export was tree-shaped — announce CleanupRetained instead of a silent
	// false-clean tombstone. A projection larger than the cap is sampled to its
	// first sorted entries.
	// +optional
	// +kubebuilder:validation:MaxItems=32
	LastExportPaths []string `json:"lastExportPaths,omitempty"`

	// conditions report per-sink Synced / debounce state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
