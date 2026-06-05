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

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/aggregate"
	"github.com/konih/kollect/internal/sink/cap"
)

type EventEnvelope struct {
	Timestamp string          `json:"timestamp"`
	Cluster   string          `json:"cluster"`
	Namespace string          `json:"namespace"`
	Payload   json.RawMessage `json:"payload"`
}

type Backend struct {
	cfg Config
	tls TLSConfig
	mu  sync.Mutex
	nc  *natsgo.Conn
	js  jetstream.JetStream
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
	return &Backend{cfg: cfg, tls: tlsCfg}, nil
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
	js, err := b.jetStream(ctx)
	if err != nil {
		return err
	}
	envelope := EventEnvelope{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Cluster:   b.cfg.Cluster,
		Namespace: namespaceFromObjectPath(objectPath),
		Payload:   json.RawMessage(payload),
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("nats export: marshal envelope: %w", err)
	}

	msgID := aggregate.ContentHash(append([]byte(b.cfg.Cluster+"/"+envelope.Namespace+"/"), payload...))
	_, err = js.Publish(ctx, b.cfg.Subject, body, jetstream.WithMsgID(msgID))
	if err != nil {
		return fmt.Errorf("nats publish: %w", err)
	}
	return nil
}

func (b *Backend) jetStream(ctx context.Context) (jetstream.JetStream, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.js != nil {
		return b.js, nil
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
