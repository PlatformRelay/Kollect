// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import (
	"fmt"
	"os"
	"strings"
)

const (
	//nolint:gosec // G101: standard Kubernetes projected SA token mount path, not a secret value
	defaultSATokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	//nolint:gosec // G101: env var name for test override, not a credential
	envSpokeToken = "KOLLECT_SPOKE_TOKEN"
)

// serviceAccountToken returns the in-cluster service account token for hub push auth (ADR-0503).
// KOLLECT_SPOKE_TOKEN overrides the default mount path for tests and local dev.
func serviceAccountToken() (string, error) {
	if token := strings.TrimSpace(os.Getenv(envSpokeToken)); token != "" {
		return token, nil
	}

	data, err := os.ReadFile(defaultSATokenPath)
	if err != nil {
		return "", fmt.Errorf("read service account token: %w", err)
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("service account token is empty")
	}

	return token, nil
}
