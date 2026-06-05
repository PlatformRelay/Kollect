// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package transport defines a lean publish/subscribe abstraction for inventory change
// notifications. Phase 1 uses an in-process implementation; NATS (or JetStream) is the
// intended external adapter for multi-replica and hub-spoke topologies (see README).
package transport

import "context"

// Publisher delivers messages to a subject.
type Publisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}

// Handler processes a message payload.
type Handler func(ctx context.Context, payload []byte) error

// WireHandler processes a message with optional queue wire cluster metadata (ADR-0028).
type WireHandler func(ctx context.Context, wireCluster string, payload []byte) error

// Subscriber receives messages for a subject.
type Subscriber interface {
	Subscribe(ctx context.Context, subject string, handler Handler) error
}

// WireSubscriber delivers queue wire metadata (e.g. Redis cluster_id field) to handlers.
type WireSubscriber interface {
	Subscriber
	SubscribeWire(ctx context.Context, subject string, handler WireHandler) error
}

// Bus combines publish and subscribe for in-process wiring.
type Bus interface {
	Publisher
	Subscriber
}
