// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithTransportRetry_succeedsFirstAttempt(t *testing.T) {
	t.Parallel()

	calls := 0
	err := withTransportRetry(t.Context(), defaultTransportRetry(), func() error {
		calls++

		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestWithTransportRetry_retriesTransient(t *testing.T) {
	t.Parallel()

	cfg := defaultTransportRetry()
	cfg.initialDelay = time.Millisecond
	cfg.maxDelay = time.Millisecond

	calls := 0
	err := withTransportRetry(t.Context(), cfg, func() error {
		calls++
		if calls < 3 {
			return errors.New("connection reset by peer")
		}

		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestWithTransportRetry_stopsOnTerminal(t *testing.T) {
	t.Parallel()

	cfg := defaultTransportRetry()
	cfg.initialDelay = time.Millisecond

	calls := 0
	err := withTransportRetry(t.Context(), cfg, func() error {
		calls++

		return errors.New("authentication required")
	})
	if err == nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestWithTransportRetry_respectsContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	cfg := defaultTransportRetry()
	cfg.initialDelay = time.Second

	err := withTransportRetry(ctx, cfg, func() error {
		return errors.New("connection reset")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestRetryBackoff_capsAtMax(t *testing.T) {
	t.Parallel()

	cfg := retryConfig{initialDelay: time.Second, maxDelay: 5 * time.Second, factor: 10}
	got := retryBackoff(cfg, 3)
	if got != 5*time.Second {
		t.Fatalf("backoff = %v", got)
	}
}

func TestIsTransientTransportError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err  string
		want bool
	}{
		{"connection reset by peer", true},
		{"authentication required", false},
		{"503 service unavailable", true},
		{"pre-receive hook declined", false},
	}

	for _, tc := range cases {
		if got := isTransientTransportError(errors.New(tc.err)); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.err, got, tc.want)
		}
	}
}
