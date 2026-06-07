// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// Export modes for KollectProfile / KollectClusterProfile (ADR-0306).
const (
	// ExportModeAttributes is the default: only spec.attributes[] are extracted.
	ExportModeAttributes = "Attributes"
	// ExportModeResource serializes a pruned copy of the target object into export.
	ExportModeResource = "Resource"
)

// Export include selectors choosing which top-level object sections survive pruning (ADR-0306).
const (
	ExportIncludeMetadataOnly  = "MetadataOnly"
	ExportIncludeSpecOnly      = "SpecOnly"
	ExportIncludeStatusOnly    = "StatusOnly"
	ExportIncludeSpecAndStatus = "SpecAndStatus"
	ExportIncludeAll           = "All"
)

// DefaultExportAs is the default attribute key for the embedded pruned object.
const DefaultExportAs = "resource"

// AllowFullResourceExportAnnotation opts a Profile into full-object export for
// sensitive kinds (Secret) when export.mode is Resource (ADR-0306 §Security).
//
//nolint:gosec // G101: annotation key name, not a credential
const AllowFullResourceExportAnnotation = "kollect.dev/allow-full-resource-export"

// ExportSpec configures full-resource export with path pruning (ADR-0306).
//
// When Mode is Resource, the collector serializes a pruned copy of the informer
// object into Item.attributes[As] instead of (or alongside) hand-picked attributes.
type ExportSpec struct {
	// mode selects between curated attribute extraction and full-resource export.
	// Attributes (default) keeps current behaviour; Resource embeds a pruned object.
	// +kubebuilder:validation:Enum=Attributes;Resource
	// +kubebuilder:default=Attributes
	// +optional
	Mode string `json:"mode,omitempty"`

	// as is the attribute key under which the embedded pruned object is stored.
	// +kubebuilder:default=resource
	// +optional
	As string `json:"as,omitempty"`

	// include selects which top-level object sections survive pruning.
	// +kubebuilder:validation:Enum=MetadataOnly;SpecOnly;StatusOnly;SpecAndStatus;All
	// +kubebuilder:default=SpecAndStatus
	// +optional
	Include string `json:"include,omitempty"`

	// dedupeIdentity strips identity fields already carried on the Item envelope
	// (uid, namespace, name, GVK) from the embedded object to avoid triple storage.
	// Defaults to true.
	// +optional
	DedupeIdentity *bool `json:"dedupeIdentity,omitempty"`

	// prune declares path-based exclusions applied to the embedded object.
	// +optional
	Prune *PruneSpec `json:"prune,omitempty"`
}

// PruneSpec declares path-based exclusions for full-resource export (ADR-0306).
type PruneSpec struct {
	// defaults applies built-in noise exclusions (managedFields, resourceVersion,
	// generation, last-applied-configuration, argo tracking-id). Defaults to true.
	// +optional
	Defaults *bool `json:"defaults,omitempty"`

	// jsonPointers lists RFC 6901 pointers to drop, matching Argo CD ignoreDifferences UX.
	// +optional
	JSONPointers []string `json:"jsonPointers,omitempty"`

	// jsonPaths lists kubectl/JSONPath expressions to drop (warn-only parse in Phase 1).
	// +optional
	JSONPaths []string `json:"jsonPaths,omitempty"`

	// scrubKeys lists case-insensitive key names redacted at any depth, merging with
	// the operator scrubKeys denylist (ADR-0303).
	// +optional
	ScrubKeys []string `json:"scrubKeys,omitempty"`

	// cel lists CEL predicates evaluated against the object; true drops the matched
	// value. Reserved for Phase 2 — not yet enforced by the collector.
	// +optional
	CEL []string `json:"cel,omitempty"`
}

// ResourceExportEnabled reports whether the spec opts into full-resource export.
func (e *ExportSpec) ResourceExportEnabled() bool {
	return e != nil && e.Mode == ExportModeResource
}

// AttributeKey returns the configured embedded-object key, defaulting to "resource".
func (e *ExportSpec) AttributeKey() string {
	if e == nil || e.As == "" {
		return DefaultExportAs
	}

	return e.As
}

// IncludeOrDefault returns the configured include selector, defaulting to SpecAndStatus.
func (e *ExportSpec) IncludeOrDefault() string {
	if e == nil || e.Include == "" {
		return ExportIncludeSpecAndStatus
	}

	return e.Include
}

// DedupeIdentityEnabled reports whether envelope identity fields are stripped (default true).
func (e *ExportSpec) DedupeIdentityEnabled() bool {
	if e == nil || e.DedupeIdentity == nil {
		return true
	}

	return *e.DedupeIdentity
}

// DefaultsEnabled reports whether built-in pruning defaults apply (default true).
func (p *PruneSpec) DefaultsEnabled() bool {
	if p == nil || p.Defaults == nil {
		return true
	}

	return *p.Defaults
}
