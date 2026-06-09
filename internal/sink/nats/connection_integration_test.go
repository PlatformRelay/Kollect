//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestTestConnection_liveJetStream(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	url := startNATSTestContainer(t)

	err := TestConnection(context.Background(), kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{
			URL:     url,
			Subject: "inventory.events",
			Stream:  "kollect_test_probe",
		},
	}, nil, nil)
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
}
