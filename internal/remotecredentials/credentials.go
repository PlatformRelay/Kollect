// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package remotecredentials loads Istio-style remote kubeconfig secrets (ADR-0503).
package remotecredentials

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const healthCheckTimeout = 5 * time.Second

// ExtractKubeconfig returns raw kubeconfig bytes from an Istio-style remote secret.
// The data key must match clusterName (same as remotesecret generator).
func ExtractKubeconfig(secret *corev1.Secret, clusterName string) ([]byte, error) {
	if secret == nil {
		return nil, fmt.Errorf("secret is nil")
	}

	clusterName = strings.TrimSpace(clusterName)
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name is required")
	}

	if secret.Data == nil {
		return nil, fmt.Errorf("secret %q has no data", secret.Name)
	}

	raw, ok := secret.Data[clusterName]
	if !ok {
		return nil, fmt.Errorf("secret %q missing data key %q", secret.Name, clusterName)
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("secret %q data key %q is empty", secret.Name, clusterName)
	}

	return raw, nil
}

// ValidateFragment parses kubeconfig and ensures server and user credentials exist.
func ValidateFragment(data []byte) error {
	cfg, err := clientcmd.Load(data)
	if err != nil {
		return fmt.Errorf("parse kubeconfig: %w", err)
	}

	if len(cfg.Clusters) == 0 {
		return fmt.Errorf("kubeconfig has no clusters")
	}

	if len(cfg.AuthInfos) == 0 {
		return fmt.Errorf("kubeconfig has no users")
	}

	for name, cluster := range cfg.Clusters {
		if strings.TrimSpace(cluster.Server) == "" {
			return fmt.Errorf("cluster %q missing server", name)
		}
	}

	for name, user := range cfg.AuthInfos {
		if user.Token == "" && user.ClientCertificateData == nil && user.ClientKeyData == nil {
			return fmt.Errorf("user %q has no credentials", name)
		}
	}

	return nil
}

// APIChecker performs a lightweight spoke API health probe using kubeconfig bytes.
type APIChecker interface {
	Check(ctx context.Context, kubeconfig []byte) error
}

// DefaultAPIChecker GETs /version on the spoke API server.
type DefaultAPIChecker struct{}

func (DefaultAPIChecker) Check(ctx context.Context, kubeconfig []byte) error {
	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("rest config: %w", err)
	}

	restCfg.Timeout = healthCheckTimeout

	client, err := rest.UnversionedRESTClientFor(restCfg)
	if err != nil {
		return fmt.Errorf("rest client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	result := client.Get().AbsPath("/version").Do(ctx)
	if err := result.Error(); err != nil {
		return fmt.Errorf("spoke API /version: %w", err)
	}

	return nil
}

// VerifySecret loads kubeconfig from secret, validates structure, and optionally probes the API.
func VerifySecret(
	ctx context.Context,
	secret *corev1.Secret,
	clusterName string,
	checker APIChecker,
) error {
	raw, err := ExtractKubeconfig(secret, clusterName)
	if err != nil {
		return err
	}

	if err := ValidateFragment(raw); err != nil {
		return err
	}

	if checker == nil {
		return nil
	}

	return checker.Check(ctx, raw)
}

// SecretGetErrorClass maps secret load failures to reconcile error classes.
func SecretGetErrorClass(err error) string {
	if err == nil {
		return ""
	}

	if apierrors.IsNotFound(err) {
		return "terminal"
	}

	return "transient"
}
