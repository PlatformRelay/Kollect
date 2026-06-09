// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"testing"
)

func TestConnect_unreachableWithToken(t *testing.T) {
	t.Parallel()

	_, err := connect(Config{
		URL:   "nats://127.0.0.1:1",
		Token: "secret",
	}, TLSConfig{})
	if err == nil {
		t.Fatal("expected connect error for unreachable server")
	}
}

func TestConnect_unreachableWithUserInfo(t *testing.T) {
	t.Parallel()

	_, err := connect(Config{
		URL:      "nats://127.0.0.1:1",
		Username: "user",
		Password: "pass",
	}, TLSConfig{InsecureSkipVerify: true})
	if err == nil {
		t.Fatal("expected connect error for unreachable server")
	}
}
