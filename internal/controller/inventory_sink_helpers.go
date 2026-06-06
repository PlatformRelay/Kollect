// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink"
)

func loadResolvedSink(
	ctx context.Context,
	c client.Client,
	namespace string,
	binding kollectdevv1alpha1.InventorySinkBinding,
	clusterScoped bool,
) (*sink.ResolvedSink, error) {
	opts := sink.ResolveOptionsForBinding(namespace, binding, clusterScoped)
	resolved, err := sink.ResolveSink(ctx, c, opts)
	if err != nil {
		kind := familySinkKind(binding.Family, clusterScoped)
		return nil, fmt.Errorf("get %s %q: %w", kind, binding.Name, err)
	}

	return resolved, nil
}

func familySinkKind(family string, clusterScoped bool) string {
	prefix := "Kollect"
	if clusterScoped {
		prefix = "KollectCluster"
	}

	switch family {
	case kollectdevv1alpha1.SinkFamilySnapshot:
		return prefix + "SnapshotSink"
	case kollectdevv1alpha1.SinkFamilyDatabase:
		return prefix + "DatabaseSink"
	case kollectdevv1alpha1.SinkFamilyEvent:
		return prefix + "EventSink"
	default:
		return prefix + "Sink"
	}
}

func inventorySinkBindings(inv *kollectdevv1alpha1.KollectInventory) []kollectdevv1alpha1.InventorySinkBinding {
	if inv == nil {
		return nil
	}

	return kollectdevv1alpha1.CollectInventorySinkBindings(&inv.Spec)
}

func clusterInventorySinkBindings(inv *kollectdevv1alpha1.KollectClusterInventory) []kollectdevv1alpha1.InventorySinkBinding {
	if inv == nil {
		return nil
	}

	return kollectdevv1alpha1.CollectClusterInventorySinkBindings(&inv.Spec)
}

func totalInventorySinkRefs(inv *kollectdevv1alpha1.KollectInventory) int {
	return kollectdevv1alpha1.TotalInventorySinkRefCount(&inv.Spec)
}

func totalClusterInventorySinkRefs(inv *kollectdevv1alpha1.KollectClusterInventory) int {
	return kollectdevv1alpha1.TotalClusterInventorySinkRefCount(&inv.Spec)
}

func sinkExportKey(binding kollectdevv1alpha1.InventorySinkBinding) string {
	return binding.Family + "/" + binding.Name
}

func sinkExportMinInterval(resolved *sink.ResolvedSink) *metav1.Duration {
	if resolved == nil || resolved.ExportMinInterval == nil {
		return nil
	}
	return resolved.ExportMinInterval.ExportMinInterval
}

func loadClusterInventorySink(
	ctx context.Context,
	c client.Client,
	sinkNS string,
	binding kollectdevv1alpha1.InventorySinkBinding,
) (*sink.ResolvedSink, error) {
	resolved, err := loadResolvedSink(ctx, c, sinkNS, binding, false)
	if err == nil {
		return resolved, nil
	}
	if apierrors.IsNotFound(err) {
		return loadResolvedSink(ctx, c, sinkNS, binding, true)
	}
	return nil, err
}
