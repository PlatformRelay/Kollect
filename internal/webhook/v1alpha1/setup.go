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

	return nil
}
