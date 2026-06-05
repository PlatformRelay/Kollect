// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"
	"sync"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/git"
	"github.com/konih/kollect/internal/sink/s3"
)

// Backend exports inventory payloads to an external destination.
type Backend interface {
	Type() string
	Export(ctx context.Context, payload []byte, path string) error
}

// BuildContext carries resolved material for backend construction.
type BuildContext struct {
	CAPEM      []byte
	SecretData map[string][]byte
}

// Factory constructs a Backend from a KollectSink spec.
type Factory func(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error)

// Registry maps sink type strings to backend factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry returns a registry with built-in backends registered.
func NewRegistry() *Registry {
	r := &Registry{factories: make(map[string]Factory)}
	r.Register("git", newGitBackend)
	r.Register("s3", newS3Backend)

	return r
}

// Register adds or replaces a factory for sinkType.
func (r *Registry) Register(sinkType string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories[sinkType] = factory
}

// NewBackend resolves spec.Type via the registry and constructs a backend instance.
func (r *Registry) NewBackend(
	spec kollectdevv1alpha1.KollectSinkSpec,
	ctx BuildContext,
) (Backend, error) {
	r.mu.RLock()
	factory, ok := r.factories[spec.Type]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown sink type %q", spec.Type)
	}

	return factory(spec, ctx)
}

func newGitBackend(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error) {
	auth := git.Auth{}
	if ctx.SecretData != nil {
		if v, ok := ctx.SecretData["username"]; ok {
			auth.Username = string(v)
		}

		for _, key := range []string{"password", "token"} {
			if v, ok := ctx.SecretData[key]; ok {
				if key == "token" {
					auth.Token = string(v)
				} else {
					auth.Password = string(v)
				}
			}
		}
	}

	b, err := git.NewBackend(spec, ctx.CAPEM, auth)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func newS3Backend(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error) {
	return s3.NewBackend(spec, ctx.SecretData)
}
