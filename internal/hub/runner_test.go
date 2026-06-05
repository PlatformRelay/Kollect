// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"context"
	"testing"
	"time"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/transport"
)

func TestConfigFromEnvCustomSubject(t *testing.T) {
	t.Setenv("KOLLECT_HUB_NAME", "platform")
	t.Setenv("KOLLECT_HUB_SUBJECT", "custom/subject")

	cfg, err := hub.ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Subject != "custom/subject" {
		t.Fatalf("subject = %q", cfg.Subject)
	}
}

func TestConfigFromEnvRequiresHubName(t *testing.T) {
	t.Setenv("KOLLECT_HUB_NAME", "")

	if _, err := hub.ConfigFromEnv(); err == nil {
		t.Fatal("expected error when KOLLECT_HUB_NAME unset")
	}
}

func TestConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("KOLLECT_HUB_NAME", "platform")
	t.Setenv("KOLLECT_TRANSPORT_TYPE", "")
	t.Setenv("KOLLECT_REMOTE_CLUSTERS", "platform/spoke-a:spoke-a,platform/spoke-b:spoke-b")

	cfg, err := hub.ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.HubName != "platform" {
		t.Fatalf("hub name = %q", cfg.HubName)
	}

	if cfg.Transport.Type != transport.TypeInProcess {
		t.Fatalf("transport = %q", cfg.Transport.Type)
	}

	if len(cfg.RemoteClusters) != 2 || cfg.RemoteClusters[0] != "spoke-a" || cfg.RemoteClusters[1] != "spoke-b" {
		t.Fatalf("remote clusters = %#v", cfg.RemoteClusters)
	}

	if !cfg.AllowlistEnforced {
		t.Fatal("expected allowlist enforced when KOLLECT_REMOTE_CLUSTERS is set")
	}
}

func TestExportConfigFromEnv(t *testing.T) {
	t.Setenv("KOLLECT_HUB_EXPORT_NAMESPACE", "platform")
	t.Setenv("KOLLECT_HUB_SINK_REFS", "hub-postgres, hub-kafka ,hub-postgres")

	cfg := hub.ExportConfigFromEnv()
	if !cfg.ExportEnabled() {
		t.Fatal("expected export enabled")
	}
	if cfg.ExportNamespace != "platform" {
		t.Fatalf("namespace = %q", cfg.ExportNamespace)
	}
	if len(cfg.SinkRefs) != 2 {
		t.Fatalf("sink refs = %#v", cfg.SinkRefs)
	}
}

func TestIngestConfigFromEnv(t *testing.T) {
	t.Setenv("KOLLECT_HUB_INGEST_PORT", "9090")
	t.Setenv("KOLLECT_HUB_INGEST_AUTH_MODE", "")
	t.Setenv("KOLLECT_PLATFORM_NAMESPACE", " kollect-system ")

	port, mode := hub.IngestConfigFromEnv()
	if port != 9090 || mode != hub.IngestAuthModeKubernetes {
		t.Fatalf("port=%d mode=%q", port, mode)
	}
	if hub.PlatformNamespaceFromEnv() != "kollect-system" {
		t.Fatalf("platform ns = %q", hub.PlatformNamespaceFromEnv())
	}
}

func TestRunnerStartRequiresConsumer(t *testing.T) {
	t.Parallel()

	var r hub.Runner
	if err := r.Start(context.Background()); err == nil {
		t.Fatal("expected error for nil runner")
	}
}

func TestRunnerNeedLeaderElection(t *testing.T) {
	t.Parallel()

	r, err := hub.NewRunner(collect.NewStore(), hub.RunnerConfig{
		HubName:   "hub",
		Subject:   "inventory/reports",
		Transport: transport.Config{Type: transport.TypeInProcess},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.NeedLeaderElection() {
		t.Fatal("hub runner should not require leader election")
	}
}

func TestNewRunnerInProcess(t *testing.T) {
	t.Parallel()

	cfg := hub.RunnerConfig{
		HubName: "test",
		Subject: "inventory/reports",
		Transport: transport.Config{
			Type: transport.TypeInProcess,
		},
	}

	runner, err := hub.NewRunner(collect.NewStore(), cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if runner == nil {
		t.Fatal("expected runner")
	}
}

func TestRunnerStartInProcess(t *testing.T) {
	t.Parallel()

	runner, err := hub.NewRunner(collect.NewStore(), hub.RunnerConfig{
		HubName:   "test",
		Subject:   "inventory/reports",
		Transport: transport.Config{Type: transport.TypeInProcess},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Start(ctx)
	}()

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runner did not stop")
	}
}

func TestExportConfigDisabled(t *testing.T) {
	t.Parallel()

	cfg := hub.ExportConfig{ExportNamespace: "platform"}
	if cfg.ExportEnabled() {
		t.Fatal("missing sink refs should disable export")
	}
	cfg = hub.ExportConfig{SinkRefs: []string{"a"}}
	if cfg.ExportEnabled() {
		t.Fatal("missing namespace should disable export")
	}
}
