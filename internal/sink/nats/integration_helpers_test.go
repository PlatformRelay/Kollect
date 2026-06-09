//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"testing"

	tc "github.com/testcontainers/testcontainers-go/modules/nats"

	"github.com/konih/kollect/internal/integrationtest"
)

func startNATSTestContainer(t *testing.T) string {
	t.Helper()

	ctx := context.Background()
	container, err := tc.Run(ctx, "nats:2.11")
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
