// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestTestConnection_missingURL(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{Subject: "inventory.events"},
	}, nil, nil)
	if err == nil {
		t.Fatal("expected error when url is missing")
	}
}

func TestTestConnection_unreachableServer(t *testing.T) {
	t.Parallel()

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{
			URL:     "nats://127.0.0.1:1",
			Subject: "inventory.events",
		},
	}, nil, nil)
	if err == nil {
		t.Fatal("expected connect error for unreachable server")
	}
}
