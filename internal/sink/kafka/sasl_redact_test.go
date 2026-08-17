// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"strings"
	"testing"
)

func TestDialTransport_SASLPrepErrorOmitsPassword(t *testing.T) {
	t.Parallel()

	const secret = "s3cret\x07"
	_, err := dialTransport(Config{
		Brokers:  []string{"127.0.0.1:1"},
		Topic:    "inventory",
		Username: "alice",
		Password: secret,
	})
	if err == nil {
		t.Fatal("expected SASL construction error for a prohibited password rune")
	}
	if strings.Contains(err.Error(), "s3cret") || strings.Contains(err.Error(), secret) {
		t.Fatalf("SASL error must not include the password: %q", err.Error())
	}
}
