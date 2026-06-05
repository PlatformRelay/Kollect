// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"context"
	"fmt"
	"os"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/transport"
)

// RunnerConfig configures a hub-side spoke-report consumer.
type RunnerConfig struct {
	HubName   string
	Subject   string
	Transport transport.Config
}

// ConfigFromEnv reads hub consumer settings from the environment (set by KollectHub Deployment).
func ConfigFromEnv() (RunnerConfig, error) {
	hubName := os.Getenv("KOLLECT_HUB_NAME")
	if hubName == "" {
		return RunnerConfig{}, fmt.Errorf("KOLLECT_HUB_NAME is required in hub consumer mode")
	}

	transportType := os.Getenv("KOLLECT_TRANSPORT_TYPE")
	if transportType == "" {
		transportType = string(transport.TypeInProcess)
	}

	cfg := transport.Config{
		Type:   transport.Type(transportType),
		Stream: defaultSubject,
	}
	if cfg.Type == transport.TypeRedis {
		cfg.Redis.URL = os.Getenv("KOLLECT_REDIS_URL")
	}

	subject := os.Getenv("KOLLECT_HUB_SUBJECT")
	if subject == "" {
		subject = defaultSubject
	}

	return RunnerConfig{
		HubName:   hubName,
		Subject:   subject,
		Transport: cfg,
	}, nil
}

// Runner subscribes to spoke reports and merges them into a hub-side store.
type Runner struct {
	consumer *Consumer
}

// NewRunner wires transport subscriber → merger → consumer.
func NewRunner(store *collect.Store, cfg RunnerConfig) (*Runner, error) {
	_, sub, err := transport.NewTransport(cfg.Transport)
	if err != nil {
		return nil, fmt.Errorf("hub runner transport: %w", err)
	}

	merger := NewMerger(store)
	consumer := NewConsumer(sub, merger, cfg.Subject, cfg.HubName)

	return &Runner{consumer: consumer}, nil
}

// Start blocks until ctx is cancelled.
func (r *Runner) Start(ctx context.Context) error {
	if r == nil || r.consumer == nil {
		return fmt.Errorf("hub runner: not initialized")
	}

	return r.consumer.Start(ctx)
}

// NeedLeaderElection is false — each hub consumer pod may subscribe concurrently when sharded.
func (r *Runner) NeedLeaderElection() bool {
	return false
}
