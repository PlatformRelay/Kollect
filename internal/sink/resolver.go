// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// ResolvedSink holds a normalized sink spec loaded from a namespaced family CRD (ADR-0414, ADR-0208).
type ResolvedSink struct {
	Spec              kollectdevv1alpha1.KollectSinkSpec
	ExportMinInterval *kollectdevv1alpha1.SinkCommonFields
	Namespace         string
	Name              string
	Family            string
	UID               types.UID
	// Generation is the family sink object's metadata.generation; a spec edit bumps it.
	Generation int64
}

// ResolveOptions configures sink loading for inventory export.
type ResolveOptions struct {
	Namespace string
	Name      string
	Family    string
}

// ResolveSink loads a namespaced family sink CRD and normalizes it to KollectSinkSpec.
func ResolveSink(ctx context.Context, c client.Client, opts ResolveOptions) (*ResolvedSink, error) {
	if c == nil {
		return nil, fmt.Errorf("client is required")
	}
	if opts.Name == "" {
		return nil, fmt.Errorf("sink name is required")
	}
	if opts.Family == "" {
		return resolveSinkAnyFamily(ctx, c, opts)
	}

	switch opts.Family {
	case kollectdevv1alpha1.SinkFamilySnapshot:
		return resolveSnapshotSink(ctx, c, opts)
	case kollectdevv1alpha1.SinkFamilyDatabase:
		return resolveDatabaseSink(ctx, c, opts)
	case kollectdevv1alpha1.SinkFamilyEvent:
		return resolveEventSink(ctx, c, opts)
	default:
		return nil, fmt.Errorf("unknown sink family %q", opts.Family)
	}
}

func resolveSnapshotSink(ctx context.Context, c client.Client, opts ResolveOptions) (*ResolvedSink, error) {
	var ks kollectdevv1alpha1.KollectSnapshotSink
	if err := c.Get(ctx, client.ObjectKey{Namespace: opts.Namespace, Name: opts.Name}, &ks); err != nil {
		return nil, err
	}
	return &ResolvedSink{
		Spec:              ks.Spec.ToKollectSinkSpec(),
		ExportMinInterval: &ks.Spec.SinkCommonFields,
		Namespace:         opts.Namespace,
		Name:              opts.Name,
		Family:            kollectdevv1alpha1.SinkFamilySnapshot,
		UID:               ks.UID,
		Generation:        ks.Generation,
	}, nil
}

func resolveDatabaseSink(ctx context.Context, c client.Client, opts ResolveOptions) (*ResolvedSink, error) {
	var ks kollectdevv1alpha1.KollectDatabaseSink
	if err := c.Get(ctx, client.ObjectKey{Namespace: opts.Namespace, Name: opts.Name}, &ks); err != nil {
		return nil, err
	}
	return &ResolvedSink{
		Spec:              ks.Spec.ToKollectSinkSpec(),
		ExportMinInterval: &ks.Spec.SinkCommonFields,
		Namespace:         opts.Namespace,
		Name:              opts.Name,
		Family:            kollectdevv1alpha1.SinkFamilyDatabase,
		UID:               ks.UID,
		Generation:        ks.Generation,
	}, nil
}

func resolveEventSink(ctx context.Context, c client.Client, opts ResolveOptions) (*ResolvedSink, error) {
	var ks kollectdevv1alpha1.KollectEventSink
	if err := c.Get(ctx, client.ObjectKey{Namespace: opts.Namespace, Name: opts.Name}, &ks); err != nil {
		return nil, err
	}
	return &ResolvedSink{
		Spec:              ks.Spec.ToKollectSinkSpec(),
		ExportMinInterval: &ks.Spec.SinkCommonFields,
		Namespace:         opts.Namespace,
		Name:              opts.Name,
		Family:            kollectdevv1alpha1.SinkFamilyEvent,
		UID:               ks.UID,
		Generation:        ks.Generation,
	}, nil
}

// SinkNamespaceForResolved returns the namespace used for secret resolution and backend pool keys.
func SinkNamespaceForResolved(resolved *ResolvedSink, fallback string) string {
	if resolved == nil {
		return fallback
	}
	if resolved.Namespace != "" {
		return resolved.Namespace
	}
	return fallback
}

// ResolveOptionsForBinding builds resolve options for an inventory binding in the given namespace.
func ResolveOptionsForBinding(
	namespace string,
	binding kollectdevv1alpha1.InventorySinkBinding,
) ResolveOptions {
	return ResolveOptions{Namespace: namespace, Name: binding.Name, Family: binding.Family}
}

func resolveSinkAnyFamily(ctx context.Context, c client.Client, opts ResolveOptions) (*ResolvedSink, error) {
	families := []string{
		kollectdevv1alpha1.SinkFamilySnapshot,
		kollectdevv1alpha1.SinkFamilyDatabase,
		kollectdevv1alpha1.SinkFamilyEvent,
	}
	var lastErr error
	for _, family := range families {
		try := opts
		try.Family = family
		var resolved *ResolvedSink
		var err error
		switch family {
		case kollectdevv1alpha1.SinkFamilySnapshot:
			resolved, err = resolveSnapshotSink(ctx, c, try)
		case kollectdevv1alpha1.SinkFamilyDatabase:
			resolved, err = resolveDatabaseSink(ctx, c, try)
		case kollectdevv1alpha1.SinkFamilyEvent:
			resolved, err = resolveEventSink(ctx, c, try)
		}
		if err == nil {
			return resolved, nil
		}
		if apierrors.IsNotFound(err) {
			lastErr = err
			continue
		}
		return nil, err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("sink %q not found", opts.Name)
}
