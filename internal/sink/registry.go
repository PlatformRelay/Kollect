// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"
	"sync"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/git"
)

// Backend exports inventory payloads to an external destination.
type Backend interface {
	Type() string
	Export(ctx context.Context, payload []byte) error
}

// Factory constructs a Backend from a KollectSink spec.
type Factory func(spec kollectdevv1alpha1.KollectSinkSpec) (Backend, error)

// Registry maps sink type strings to backend factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry returns a registry with built-in placeholder backends registered.
func NewRegistry() *Registry {
	r := &Registry{factories: make(map[string]Factory)}
	r.Register("git", newGitBackendFromSpec)

	return r
}

// Register adds or replaces a factory for sinkType.
func (r *Registry) Register(sinkType string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories[sinkType] = factory
}

// NewBackend resolves spec.Type via the registry and constructs a backend instance.
func (r *Registry) NewBackend(spec kollectdevv1alpha1.KollectSinkSpec) (Backend, error) {
	r.mu.RLock()
	factory, ok := r.factories[spec.Type]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown sink type %q", spec.Type)
	}

	return factory(spec)
}

func newGitBackendFromSpec(spec kollectdevv1alpha1.KollectSinkSpec) (Backend, error) {
	b, err := git.NewBackend(spec, nil)
	if err != nil {
		return nil, err
	}

	return b, nil
}
