// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// EventEnvelope is the JSON message published to Kafka topics.
type EventEnvelope struct {
	Timestamp string          `json:"timestamp"`
	Cluster   string          `json:"cluster"`
	Namespace string          `json:"namespace"`
	Payload   json.RawMessage `json:"payload"`
}

// Backend publishes inventory change events to Kafka.
type Backend struct {
	cfg    Config
	writer *kafka.Writer
}

// NewBackend constructs a kafka sink backend.
func NewBackend(
	spec kollectdevv1alpha1.KollectSinkSpec,
	secretData map[string][]byte,
) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec, secretData)
	if err != nil {
		return nil, err
	}

	transport, err := dialTransport(cfg)
	if err != nil {
		return nil, err
	}

	writer := &kafka.Writer{
		Addr:      kafka.TCP(cfg.Brokers...),
		Topic:     cfg.Topic,
		Balancer:  &kafka.LeastBytes{},
		Transport: transport,
	}

	return &Backend{cfg: cfg, writer: writer}, nil
}

// Type returns the sink type identifier.
func (b *Backend) Type() string {
	return typeName
}

// Close releases the Kafka writer.
func (b *Backend) Close() error {
	if b.writer == nil {
		return nil
	}

	return b.writer.Close()
}

// Export publishes an inventory change envelope to the configured topic.
func (b *Backend) Export(ctx context.Context, payload []byte, objectPath string) error {
	if len(payload) == 0 {
		return fmt.Errorf("kafka export: empty payload")
	}

	namespace := namespaceFromObjectPath(objectPath)
	envelope := EventEnvelope{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Cluster:   b.cfg.Cluster,
		Namespace: namespace,
		Payload:   json.RawMessage(payload),
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("kafka export: marshal envelope: %w", err)
	}

	key := b.cfg.Cluster + "/" + namespace
	if strings.TrimSpace(key) == "/" {
		key = objectPath
	}

	err = b.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: body,
	})
	if err != nil {
		return fmt.Errorf("kafka publish: %w", err)
	}

	return nil
}

func namespaceFromObjectPath(objectPath string) string {
	objectPath = strings.TrimPrefix(strings.TrimSpace(objectPath), "inventory/")
	parts := strings.Split(objectPath, "/")
	if len(parts) >= 1 && parts[0] != "" {
		return parts[0]
	}

	return ""
}

func dialTransport(cfg Config) (*kafka.Transport, error) {
	transport := &kafka.Transport{}

	if cfg.Username != "" {
		mechanism, err := scram.Mechanism(scram.SHA256, cfg.Username, cfg.Password)
		if err != nil {
			return nil, fmt.Errorf("kafka SASL: %w", err)
		}

		transport.SASL = mechanism
	}

	return transport, nil
}
