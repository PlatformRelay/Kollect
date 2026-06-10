// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestWebhookEnvtest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook envtest Suite")
}

var (
	webhookCtx    context.Context
	webhookCancel context.CancelFunc
	webhookEnv    *envtest.Environment
	webhookCfg    *rest.Config
	webhookClient client.Client
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	webhookCtx, webhookCancel = context.WithCancel(context.TODO())

	Expect(kollectdevv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(corev1.AddToScheme(scheme.Scheme)).To(Succeed())

	webhookEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook", "manifests.yaml")},
		},
	}

	if dir := firstEnvtestBinaryDir(); dir != "" {
		webhookEnv.BinaryAssetsDirectory = dir
	}

	var err error
	webhookCfg, err = webhookEnv.Start()
	Expect(err).NotTo(HaveOccurred())

	webhookClient, err = client.New(webhookCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	mgr, err := manager.New(webhookCfg, manager.Options{
		Scheme: scheme.Scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookEnv.WebhookInstallOptions.LocalServingHost,
			Port:    webhookEnv.WebhookInstallOptions.LocalServingPort,
			CertDir: webhookEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(SetupWithManager(mgr, false)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(webhookCtx)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	webhookCancel()
	Eventually(func() error {
		return webhookEnv.Stop()
	}, time.Minute, time.Second).Should(Succeed())
})

func firstEnvtestBinaryDir() string {
	basePath := filepath.Join("..", "..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}
