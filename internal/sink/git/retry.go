// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel
//
// Adapted from Argo CD Image Updater (Apache-2.0): ext/git/client.go (LsRemote retry loop)

package git

import (
	"context"
	"math"
	"time"
)

const (
	defaultRetryAttempts   = 3
	defaultRetryInitial    = 1 * time.Second
	defaultRetryMaxBackoff = 30 * time.Second
	defaultRetryFactor     = 2.0
)

type retryConfig struct {
	maxAttempts  int
	initialDelay time.Duration
	maxDelay     time.Duration
	factor       float64
	shouldRetry  func(error) bool
}

func defaultTransportRetry() retryConfig {
	return retryConfig{
		maxAttempts:  defaultRetryAttempts,
		initialDelay: defaultRetryInitial,
		maxDelay:     defaultRetryMaxBackoff,
		factor:       defaultRetryFactor,
		shouldRetry:  isTransientTransportError,
	}
}

func withTransportRetry(ctx context.Context, cfg retryConfig, fn func() error) error {
	if cfg.maxAttempts < 1 {
		cfg.maxAttempts = 1
	}

	if cfg.shouldRetry == nil {
		cfg.shouldRetry = isTransientTransportError
	}

	var lastErr error
	for attempt := 0; attempt < cfg.maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if !cfg.shouldRetry(lastErr) || attempt+1 >= cfg.maxAttempts {
			return lastErr
		}

		delay := retryBackoff(cfg, attempt)
		if err := sleepContext(ctx, delay); err != nil {
			return err
		}
	}

	return lastErr
}

func retryBackoff(cfg retryConfig, attempt int) time.Duration {
	if cfg.initialDelay <= 0 {
		cfg.initialDelay = defaultRetryInitial
	}

	if cfg.factor <= 0 {
		cfg.factor = defaultRetryFactor
	}

	wait := float64(cfg.initialDelay) * math.Pow(cfg.factor, float64(attempt))
	if cfg.maxDelay > 0 {
		wait = math.Min(wait, float64(cfg.maxDelay))
	}

	return time.Duration(wait)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
