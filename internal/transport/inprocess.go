// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"context"
	"fmt"
	"sync"
)

// InProcessBus is a goroutine-safe pub/sub bus for a single process.
type InProcessBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewInProcessBus returns an empty in-process bus.
func NewInProcessBus() *InProcessBus {
	return &InProcessBus{handlers: make(map[string][]Handler)}
}

// Publish invokes all handlers registered for subject.
func (b *InProcessBus) Publish(ctx context.Context, subject string, payload []byte) error {
	b.mu.RLock()
	handlers := append([]Handler(nil), b.handlers[subject]...)
	b.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, payload); err != nil {
			return fmt.Errorf("handler for %q: %w", subject, err)
		}
	}

	return nil
}

// Subscribe registers a handler for subject.
func (b *InProcessBus) Subscribe(_ context.Context, subject string, handler Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[subject] = append(b.handlers[subject], handler)

	return nil
}
