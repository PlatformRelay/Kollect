// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/transport"
)

const defaultSubject = "inventory/reports"

// Consumer merges spoke reports from a transport subscriber into store.
type Consumer struct {
	Subscriber   transport.Subscriber
	Merger       *Merger
	Subject      string
	HubName      string
	StatusClient client.Client
}

// NewConsumer returns a hub consumer for subject (default inventory/reports).
func NewConsumer(
	sub transport.Subscriber,
	merger *Merger,
	subject, hubName string,
	statusClient client.Client,
) *Consumer {
	if subject == "" {
		subject = defaultSubject
	}

	return &Consumer{
		Subscriber:   sub,
		Merger:       merger,
		Subject:      subject,
		HubName:      hubName,
		StatusClient: statusClient,
	}
}

// Start subscribes to spoke reports until ctx is cancelled.
func (c *Consumer) Start(ctx context.Context) error {
	if c == nil || c.Subscriber == nil || c.Merger == nil {
		return fmt.Errorf("hub consumer: subscriber and merger are required")
	}

	handler := func(handleCtx context.Context, wireCluster string, payload []byte) error {
		report, _, err := ReceiveReport(wireCluster, payload, c.Merger)
		if err != nil {
			metrics.HubSpokeReportsTotal.WithLabelValues(c.hubLabel(), metrics.ResultFailure).Inc()

			return err
		}

		if c.StatusClient != nil {
			_ = MarkRemoteClusterConnected(handleCtx, c.StatusClient, report.Cluster)
		}

		metrics.HubSpokeReportsTotal.WithLabelValues(c.hubLabel(), metrics.ResultSuccess).Inc()

		return nil
	}

	if ws, ok := c.Subscriber.(transport.WireSubscriber); ok {
		return ws.SubscribeWire(ctx, c.Subject, handler)
	}

	return c.Subscriber.Subscribe(ctx, c.Subject, func(handleCtx context.Context, payload []byte) error {
		return handler(handleCtx, "", payload)
	})
}

func (c *Consumer) hubLabel() string {
	if c.HubName != "" {
		return c.HubName
	}

	return "default"
}
