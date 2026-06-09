// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/sink/gcs"
	"github.com/konih/kollect/internal/sink/git"
	"github.com/konih/kollect/internal/sink/gitlab"
	kafkasink "github.com/konih/kollect/internal/sink/kafka"
	"github.com/konih/kollect/internal/sink/mongodb"
	natssink "github.com/konih/kollect/internal/sink/nats"
	"github.com/konih/kollect/internal/sink/postgres"
	"github.com/konih/kollect/internal/sink/s3"
)

// Backend exports inventory payloads to an external destination.
type Backend interface {
	Type() string
	Capabilities() Capabilities
	Export(ctx context.Context, payload []byte, path string) error
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
	r.Register(git.TypeName, newGitBackend)
	r.Register("gitlab", newGitLabBackend)
	r.Register("s3", newS3Backend)
	r.Register("gcs", newGCSBackend)
	r.Register("postgres", newPostgresBackend)
	r.Register(mongodb.TypeName, newMongoBackend)
	r.Register("kafka", newKafkaBackend)
	r.Register("nats", newNatsBackend)

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

	var supported []string
	if !ok {
		supported = make([]string, 0, len(r.factories))
		for t := range r.factories {
			supported = append(supported, t)
		}
	}
	r.mu.RUnlock()

	if !ok {
		// Terminal: an unknown type cannot succeed on retry — only a spec change fixes it.
		sort.Strings(supported)

		return nil, kollecterrors.Terminal(fmt.Errorf(
			"unknown sink type %q (supported: %s)", spec.Type, strings.Join(supported, ", "),
		))
	}

	return factory(spec, ctx)
}

func newGitBackend(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error) {
	b, err := git.NewBackend(
		spec,
		ctx.CAPEM,
		GitAuthFromSecretData(ctx.SecretData, gitAuthTypeFromSpec(spec)),
		GitSSHKnownHostsFromSecretData(ctx.SecretData),
	)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func newGitLabBackend(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error) {
	b, err := gitlab.NewBackend(spec, ctx.CAPEM, GitAuthFromSecretData(ctx.SecretData, ""))
	if err != nil {
		return nil, err
	}

	return b, nil
}

func newS3Backend(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error) {
	return s3.NewBackend(spec, ctx.SecretData)
}

func newGCSBackend(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error) {
	return gcs.NewBackend(spec, ctx.SecretData)
}

func newPostgresBackend(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error) {
	connectCtx := ctx.Ctx
	if connectCtx == nil {
		connectCtx = context.Background()
	}

	return postgres.NewBackend(connectCtx, spec, ctx.DatabaseSecretData)
}

func newMongoBackend(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error) {
	connectCtx := ctx.Ctx
	if connectCtx == nil {
		connectCtx = context.Background()
	}

	return mongodb.NewBackend(connectCtx, spec, ctx.DatabaseSecretData)
}

func newKafkaBackend(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error) {
	return kafkasink.NewBackend(spec, ctx.SecretData)
}

func newNatsBackend(spec kollectdevv1alpha1.KollectSinkSpec, ctx BuildContext) (Backend, error) {
	return natssink.NewBackend(spec, ctx.SecretData, ctx.CAPEM)
}
