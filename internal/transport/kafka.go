// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

const defaultKafkaTopic = "kollect.hub"

// KafkaConfig configures a Kafka/Redpanda transport backend.
type KafkaConfig struct {
	Brokers []string
	Topic   string
	Group   string
}

// KafkaTransport publishes and consumes inventory reports via Kafka topics.
type KafkaTransport struct {
	writer  *kafka.Writer
	brokers []string
	topic   string
	group   string
	mu      sync.Mutex
	subs    map[string]context.CancelFunc
}

func newKafkaTransport(cfg Config) (Publisher, Subscriber, error) {
	kcfg := cfg.Kafka
	if len(kcfg.Brokers) == 0 {
		return nil, nil, fmt.Errorf("kafka transport: brokers are required")
	}

	topic := kcfg.Topic
	if topic == "" {
		topic = defaultKafkaTopic
	}

	group := kcfg.Group
	if group == "" {
		group = defaultHubGroup
	}

	writer := &kafka.Writer{
		Addr:     kafka.TCP(kcfg.Brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	t := &KafkaTransport{
		writer:  writer,
		brokers: append([]string(nil), kcfg.Brokers...),
		topic:   topic,
		group:   group,
		subs:    make(map[string]context.CancelFunc),
	}

	return t, t, nil
}

// Publish writes payload to the topic with subject metadata in headers.
func (k *KafkaTransport) Publish(ctx context.Context, subject string, payload []byte) error {
	err := k.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(subject),
		Value: payload,
		Headers: []kafka.Header{
			{Key: "subject", Value: []byte(subject)},
		},
	})
	if err != nil {
		return fmt.Errorf("kafka publish: %w", err)
	}

	return nil
}

// Subscribe reads from a consumer group and invokes handler for matching subjects.
func (k *KafkaTransport) Subscribe(ctx context.Context, subject string, handler Handler) error {
	k.mu.Lock()
	if _, exists := k.subs[subject]; exists {
		k.mu.Unlock()

		return fmt.Errorf("already subscribed to %q", subject)
	}

	loopCtx, cancel := context.WithCancel(ctx)
	k.subs[subject] = cancel
	k.mu.Unlock()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: k.brokers,
		Topic:   k.topic,
		GroupID: k.group,
	})

	go func() {
		defer cancel()
		defer func() { _ = reader.Close() }()

		for {
			select {
			case <-loopCtx.Done():
				return
			default:
			}

			readCtx, readCancel := context.WithTimeout(loopCtx, 5*time.Second)
			msg, err := reader.FetchMessage(readCtx)
			readCancel()

			if err != nil {
				if loopCtx.Err() != nil {
					return
				}

				continue
			}

			msgSubject := string(msg.Key)
			for _, h := range msg.Headers {
				if h.Key == "subject" {
					msgSubject = string(h.Value)

					break
				}
			}

			if msgSubject != subject {
				_ = reader.CommitMessages(loopCtx, msg)

				continue
			}

			if err := kafkaConsumeHandler(loopCtx, msg.Value, handler, func() {
				_ = reader.CommitMessages(loopCtx, msg)
			}); err != nil {
				continue
			}
		}
	}()

	return nil
}

func kafkaConsumeHandler(
	ctx context.Context,
	payload []byte,
	handler Handler,
	commit func(),
) error {
	if err := handler(ctx, payload); err != nil {
		return fmt.Errorf("kafka handler: %w", err)
	}

	commit()

	return nil
}

// Close shuts down subscribers and the Kafka writer.
func (k *KafkaTransport) Close() error {
	k.mu.Lock()
	for _, cancel := range k.subs {
		cancel()
	}

	k.subs = make(map[string]context.CancelFunc)
	k.mu.Unlock()

	return k.writer.Close()
}
