//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestRedisTransportRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := redis.Run(ctx, "redis:7-alpine")
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("docker not available: %v", err)
		}

		t.Fatalf("start redis: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	pub, sub, err := NewTransport(Config{
		Type: TypeRedis,
		Redis: RedisConfig{
			URL: connStr,
		},
		Stream: "kollect.test",
		Group:  "kollect-test",
	})
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = Close(pub)
		_ = Close(sub)
	}()

	rtCtx, rtCancel := context.WithTimeout(ctx, 30*time.Second)
	defer rtCancel()

	if err := RoundTrip(rtCtx, struct {
		Publisher
		Subscriber
	}{pub, sub}, "inventory/default", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

func isDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "cannot connect to the docker daemon") ||
		strings.Contains(msg, "docker.sock") ||
		strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "permission denied")
}
