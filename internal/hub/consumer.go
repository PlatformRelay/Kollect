// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"context"
	"fmt"

	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/transport"
)

const defaultSubject = "inventory/reports"

// Consumer merges spoke reports from a transport subscriber into store.
type Consumer struct {
	Subscriber transport.Subscriber
	Merger     *Merger
	Subject    string
	HubName    string
}

// NewConsumer returns a hub consumer for subject (default inventory/reports).
func NewConsumer(sub transport.Subscriber, merger *Merger, subject, hubName string) *Consumer {
	if subject == "" {
		subject = defaultSubject
	}

	return &Consumer{
		Subscriber: sub,
		Merger:     merger,
		Subject:    subject,
		HubName:    hubName,
	}
}

// Start subscribes to spoke reports until ctx is cancelled.
func (c *Consumer) Start(ctx context.Context) error {
	if c == nil || c.Subscriber == nil || c.Merger == nil {
		return fmt.Errorf("hub consumer: subscriber and merger are required")
	}

	return c.Subscriber.Subscribe(ctx, c.Subject, func(_ context.Context, payload []byte) error {
		if _, err := c.Merger.ApplyJSON(payload); err != nil {
			metrics.HubSpokeReportsTotal.WithLabelValues(c.hubLabel(), metrics.ResultFailure).Inc()

			return err
		}

		metrics.HubSpokeReportsTotal.WithLabelValues(c.hubLabel(), metrics.ResultSuccess).Inc()

		return nil
	})
}

func (c *Consumer) hubLabel() string {
	if c.HubName != "" {
		return c.HubName
	}

	return "default"
}
