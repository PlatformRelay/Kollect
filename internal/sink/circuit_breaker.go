// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"fmt"
	"sync"
	"time"

	"github.com/sony/gobreaker/v2"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

const (
	circuitBreakerInterval = 60 * time.Second
	circuitBreakerTimeout  = 30 * time.Second
	circuitBreakerTripAt   = uint32(5)
)

var (
	breakerRegistry sync.Map // map[string]*gobreaker.CircuitBreaker[struct{}]
)

func exportThroughBreaker(key string, fn func() error) error {
	br := breakerFor(key)
	_, err := br.Execute(func() (struct{}, error) {
		if err := fn(); err != nil {
			return struct{}{}, err
		}

		return struct{}{}, nil
	})
	if err == nil {
		return nil
	}

	if err == gobreaker.ErrOpenState {
		return kollecterrors.Transient(fmt.Errorf("sink circuit breaker open for %q", key))
	}

	return err
}

func breakerFor(key string) *gobreaker.CircuitBreaker[struct{}] {
	if existing, ok := breakerRegistry.Load(key); ok {
		return existing.(*gobreaker.CircuitBreaker[struct{}])
	}

	br := gobreaker.NewCircuitBreaker[struct{}](gobreaker.Settings{
		Name:        key,
		MaxRequests: 1,
		Interval:    circuitBreakerInterval,
		Timeout:     circuitBreakerTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= circuitBreakerTripAt
		},
	})

	actual, _ := breakerRegistry.LoadOrStore(key, br)
	return actual.(*gobreaker.CircuitBreaker[struct{}])
}

// ResetBreakersForTest clears per-sink circuit breakers between tests. It
// clears the registry in place, so it is safe to call while parallel tests
// export through it; assigning a fresh sync.Map would race with their reads.
func ResetBreakersForTest() {
	breakerRegistry.Clear()
}
