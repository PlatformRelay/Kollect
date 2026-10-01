// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	natsgo "github.com/nats-io/nats.go"
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

// errStubDial is the sentinel the stub dialer wraps, so the tests can prove
// redaction keeps error identity.
var errStubDial = errors.New("stub dial failure")

// stubDial returns a dialer whose error echoes the credentials the way a
// driver error might: the resolved values verbatim, outside any URL.
func stubDial(text string) natsDialer {
	return func(string, ...natsgo.Option) (*natsgo.Conn, error) {
		return nil, fmt.Errorf("%s: %w", text, errStubDial)
	}
}

func TestConnectWith_redactsResolvedCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		text string
		leak string
	}{
		{
			name: "token",
			cfg:  Config{URL: "nats://127.0.0.1:1", Token: "t0k3n-value"},
			text: "authorization violation: token t0k3n-value rejected",
			leak: "t0k3n-value",
		},
		{
			name: "password",
			cfg:  Config{URL: "nats://127.0.0.1:1", Username: "user", Password: "pa55 word"},
			text: "authorization violation: user user password pa55 word rejected",
			leak: "pa55 word",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := connectWith(stubDial(tc.text), tc.cfg, TLSConfig{})
			if err == nil {
				t.Fatal("expected connect error")
			}

			if strings.Contains(err.Error(), tc.leak) {
				t.Fatalf("connect error leaked %q: %q", tc.leak, err.Error())
			}

			if !strings.Contains(err.Error(), "authorization violation") {
				t.Fatalf("connect error lost its driver context: %q", err.Error())
			}

			if !errors.Is(err, errStubDial) {
				t.Fatalf("errors.Is identity lost through redaction: %v", err)
			}
		})
	}
}
