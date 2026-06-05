// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"context"
	"fmt"
)

// Type identifies a transport backend implementation.
type Type string

const (
	TypeInProcess Type = "inprocess"
	TypeHTTP      Type = "http"
	TypeRedis     Type = "redis"
	TypeNATS      Type = "nats"
	TypeKafka     Type = "kafka"
)

// Config holds factory parameters for a transport backend.
type Config struct {
	Type   Type
	HTTP   HTTPConfig
	Redis  RedisConfig
	Kafka  KafkaConfig
	NATS   NATSConfig
	ACL    ACLSettings
	Stream string
	Group  string
}

// HTTPConfig configures spoke → hub HTTP push ingest (ADR-0028).
type HTTPConfig struct {
	URL string
}

// RedisConfig configures a Redis Streams transport.
type RedisConfig struct {
	URL string
	TLS TLSSettings
}

// NewTransport returns a Publisher and Subscriber for the configured backend.
func NewTransport(cfg Config) (Publisher, Subscriber, error) {
	switch cfg.Type {
	case "", TypeInProcess:
		bus := NewInProcessBus()

		return bus, bus, nil
	case TypeRedis:
		return newRedisTransport(cfg)
	case TypeNATS:
		return newNATSTransport(cfg)
	case TypeKafka:
		return newKafkaTransport(cfg)
	default:
		return nil, nil, fmt.Errorf("unknown transport type %q", cfg.Type)
	}
}

// CloseableTransport can release external resources.
type CloseableTransport interface {
	Close() error
}

// Close closes a transport when it implements CloseableTransport.
func Close(t any) error {
	if c, ok := t.(CloseableTransport); ok {
		return c.Close()
	}

	return nil
}

// PublishSubscribe combines Publisher and Subscriber for tests.
type PublishSubscribe interface {
	Publisher
	Subscriber
}

// RoundTrip publishes payload on subject and waits for subscriber handler success.
func RoundTrip(ctx context.Context, ps PublishSubscribe, subject string, payload []byte) error {
	done := make(chan error, 1)
	if err := ps.Subscribe(ctx, subject, func(c context.Context, p []byte) error {
		if string(p) != string(payload) {
			done <- fmt.Errorf("payload mismatch: got %q", p)

			return nil
		}

		done <- nil

		return nil
	}); err != nil {
		return err
	}

	if err := ps.Publish(ctx, subject, payload); err != nil {
		return err
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
