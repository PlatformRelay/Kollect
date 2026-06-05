// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"testing"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/transport"
)

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
