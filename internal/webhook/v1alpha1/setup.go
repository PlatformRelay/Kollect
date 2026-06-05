// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupWithManager registers validating webhooks with the manager.
func SetupWithManager(mgr ctrl.Manager) error {
	if err := setupKollectProfileWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectProfile webhook: %w", err)
	}

	if err := setupKollectHubWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectHub webhook: %w", err)
	}

	if err := setupKollectScopeWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectScope webhook: %w", err)
	}

	if err := setupKollectRemoteClusterWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectRemoteCluster webhook: %w", err)
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

	if err := setupKollectClusterTargetWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectClusterTarget webhook: %w", err)
	}

	if err := setupKollectClusterInventoryWebhook(mgr); err != nil {
		return fmt.Errorf("setup KollectClusterInventory webhook: %w", err)
	}

	return nil
}
