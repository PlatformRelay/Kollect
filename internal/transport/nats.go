// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const defaultNATSStream = "kollect_hub"

// NATSConfig configures a NATS JetStream transport backend.
type NATSConfig struct {
	URL string
}

// NATSTransport publishes and consumes inventory reports via NATS JetStream.
type NATSTransport struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	stream string
	group  string
	mu     sync.Mutex
	subs   map[string]jetstream.ConsumeContext
}

func newNATSTransport(cfg Config) (Publisher, Subscriber, error) {
	url := cfg.NATS.URL
	if url == "" {
		return nil, nil, fmt.Errorf("nats transport: url is required")
	}

	nc, err := nats.Connect(url)
	if err != nil {
		return nil, nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()

		return nil, nil, fmt.Errorf("nats jetstream: %w", err)
	}

	stream := cfg.Stream
	if stream == "" {
		stream = defaultNATSStream
	}
	stream = strings.ReplaceAll(stream, ".", "_")

	group := cfg.Group
	if group == "" {
		group = "kollect-hub"
	}

	t := &NATSTransport{
		nc:     nc,
		js:     js,
		stream: stream,
		group:  group,
		subs:   make(map[string]jetstream.ConsumeContext),
	}

	if err := t.ensureStream(context.Background()); err != nil {
		nc.Close()

		return nil, nil, err
	}

	return t, t, nil
}

func (n *NATSTransport) ensureStream(ctx context.Context) error {
	_, err := n.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     n.stream,
		Subjects: []string{"inventory/reports", "inventory.>"},
	})
	if err != nil {
		return fmt.Errorf("nats create stream: %w", err)
	}

	return nil
}

// Publish writes payload to JetStream on subject.
func (n *NATSTransport) Publish(ctx context.Context, subject string, payload []byte) error {
	_, err := n.js.Publish(ctx, subject, payload)
	if err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}

	return nil
}

// Subscribe consumes messages for subject via a durable JetStream consumer.
func (n *NATSTransport) Subscribe(ctx context.Context, subject string, handler Handler) error {
	n.mu.Lock()
	if _, exists := n.subs[subject]; exists {
		n.mu.Unlock()

		return fmt.Errorf("already subscribed to %q", subject)
	}

	cons, err := n.js.CreateOrUpdateConsumer(ctx, n.stream, jetstream.ConsumerConfig{
		Durable:       fmt.Sprintf("%s-%s", n.group, sanitizeConsumerToken(subject)),
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		n.mu.Unlock()

		return fmt.Errorf("nats consumer: %w", err)
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		if err := handler(ctx, msg.Data()); err != nil {
			return
		}

		_ = msg.Ack()
	})
	if err != nil {
		n.mu.Unlock()

		return fmt.Errorf("nats consume: %w", err)
	}

	n.subs[subject] = cc
	n.mu.Unlock()

	go func() {
		<-ctx.Done()
		n.mu.Lock()
		if c, ok := n.subs[subject]; ok {
			c.Stop()
			delete(n.subs, subject)
		}
		n.mu.Unlock()
	}()

	return nil
}

// Close drains subscribers and the NATS connection.
func (n *NATSTransport) Close() error {
	n.mu.Lock()
	for _, cc := range n.subs {
		cc.Stop()
	}

	n.subs = make(map[string]jetstream.ConsumeContext)
	n.mu.Unlock()

	if n.nc != nil {
		n.nc.Close()
	}

	return nil
}

func sanitizeConsumerToken(subject string) string {
	out := make([]byte, len(subject))
	for i := range len(subject) {
		c := subject[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			out[i] = c
		default:
			out[i] = '_'
		}
	}

	return string(out)
}
