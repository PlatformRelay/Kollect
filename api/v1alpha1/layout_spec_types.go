// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// Layout modes for snapshot Git/GitLab sinks (ADR-0419).
const (
	// LayoutModeDocument writes a single inventory file per export (default).
	LayoutModeDocument = "document"
	// LayoutModePerResource writes one file per Item at layout.pathTemplate.
	LayoutModePerResource = "perResource"
	// LayoutModeSplit writes an index sidecar plus a per-resource tree.
	LayoutModeSplit = "split"
)

// Layout content shapes controlling per-file payloads (ADR-0419).
const (
	// LayoutContentItem writes the full Item row (default).
	LayoutContentItem = "item"
	// LayoutContentAttributes writes only the Item.attributes map.
	LayoutContentAttributes = "attributes"
	// LayoutContentManifest writes the native Kubernetes object (ADR-0306 Resource export).
	LayoutContentManifest = "manifest"
)

// Filename group-in-path policies (ADR-0419).
const (
	LayoutGroupInPathAuto   = "auto"
	LayoutGroupInPathAlways = "always"
	LayoutGroupInPathNever  = "never"
)

// Layout defaults (ADR-0419).
const (
	// DefaultLayoutPathTemplate is the per-resource path used in perResource/split modes.
	DefaultLayoutPathTemplate = "{cluster}/{sourceNamespace}/{kind}/{sourceName}{extension}"
	// DefaultLayoutIndexPathTemplate is the default index sidecar path in split mode.
	DefaultLayoutIndexPathTemplate = "inventory/{namespace}/{name}{extension}"
	// DefaultLayoutMaxSegmentLength keeps path segments DNS-safe.
	DefaultLayoutMaxSegmentLength int32 = 63
)

// LayoutSpec configures document shape and folder layout for snapshot Git/GitLab sinks (ADR-0419).
//
// The entire block is optional: a Git sink needs only type + endpoint to produce readable YAML.
// When omitted the operator behaves as layout.mode=document.
type LayoutSpec struct {
	// mode selects the on-disk layout: document (one inventory file, default),
	// perResource (one file per Item), or split (index sidecar + per-resource tree).
	// +kubebuilder:validation:Enum=document;perResource;split
	// +kubebuilder:default=document
	// +optional
	Mode string `json:"mode,omitempty"`

	// content selects the per-file payload: item (full Item row, default),
	// attributes (Item.attributes only), or manifest (native object, ADR-0306 Resource export).
	// +kubebuilder:validation:Enum=item;attributes;manifest
	// +kubebuilder:default=item
	// +optional
	Content string `json:"content,omitempty"`

	// pathTemplate is the per-item path used when mode is perResource or split.
	// Placeholders: {cluster}, {namespace}, {name}, {targetNamespace}, {targetName},
	// {sourceNamespace}, {sourceName}, {group}, {kind}, {uid}, {generation}, {extension}.
	// Default: {cluster}/{sourceNamespace}/{kind}/{sourceName}{extension}
	// +optional
	PathTemplate string `json:"pathTemplate,omitempty"`

	// index configures the split-mode index sidecar.
	// +optional
	Index *LayoutIndexSpec `json:"index,omitempty"`

	// filename tunes path-segment sanitization and grouping.
	// +optional
	Filename *LayoutFilenameSpec `json:"filename,omitempty"`
}

// LayoutIndexSpec configures the split-mode index sidecar (ADR-0419).
type LayoutIndexSpec struct {
	// enabled toggles the index sidecar (default false; true in split mode).
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// pathTemplate is the index file path (default inventory/{namespace}/{name}{extension}).
	// +optional
	PathTemplate string `json:"pathTemplate,omitempty"`
}

// LayoutFilenameSpec tunes per-resource path segments (ADR-0419).
type LayoutFilenameSpec struct {
	// groupInPath controls the {group} segment: auto (omit for core types, default),
	// always, or never.
	// +kubebuilder:validation:Enum=auto;always;never
	// +optional
	GroupInPath string `json:"groupInPath,omitempty"`

	// lowercaseKind lowercases {kind} in paths (default true).
	// +optional
	LowercaseKind *bool `json:"lowercaseKind,omitempty"`

	// maxSegmentLength caps each path segment for DNS safety (default 63).
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxSegmentLength *int32 `json:"maxSegmentLength,omitempty"`
}

// ModeOrDefault returns the configured layout mode, defaulting to document.
func (l *LayoutSpec) ModeOrDefault() string {
	if l == nil || l.Mode == "" {
		return LayoutModeDocument
	}

	return l.Mode
}

// ModeExplicit reports whether layout.mode was set by the author.
func (l *LayoutSpec) ModeExplicit() bool {
	return l != nil && l.Mode != ""
}

// ContentExplicit reports whether layout.content was set by the author.
func (l *LayoutSpec) ContentExplicit() bool {
	return l != nil && l.Content != ""
}

// IsDocumentMode reports whether the resolved mode is document (single inventory file).
func (l *LayoutSpec) IsDocumentMode() bool {
	return l.ModeOrDefault() == LayoutModeDocument
}

// GroupInPathOrDefault returns the configured group policy, defaulting to auto.
func (f *LayoutFilenameSpec) GroupInPathOrDefault() string {
	if f == nil || f.GroupInPath == "" {
		return LayoutGroupInPathAuto
	}

	return f.GroupInPath
}

// LowercaseKindEnabled reports whether kind segments are lowercased (default true).
func (f *LayoutFilenameSpec) LowercaseKindEnabled() bool {
	if f == nil || f.LowercaseKind == nil {
		return true
	}

	return *f.LowercaseKind
}

// MaxSegmentLengthOrDefault returns the configured segment cap, defaulting to 63.
func (f *LayoutFilenameSpec) MaxSegmentLengthOrDefault() int32 {
	if f == nil || f.MaxSegmentLength == nil || *f.MaxSegmentLength <= 0 {
		return DefaultLayoutMaxSegmentLength
	}

	return *f.MaxSegmentLength
}

// IndexEnabled reports whether the index sidecar is on for the given mode (default split-only).
func (l *LayoutSpec) IndexEnabled() bool {
	if l != nil && l.Index != nil && l.Index.Enabled != nil {
		return *l.Index.Enabled
	}

	return l.ModeOrDefault() == LayoutModeSplit
}
