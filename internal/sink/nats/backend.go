// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/digest"
	"github.com/platformrelay/kollect/internal/sink/cap"
)

// EventEnvelope is the JSON message published to NATS JetStream subjects.
type EventEnvelope struct {
	SchemaVersion string          `json:"schemaVersion"`
	Timestamp     string          `json:"timestamp"`
	Cluster       string          `json:"cluster"`
	Namespace     string          `json:"namespace"`
	Payload       json.RawMessage `json:"payload"`
}

type Backend struct {
	cfg Config
	tls TLSConfig
	mu  sync.Mutex
	nc  *natsgo.Conn
	js  jetstream.JetStream

	// jsProvider is the seam over jetStream that lets Export be unit-tested
	// with a fake JetStream instead of a real connection. NewBackend defaults
	// it to b.jetStream, preserving connection caching and reconnect behavior.
	jsProvider func(ctx context.Context) (jetstream.JetStream, error)
}

func NewBackend(
	spec kollectdevv1alpha1.KollectSinkSpec,
	secretData map[string][]byte,
	caPEM []byte,
) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec, secretData)
	if err != nil {
		return nil, err
	}
	tlsCfg, err := TLSConfigFromSpec(spec.TLS, caPEM)
	if err != nil {
		return nil, err
	}
	b := &Backend{cfg: cfg, tls: tlsCfg}
	b.jsProvider = b.jetStream
	return b, nil
}

func (b *Backend) Type() string { return typeName }

func (b *Backend) Capabilities() cap.Capabilities { return cap.StreamEmitter() }

func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.nc != nil {
		b.nc.Close()
		b.nc = nil
		b.js = nil
	}
	return nil
}

func (b *Backend) Export(ctx context.Context, payload []byte, objectPath string) error {
	if len(payload) == 0 {
		return fmt.Errorf("nats export: empty payload")
	}
	js, err := b.jsProvider(ctx)
	if err != nil {
		return err
	}
	namespace := namespaceFromObjectPath(objectPath)
	body, err := marshalEventEnvelope(b.cfg.Cluster, namespace, payload, time.Now())
	if err != nil {
		return fmt.Errorf("nats export: marshal envelope: %w", err)
	}

	_, err = js.Publish(ctx, b.cfg.Subject, body, jetstream.WithMsgID(msgID(b.cfg.Cluster, namespace, payload)))
	if err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}
	return nil
}

func (b *Backend) jetStream(ctx context.Context) (jetstream.JetStream, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.js != nil {
		if b.nc != nil && !b.nc.IsClosed() {
			return b.js, nil
		}
		// The cached connection is closed (e.g. the server went away and the
		// client exhausted its reconnect attempts). Drop it so a fresh
		// connection is established below instead of failing every Export
		// for the lifetime of the operator process.
		if b.nc != nil {
			b.nc.Close()
		}
		b.nc = nil
		b.js = nil
	}
	nc, err := connect(b.cfg, b.tls)
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats jetstream: %w", err)
	}
	if err := ensureStream(ctx, js, b.cfg); err != nil {
		nc.Close()
		return nil, err
	}
	b.nc = nc
	b.js = js
	return b.js, nil
}

// msgID derives the deterministic JetStream message ID for a payload, used for
// server-side de-duplication. Isolated so it can be unit-tested directly.
func msgID(cluster, namespace string, payload []byte) string {
	return digest.ContentHash(append([]byte(cluster+"/"+namespace+"/"), payload...))
}

func ensureStream(ctx context.Context, js jetstream.JetStream, cfg Config) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     cfg.Stream,
		Subjects: streamSubjects(cfg.Subject),
	})
	if err != nil {
		return fmt.Errorf("nats create stream: %w", err)
	}
	return nil
}

func streamSubjects(subject string) []string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil
	}
	return []string{subject}
}

func namespaceFromObjectPath(objectPath string) string {
	objectPath = strings.TrimPrefix(strings.TrimSpace(objectPath), "inventory/")
	parts := strings.Split(objectPath, "/")
	if len(parts) >= 1 && parts[0] != "" {
		return parts[0]
	}
	return ""
}
