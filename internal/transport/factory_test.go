// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
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

func TestNewTransportNATSStub(t *testing.T) {
	t.Parallel()

	_, _, err := NewTransport(Config{Type: TypeNATS})
	if err == nil {
		t.Fatal("expected error for NATS stub")
	}
}

func TestNewTransportKafkaStub(t *testing.T) {
	t.Parallel()

	_, _, err := NewTransport(Config{Type: TypeKafka})
	if err == nil {
		t.Fatal("expected error for Kafka stub")
	}
}
