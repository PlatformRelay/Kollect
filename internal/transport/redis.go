// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultStream   = "kollect.hub"
	defaultHubGroup = "kollect-hub"
)

// RedisTransport publishes and consumes inventory reports via Redis Streams.
type RedisTransport struct {
	client *redis.Client
	stream string
	group  string
	mu     sync.Mutex
	subs   map[string]context.CancelFunc
}

func newRedisTransport(cfg Config) (Publisher, Subscriber, error) {
	url := cfg.Redis.URL
	if url == "" {
		return nil, nil, fmt.Errorf("redis transport: url is required")
	}

	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	stream := cfg.Stream
	if stream == "" {
		stream = defaultStream
	}

	group := cfg.Group
	if group == "" {
		group = defaultHubGroup
	}

	rt := &RedisTransport{
		client: client,
		stream: stream,
		group:  group,
		subs:   make(map[string]context.CancelFunc),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.ensureGroup(ctx); err != nil {
		_ = client.Close()

		return nil, nil, err
	}

	return rt, rt, nil
}

func (r *RedisTransport) ensureGroup(ctx context.Context) error {
	err := r.client.XGroupCreateMkStream(ctx, r.stream, r.group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create consumer group: %w", err)
	}

	return nil
}

// Publish appends payload to the Redis stream with subject metadata.
func (r *RedisTransport) Publish(ctx context.Context, subject string, payload []byte) error {
	values := map[string]any{
		"subject": subject,
		"payload": payload,
	}
	if cluster := WireClusterID(ctx); cluster != "" {
		values[WireClusterIDKey] = cluster
	}

	_, err := r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: r.stream,
		Values: values,
	}).Result()
	if err != nil {
		return fmt.Errorf("redis XADD: %w", err)
	}

	return nil
}

// SubscribeWire reads stream entries and passes cluster_id wire metadata to the handler.
func (r *RedisTransport) SubscribeWire(ctx context.Context, subject string, handler WireHandler) error {
	return r.subscribe(ctx, subject, func(loopCtx context.Context, wireCluster string, payload []byte) error {
		return handler(loopCtx, wireCluster, payload)
	})
}

// Subscribe reads from the consumer group and invokes handler for matching subjects.
func (r *RedisTransport) Subscribe(ctx context.Context, subject string, handler Handler) error {
	return r.subscribe(ctx, subject, func(loopCtx context.Context, _ string, payload []byte) error {
		return handler(loopCtx, payload)
	})
}

func (r *RedisTransport) subscribe(
	ctx context.Context,
	subject string,
	handler func(context.Context, string, []byte) error,
) error {
	r.mu.Lock()
	if _, exists := r.subs[subject]; exists {
		r.mu.Unlock()

		return fmt.Errorf("already subscribed to %q", subject)
	}

	loopCtx, cancel := context.WithCancel(ctx)
	r.subs[subject] = cancel
	r.mu.Unlock()

	consumer := fmt.Sprintf("consumer-%s", subject)

	go func() {
		defer cancel()

		for {
			select {
			case <-loopCtx.Done():
				return
			default:
			}

			streams, err := r.client.XReadGroup(loopCtx, &redis.XReadGroupArgs{
				Group:    r.group,
				Consumer: consumer,
				Streams:  []string{r.stream, ">"},
				Count:    10,
				Block:    2 * time.Second,
			}).Result()
			if err != nil {
				if err == redis.Nil || loopCtx.Err() != nil {
					continue
				}

				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					msgSubject, _ := msg.Values["subject"].(string)
					if msgSubject != subject {
						_ = r.client.XAck(loopCtx, r.stream, r.group, msg.ID).Err()

						continue
					}

					var payload []byte
					switch v := msg.Values["payload"].(type) {
					case string:
						payload = []byte(v)
					case []byte:
						payload = v
					}

					wireCluster, _ := msg.Values[WireClusterIDKey].(string)

					if err := handler(loopCtx, wireCluster, payload); err == nil {
						_ = r.client.XAck(loopCtx, r.stream, r.group, msg.ID).Err()
					}
				}
			}
		}
	}()

	return nil
}

// Close shuts down subscribers and the Redis client.
func (r *RedisTransport) Close() error {
	r.mu.Lock()
	for _, cancel := range r.subs {
		cancel()
	}

	r.subs = make(map[string]context.CancelFunc)
	r.mu.Unlock()

	return r.client.Close()
}
