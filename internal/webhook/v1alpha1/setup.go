// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
)

// clusterKindRejectedInTenantMode builds the admission denial returned for
// cluster-scoped reconciled kinds when the operator runs in tenantMode. In
// tenantMode the operator only holds namespaced RBAC (Role/RoleBinding in the
// watched namespaces), so it cannot reconcile cluster-scoped kinds — reject
// them at admission rather than letting them degrade via the forbidden path at
// reconcile time (ADR-0208).
func clusterKindRejectedInTenantMode(kind, name string) error {
	return fmt.Errorf(
		"%s %q is not supported when the operator runs in tenantMode: cluster-scoped kinds "+
			"require cluster-wide RBAC. Install the operator with cluster RBAC or use the "+
			"namespaced KollectTarget/KollectInventory kinds instead (ADR-0208)",
		kind, name,
	)
}

// SetupWithManager registers validating webhooks with the manager. When
// tenantMode is true the cluster-scoped reconciled kinds are rejected at
// admission (see clusterKindRejectedInTenantMode).
func SetupWithManager(mgr ctrl.Manager, tenantMode bool) error {
	if err := setupKollectProfileWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectProfile webhook: %w", err)
	}

	if err := setupKollectScopeWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectScope webhook: %w", err)
	}

	if err := setupKollectClusterScopeWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectClusterScope webhook: %w", err)
	}

	if err := setupKollectTargetWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectTarget webhook: %w", err)
	}

	if err := setupKollectInventoryWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectInventory webhook: %w", err)
	}

	if err := setupKollectConnectionTestWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectConnectionTest webhook: %w", err)
	}

	if err := setupKollectClusterTargetWebhook(mgr, tenantMode); err != nil {
		return fmt.Errorf("setup KollectClusterTarget webhook: %w", err)
	}

	if err := setupKollectClusterInventoryWebhook(mgr, tenantMode); err != nil {
		return fmt.Errorf("setup KollectClusterInventory webhook: %w", err)
	}

	if err := setupFamilySinkWebhooks(mgr); err != nil {
		return fmt.Errorf("setup family sink webhooks: %w", err)
	}

	return nil
}
