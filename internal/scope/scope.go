// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package scope

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// Binding is the active KollectScope for a tenant namespace, if any.
type Binding struct {
	// Enforced is true when a KollectScope exists in the namespace and allowlists apply.
	Enforced bool
	Scope    *kollectdevv1alpha1.KollectScope
}

// Load returns the scope binding for namespace. When multiple scopes exist, the oldest by name is used.
func Load(ctx context.Context, c client.Client, namespace string) (Binding, error) {
	var list kollectdevv1alpha1.KollectScopeList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return Binding{}, fmt.Errorf("list KollectScope in namespace %q: %w", namespace, err)
	}

	if len(list.Items) == 0 {
		return Binding{}, nil
	}

	scope := list.Items[0]
	for i := 1; i < len(list.Items); i++ {
		if list.Items[i].Name < scope.Name {
			scope = list.Items[i]
		}
	}

	return Binding{Enforced: true, Scope: &scope}, nil
}

// ValidateTargetGVK returns a violation when allowedGVKs is non-empty and gvk is not listed.
func ValidateTargetGVK(scope *kollectdevv1alpha1.KollectScope, gvk kollectdevv1alpha1.GroupVersionKind) error {
	if scope == nil || len(scope.Spec.AllowedGVKs) == 0 {
		return nil
	}

	for _, allowed := range scope.Spec.AllowedGVKs {
		if gvkMatches(allowed, gvk) {
			return nil
		}
	}

	return fmt.Errorf(
		"target GVK %s/%s/%s is not in KollectScope %q allowedGVKs",
		gvk.Group, gvk.Version, gvk.Kind, scope.Name,
	)
}

// ValidateWorkloadNamespaces returns a violation when allowedNamespaces is non-empty and any
// namespace is outside the allowlist.
func ValidateWorkloadNamespaces(scope *kollectdevv1alpha1.KollectScope, namespaces []string) error {
	if scope == nil || len(scope.Spec.AllowedNamespaces) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(scope.Spec.AllowedNamespaces))
	for _, ns := range scope.Spec.AllowedNamespaces {
		allowed[ns] = struct{}{}
	}

	for _, ns := range namespaces {
		if _, ok := allowed[ns]; !ok {
			return fmt.Errorf(
				"workload namespace %q is not in KollectScope %q allowedNamespaces",
				ns, scope.Name,
			)
		}
	}

	return nil
}

// ValidateInventoryFamilySinkRefs returns a violation when family allowlists are non-empty and a ref is missing.
func ValidateInventoryFamilySinkRefs(scope *kollectdevv1alpha1.KollectScope, bindings []kollectdevv1alpha1.InventorySinkBinding) error {
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

func validateRefInAllowlist(name string, allowed []string, family, scopeName string) error {
	seen := make(map[string]struct{}, len(allowed))
	for _, ref := range allowed {
		seen[ref] = struct{}{}
	}
	if _, ok := seen[name]; !ok {
		return fmt.Errorf(
			"%s sink ref %q is not in KollectScope %q %sSinkRefs",
			family, name, scopeName, family,
		)
	}
	return nil
}

// ValidateSinkRefs is deprecated — use ValidateInventoryFamilySinkRefs (ADR-0414 clean break).
func ValidateSinkRefs(scope *kollectdevv1alpha1.KollectScope, refs []string) error {
	if scope == nil {
		return nil
	}
	bindings := make([]kollectdevv1alpha1.InventorySinkBinding, 0, len(refs))
	for _, ref := range refs {
		bindings = append(bindings, kollectdevv1alpha1.InventorySinkBinding{
			Name: ref, Family: kollectdevv1alpha1.SinkFamilySnapshot,
		})
	}
	return ValidateInventoryFamilySinkRefs(scope, bindings)
}

func gvkMatches(allowed, got kollectdevv1alpha1.GroupVersionKind) bool {
	return strings.EqualFold(allowed.Group, got.Group) &&
		strings.EqualFold(allowed.Version, got.Version) &&
		strings.EqualFold(allowed.Kind, got.Kind)
}
