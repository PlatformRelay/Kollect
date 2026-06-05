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

	runner, err := hub.NewRunner(collect.NewStore(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if runner == nil {
		t.Fatal("expected runner")
	}
}
