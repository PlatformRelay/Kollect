// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"context"
	"fmt"
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/transport"
)

// RunnerConfig configures a hub-side spoke-report consumer.
type RunnerConfig struct {
	HubName           string
	Subject           string
	Transport         transport.Config
	RemoteClusters    []string
	AllowlistEnforced bool
}

// ConfigFromEnv reads hub consumer settings from the environment (set by KollectHub Deployment).
func ConfigFromEnv() (RunnerConfig, error) {
	hubName := os.Getenv("KOLLECT_HUB_NAME")
	if hubName == "" {
		return RunnerConfig{}, fmt.Errorf("KOLLECT_HUB_NAME is required in hub consumer mode")
	}

	cfg := transport.ConfigFromEnv()

	subject := os.Getenv("KOLLECT_HUB_SUBJECT")
	if subject == "" {
		subject = defaultSubject
	}

	remoteClusters, enforced := parseRemoteClustersEnv(os.Getenv("KOLLECT_REMOTE_CLUSTERS"))

	return RunnerConfig{
		HubName:           hubName,
		Subject:           subject,
		Transport:         cfg,
		RemoteClusters:    remoteClusters,
		AllowlistEnforced: enforced,
	}, nil
}

func parseRemoteClustersEnv(raw string) ([]string, bool) {
	_, enforced := os.LookupEnv("KOLLECT_REMOTE_CLUSTERS")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, enforced
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		clusterName := part
		if idx := strings.LastIndex(part, ":"); idx >= 0 && idx < len(part)-1 {
			clusterName = part[idx+1:]
		}

		clusterName = strings.TrimSpace(clusterName)
		if clusterName == "" {
			continue
		}

		if _, ok := seen[clusterName]; ok {
			continue
		}

		seen[clusterName] = struct{}{}
		out = append(out, clusterName)
	}

	return out, enforced
}

// Runner subscribes to spoke reports and merges them into a hub-side store.
type Runner struct {
	consumer *Consumer
}

// NewRunner wires transport subscriber → merger → consumer.
func NewRunner(
	store *collect.Store,
	cfg RunnerConfig,
	statusClient client.Client,
	exporter *Exporter,
) (*Runner, error) {
	_, sub, err := transport.NewTransport(cfg.Transport)
	if err != nil {
		return nil, fmt.Errorf("hub runner transport: %w", err)
	}

	merger := NewMerger(store)
	consumer := NewConsumer(sub, merger, cfg.Subject, cfg.HubName, statusClient, ConsumerOptions{
		AllowedClusters:   cfg.RemoteClusters,
		AllowlistEnforced: cfg.AllowlistEnforced,
		TransportACL:      cfg.Transport.ACL,
		Exporter:          exporter,
	})

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
