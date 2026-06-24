// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package scope

import (
	"fmt"
	"slices"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const (
	ReasonScopeDeniedNamespace = "ScopeDeniedNamespace"
)

// ValidateDeniedNamespaces returns a violation when any namespace is in scope deniedNamespaces.
func ValidateDeniedNamespaces(scope *kollectdevv1alpha1.KollectScope, namespaces []string) error {
	if scope == nil || len(scope.Spec.DeniedNamespaces) == 0 {
		return nil
	}

	denied := make(map[string]struct{}, len(scope.Spec.DeniedNamespaces))
	for _, ns := range scope.Spec.DeniedNamespaces {
		denied[ns] = struct{}{}
	}

	for _, ns := range namespaces {
		if _, ok := denied[ns]; ok {
			return fmt.Errorf(
				"workload namespace %q is in KollectScope %q deniedNamespaces",
				ns, scope.Name,
			)
		}
	}

	return nil
}

// ValidateTargetIncludedNamespaces ensures Target intent namespaces are within Scope allow and not denied.
func ValidateTargetIncludedNamespaces(scope *kollectdevv1alpha1.KollectScope, namespaces []string) error {
	if scope == nil {
		return nil
	}

	if err := ValidateDeniedNamespaces(scope, namespaces); err != nil {
		return err
	}

	return ValidateWorkloadNamespaces(scope, namespaces)
}

// ValidateResourceRuleGVKs returns a violation when allowedGVKs is non-empty and any GVK is not listed.
func ValidateResourceRuleGVKs(
	scope *kollectdevv1alpha1.KollectScope,
	gvks []kollectdevv1alpha1.GroupVersionKind,
) error {
	if scope == nil || len(scope.Spec.AllowedGVKs) == 0 {
		return nil
	}

	for _, gvk := range gvks {
		if err := ValidateTargetGVK(scope, gvk); err != nil {
			return err
		}
	}

	return nil
}

// ValidateClusterScopeDeniedNamespaces checks namespaces against cluster scope deny list.
func ValidateClusterScopeDeniedNamespaces(scope *kollectdevv1alpha1.KollectClusterScope, namespaces []string) error {
	if scope == nil || len(scope.Spec.DeniedNamespaces) == 0 {
		return nil
	}

	denied := make(map[string]struct{}, len(scope.Spec.DeniedNamespaces))
	for _, ns := range scope.Spec.DeniedNamespaces {
		denied[ns] = struct{}{}
	}

	for _, ns := range namespaces {
		if _, ok := denied[ns]; ok {
			return fmt.Errorf(
				"workload namespace %q is in KollectClusterScope %q deniedNamespaces",
				ns, scope.Name,
			)
		}
	}

	return nil
}

// ValidateClusterScopeNamespaces ensures namespaces are within cluster scope allow and not denied.
func ValidateClusterScopeNamespaces(scope *kollectdevv1alpha1.KollectClusterScope, namespaces []string) error {
	if scope == nil {
		return nil
	}

	if err := ValidateClusterScopeDeniedNamespaces(scope, namespaces); err != nil {
		return err
	}

	if len(scope.Spec.AllowedNamespaces) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(scope.Spec.AllowedNamespaces))
	for _, ns := range scope.Spec.AllowedNamespaces {
		allowed[ns] = struct{}{}
	}

	for _, ns := range namespaces {
		if _, ok := allowed[ns]; !ok {
			return fmt.Errorf(
				"workload namespace %q is not in KollectClusterScope %q allowedNamespaces",
				ns, scope.Name,
			)
		}
	}

	return nil
}

// ValidateClusterScopeGVKs validates GVKs against cluster scope allowedGVKs.
func ValidateClusterScopeGVKs(
	scope *kollectdevv1alpha1.KollectClusterScope,
	gvk kollectdevv1alpha1.GroupVersionKind,
) error {
	if scope == nil || len(scope.Spec.AllowedGVKs) == 0 {
		return nil
	}

	for _, allowed := range scope.Spec.AllowedGVKs {
		if gvkMatches(allowed, gvk) {
			return nil
		}
	}

	return fmt.Errorf(
		"target GVK %s/%s/%s is not in KollectClusterScope %q allowedGVKs",
		gvk.Group, gvk.Version, gvk.Kind, scope.Name,
	)
}

// CollectRuleGVKs returns GVKs declared in resourceRules, or profile GVK when rules are empty.
func CollectRuleGVKs(
	filter kollectdevv1alpha1.CollectionFilterSpec,
	profileGVK kollectdevv1alpha1.GroupVersionKind,
) []kollectdevv1alpha1.GroupVersionKind {
	if len(filter.ResourceRules) == 0 {
		return []kollectdevv1alpha1.GroupVersionKind{profileGVK}
	}

	gvks := make([]kollectdevv1alpha1.GroupVersionKind, 0, len(filter.ResourceRules))
	for _, rule := range filter.ResourceRules {
		gvks = append(gvks, rule.GVK)
	}

	return gvks
}

// NormalizeNamespaceList trims and deduplicates namespace strings for validation.
func NormalizeNamespaceList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}

// ValidateClusterScopeStaticRefNamespace ensures a profile or sink namespace is allowed when
// allowedStaticRefNamespaces is set on KollectClusterScope (ADR-0208).
func ValidateClusterScopeStaticRefNamespace(scope *kollectdevv1alpha1.KollectClusterScope, ns string) error {
	if scope == nil || len(scope.Spec.AllowedStaticRefNamespaces) == 0 {
		return nil
	}

	ns = strings.TrimSpace(ns)
	for _, allowed := range scope.Spec.AllowedStaticRefNamespaces {
		if allowed == ns {
			return nil
		}
	}

	return fmt.Errorf(
		"static ref namespace %q is not in KollectClusterScope %q allowedStaticRefNamespaces",
		ns, scope.Name,
	)
}

// ValidateClusterInventoryClusterScopeSinkRefs checks family sink name allowlists on cluster scope.
func ValidateClusterInventoryClusterScopeSinkRefs(
	scope *kollectdevv1alpha1.KollectClusterScope,
	bindings []kollectdevv1alpha1.InventorySinkBinding,
) error {
	if scope == nil {
		return nil
	}

	for _, binding := range bindings {
		var allowed []string
		switch binding.Family {
		case kollectdevv1alpha1.SinkFamilySnapshot:
			allowed = scope.Spec.SnapshotSinkRefs
		case kollectdevv1alpha1.SinkFamilyDatabase:
			allowed = scope.Spec.DatabaseSinkRefs
		case kollectdevv1alpha1.SinkFamilyEvent:
			allowed = scope.Spec.EventSinkRefs
		default:
			continue
		}
		if len(allowed) == 0 {
			continue
		}
		if err := validateRefInAllowlist(binding.Name, allowed, binding.Family, scope.Name); err != nil {
			return err
		}
	}

	return nil
}

// ClusterInventoryStaticRefNamespaces returns distinct namespaces referenced by a cluster inventory.
func ClusterInventoryStaticRefNamespaces(spec *kollectdevv1alpha1.KollectClusterInventorySpec, defaultSinkNS string) []string {
	if spec == nil {
		return nil
	}

	defaultSinkNS = strings.TrimSpace(defaultSinkNS)
	seen := make(map[string]struct{})
	if spec.ProfileRef != nil {
		if ns := strings.TrimSpace(spec.ProfileRef.Namespace); ns != "" {
			seen[ns] = struct{}{}
		}
	}

	for _, binding := range kollectdevv1alpha1.CollectClusterInventorySinkBindings(spec) {
		ns := strings.TrimSpace(binding.Ref.Namespace)
		if ns == "" {
			ns = defaultSinkNS
		}
		if ns != "" {
			seen[ns] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for ns := range seen {
		out = append(out, ns)
	}

	slices.Sort(out)

	return out
}
