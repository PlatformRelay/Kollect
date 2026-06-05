// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CollectionFilterSpec holds target-side collection intent (ADR-0207).
// Namespace fields combine with AND semantics: includedNamespaces (when set) ∩ namespaceSelector.
type CollectionFilterSpec struct {
	// includedNamespaces is a static namespace allowlist. Empty means no extra restriction
	// beyond namespaceSelector.
	// +listType=set
	// +optional
	IncludedNamespaces []string `json:"includedNamespaces,omitempty"`

	// excludedNamespaces is a static namespace denylist applied after include logic.
	// +listType=set
	// +optional
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty"`

	// namespaceExcludeSelector excludes namespaces whose labels match the selector.
	// +optional
	NamespaceExcludeSelector *metav1.LabelSelector `json:"namespaceExcludeSelector,omitempty"`

	// resourceRules declares GVK-scoped collection rules. When empty, collection falls back to
	// Profile targetGVK plus legacy labelSelector/names on the Target.
	// +listType=atomic
	// +optional
	ResourceRules []ResourceRule `json:"resourceRules,omitempty"`
}

// ResourceRule selects objects of a GVK within optional namespace and resource label constraints.
// Multiple rules are OR-unioned (D7). Optional matchPolicy is a CEL expression evaluated pre-store.
type ResourceRule struct {
	// gvk is the API group, version, and kind this rule applies to.
	// +required
	GVK GroupVersionKind `json:"gvk"`

	// namespaceSelector further restricts this rule to namespaces matching the selector.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// matchLabels requires resource metadata labels to match (AND with matchExpressions).
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// matchExpressions requires resource metadata labels to match.
	// +optional
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`

	// matchPolicy is an optional CEL expression with `object` bound to the resource.
	// Evaluated after label/GVK/namespace gates, before store insert (ADR-0207 Phase 3).
	// +optional
	MatchPolicy string `json:"matchPolicy,omitempty"`
}

// CollectionFilterStatus summarizes applied namespace and rule filters on a Target.
type CollectionFilterStatus struct {
	// matchedNamespaces lists workload namespaces matched by Target intent (before Scope ceiling).
	// +listType=set
	// +optional
	MatchedNamespaces []string `json:"matchedNamespaces,omitempty"`

	// effectiveNamespaces lists namespaces after intersecting Scope allowedNamespaces and subtracting
	// Target/Scope deny lists.
	// +listType=set
	// +optional
	EffectiveNamespaces []string `json:"effectiveNamespaces,omitempty"`

	// activeResourceRules is the number of compiled resourceRules entries (0 when using legacy fallback).
	// +optional
	ActiveResourceRules int `json:"activeResourceRules,omitempty"`
}

// ScopeCeilingSpec is the tenancy guardrail subset shared by KollectScope and KollectClusterScope.
type ScopeCeilingSpec struct {
	// allowedGVKs restricts which target resource kinds may be collected in this scope.
	// +listType=atomic
	// +optional
	AllowedGVKs []GroupVersionKind `json:"allowedGVKs,omitempty"`

	// allowedNamespaces restricts which workload namespaces may be collected.
	// Empty means any namespace allowed by targets in the scope namespace.
	// +listType=set
	// +optional
	AllowedNamespaces []string `json:"allowedNamespaces,omitempty"`

	// deniedNamespaces is a platform blacklist; Target intent cannot override (D8).
	// +listType=set
	// +optional
	DeniedNamespaces []string `json:"deniedNamespaces,omitempty"`

	// minExportInterval is a tenancy floor — inventory and sink intervals below this value are rejected.
	// +optional
	MinExportInterval *metav1.Duration `json:"minExportInterval,omitempty"`
}
