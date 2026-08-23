//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tc "github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/platformrelay/kollect/internal/integrationtest"
)

// natsClientPort is the NATS client port the module exposes and maps.
const natsClientPort = "4222/tcp"

// natsReadyLog is the last line nats-server emits during startup, after
// "Listening for client connections". It is an [INF] line, so it is present
// regardless of the debug/trace flags the module passes.
const natsReadyLog = "Server is ready"

// startNATSTestContainer starts a NATS container and returns its client URL
// once the server itself reports readiness.
//
// The module default (wait.ForListeningPort(natsClientPort)) is not sufficient
// here. That strategy runs an in-container check via /bin/sh, but the nats
// image has no shell, so testcontainers logs "Shell not found in container" and
// falls back to the external check alone — a single TCP dial to the *mapped
// host port*. Docker's port forwarder accepts that dial as soon as the port is
// published, before nats-server binds, so the container can be declared ready
// early and the first real client connection is closed immediately (EOF).
// Gating on the server's own ready line removes that window.
func startNATSTestContainer(t *testing.T) string {
	t.Helper()

	ctx := context.Background()
	container, err := tc.Run(ctx, "nats:2.11",
		testcontainers.WithWaitStrategy(
			wait.ForLog(natsReadyLog),
			wait.ForListeningPort(natsClientPort),
		),
	)
	if err != nil {
		if integrationtest.IsDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start nats: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	url, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return url
}
