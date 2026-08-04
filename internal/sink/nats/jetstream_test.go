// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"errors"
	"strings"
	"testing"

	natsgo "github.com/nats-io/nats.go"
)

func TestJetStream_connectErrorPropagates(t *testing.T) {
	t.Parallel()
	b := &Backend{
		cfg: Config{URL: "nats://127.0.0.1:4222", Stream: "s", Subject: "sub"},
		connectFn: func(Config, TLSConfig) (*natsgo.Conn, error) {
			return nil, errors.New("dial refused")
		},
	}
	_, err := b.jetStream(t.Context())
	if err == nil || !strings.Contains(err.Error(), "dial refused") {
		t.Fatalf("error = %v, want dial refused", err)
	}
}
