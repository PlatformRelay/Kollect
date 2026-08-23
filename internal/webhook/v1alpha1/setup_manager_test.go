// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func startWebhookManager(t *testing.T, tenantMode bool) {
	t.Helper()

	if err := kollectdevv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}

	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook", "manifests.yaml")},
		},
	}
	if dir := hostEnvtestBinaryDir(); dir != "" {
		env.BinaryAssetsDirectory = dir
	}

	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("envtest start: %v", err)
	}
	t.Cleanup(func() {
		_ = env.Stop()
	})

	mgr, err := manager.New(cfg, manager.Options{
		Scheme: scheme.Scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    env.WebhookInstallOptions.LocalServingHost,
			Port:    env.WebhookInstallOptions.LocalServingPort,
			CertDir: env.WebhookInstallOptions.LocalServingCertDir,
		}),
	})
	if err != nil {
		t.Fatalf("manager.New: %v", err)
	}
	if err := SetupWithManager(mgr, tenantMode); err != nil {
		t.Fatalf("SetupWithManager(tenantMode=%v): %v", tenantMode, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = mgr.Start(ctx) }()
	time.Sleep(200 * time.Millisecond)
}

func TestSetupWithManager_tenantModeFalse(t *testing.T) {
	startWebhookManager(t, false)
}

func TestSetupWithManager_tenantModeTrue(t *testing.T) {
	startWebhookManager(t, true)
}

func TestClusterKindRejectedInTenantMode_message(t *testing.T) {
	t.Parallel()

	err := clusterKindRejectedInTenantMode("KollectClusterTarget", "demo")
	if err == nil || err.Error() == "" {
		t.Fatalf("error = %v, want admission denial", err)
	}
}
