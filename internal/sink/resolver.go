// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// ResolvedSink holds a normalized sink spec loaded from a family CRD (ADR-0414).
type ResolvedSink struct {
	Spec              kollectdevv1alpha1.KollectSinkSpec
	ExportMinInterval *kollectdevv1alpha1.SinkCommonFields
	Namespace         string
	Name              string
	Family            string
	ClusterScoped     bool
	UID               types.UID
}

// ResolveOptions configures sink loading for inventory export.
type ResolveOptions struct {
	Namespace     string
	Name          string
	Family        string
	ClusterScoped bool
}

// ResolveSink loads a family sink CRD and normalizes it to KollectSinkSpec.
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
	if opts.ClusterScoped {
		var ks kollectdevv1alpha1.KollectClusterSnapshotSink
		if err := c.Get(ctx, client.ObjectKey{Name: opts.Name}, &ks); err != nil {
			return nil, err
		}
		return &ResolvedSink{
			Spec:              ks.Spec.ToKollectSinkSpec(),
			ExportMinInterval: &ks.Spec.SinkCommonFields,
			Name:              opts.Name,
			Family:            kollectdevv1alpha1.SinkFamilySnapshot,
			ClusterScoped:     true,
			UID:               ks.UID,
		}, nil
	}
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
	}, nil
}

func resolveDatabaseSink(ctx context.Context, c client.Client, opts ResolveOptions) (*ResolvedSink, error) {
	if opts.ClusterScoped {
		var ks kollectdevv1alpha1.KollectClusterDatabaseSink
		if err := c.Get(ctx, client.ObjectKey{Name: opts.Name}, &ks); err != nil {
			return nil, err
		}
		return &ResolvedSink{
			Spec:              ks.Spec.ToKollectSinkSpec(),
			ExportMinInterval: &ks.Spec.SinkCommonFields,
			Name:              opts.Name,
			Family:            kollectdevv1alpha1.SinkFamilyDatabase,
			ClusterScoped:     true,
			UID:               ks.UID,
		}, nil
	}
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
	}, nil
}

func resolveEventSink(ctx context.Context, c client.Client, opts ResolveOptions) (*ResolvedSink, error) {
	if opts.ClusterScoped {
		var ks kollectdevv1alpha1.KollectClusterEventSink
		if err := c.Get(ctx, client.ObjectKey{Name: opts.Name}, &ks); err != nil {
			return nil, err
		}
		return &ResolvedSink{
			Spec:              ks.Spec.ToKollectSinkSpec(),
			ExportMinInterval: &ks.Spec.SinkCommonFields,
			Name:              opts.Name,
			Family:            kollectdevv1alpha1.SinkFamilyEvent,
			ClusterScoped:     true,
			UID:               ks.UID,
		}, nil
	}
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

// ResolveOptionsForBinding builds resolve options for an inventory binding.
func ResolveOptionsForBinding(
	namespace string,
	binding kollectdevv1alpha1.InventorySinkBinding,
	clusterScoped bool,
) ResolveOptions {
	opts := ResolveOptions{Name: binding.Name, Family: binding.Family, ClusterScoped: clusterScoped}
	if !clusterScoped {
		opts.Namespace = namespace
	}
	return opts
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
