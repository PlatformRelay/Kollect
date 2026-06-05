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

// ConsumerOptions configures hub ingest merge and optional export fan-out.
type ConsumerOptions struct {
	AllowedClusters   []string
	AllowlistEnforced bool
	TransportACL      transport.ACLSettings
	Exporter          *Exporter
}

// Consumer merges spoke reports from a transport subscriber into store.
type Consumer struct {
	Subscriber        transport.Subscriber
	Merger            *Merger
	Subject           string
	HubName           string
	StatusClient      client.Client
	AllowedClusters   []string
	AllowlistEnforced bool
	TransportACL      transport.ACLSettings
	Exporter          *Exporter
}

// NewConsumer returns a hub consumer for subject (default inventory/reports).
func NewConsumer(
	sub transport.Subscriber,
	merger *Merger,
	subject, hubName string,
	statusClient client.Client,
	opts ConsumerOptions,
) *Consumer {
	if subject == "" {
		subject = defaultSubject
	}

	return &Consumer{
		Subscriber:        sub,
		Merger:            merger,
		Subject:           subject,
		HubName:           hubName,
		StatusClient:      statusClient,
		AllowedClusters:   opts.AllowedClusters,
		AllowlistEnforced: opts.AllowlistEnforced,
		TransportACL:      opts.TransportACL,
		Exporter:          opts.Exporter,
	}
}

// Start subscribes to spoke reports until ctx is cancelled.
func (c *Consumer) Start(ctx context.Context) error {
	if c == nil || c.Subscriber == nil || c.Merger == nil {
		return fmt.Errorf("hub consumer: subscriber and merger are required")
	}

	handler := func(handleCtx context.Context, wireCluster string, payload []byte) error {
		if err := c.TransportACL.ValidateClusterID(wireCluster); err != nil {
			metrics.HubSpokeReportsTotal.WithLabelValues(c.hubLabel(), metrics.ResultFailure).Inc()

			return err
		}

		report, applied, err := ReceiveReport(
			wireCluster,
			payload,
			c.Merger,
			c.AllowedClusters,
			c.AllowlistEnforced,
		)
		if err != nil {
			metrics.HubSpokeReportsTotal.WithLabelValues(c.hubLabel(), metrics.ResultFailure).Inc()

			return err
		}

		if applied > 0 {
			metrics.HubMergedItemsTotal.WithLabelValues(c.hubLabel(), report.Cluster).Add(float64(applied))
		}

		if c.StatusClient != nil {
			_ = MarkRemoteClusterConnected(handleCtx, c.StatusClient, report.Cluster)
		}

		if c.Exporter != nil {
			if err := c.Exporter.ExportAfterMerge(handleCtx, report); err != nil {
				metrics.HubSpokeReportsTotal.WithLabelValues(c.hubLabel(), metrics.ResultFailure).Inc()

				return err
			}
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

	return defaultHubMetricLabel
}
