// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"context"
	"testing"
)

func TestNewTransportInProcess(t *testing.T) {
	t.Parallel()

	pub, sub, err := NewTransport(Config{Type: TypeInProcess})
	if err != nil {
		t.Fatal(err)
	}

	if pub == nil || sub == nil {
		t.Fatal("expected non-nil publisher and subscriber")
	}
}

func TestNewTransportNATSMissingURL(t *testing.T) {
	t.Parallel()

	_, _, err := NewTransport(Config{Type: TypeNATS})
	if err == nil {
		t.Fatal("expected error for NATS without url")
	}
}

func TestNewTransportKafkaMissingBrokers(t *testing.T) {
	t.Parallel()

	_, _, err := NewTransport(Config{Type: TypeKafka})
	if err == nil {
		t.Fatal("expected error for kafka without brokers")
	}
}

func TestNewTransportUnknownType(t *testing.T) {
	t.Parallel()

	_, _, err := NewTransport(Config{Type: "mqtt"})
	if err == nil {
		t.Fatal("expected error for unknown transport")
	}
}

func TestNewTransportRedisMissingURL(t *testing.T) {
	t.Parallel()

	_, _, err := NewTransport(Config{Type: TypeRedis})
	if err == nil {
		t.Fatal("expected error for redis without url")
	}
}

func TestRoundTripInProcess(t *testing.T) {
	t.Parallel()

	bus := NewInProcessBus()
	if err := RoundTrip(context.Background(), bus, "inventory/default", []byte("payload")); err != nil {
		t.Fatal(err)
	}
}

func TestCloseNoOp(t *testing.T) {
	t.Parallel()

	if err := Close(NewInProcessBus()); err != nil {
		t.Fatal(err)
	}
}
