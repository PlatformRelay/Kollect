// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"strings"

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
) (*sink.ResolvedSink, error) {
	opts := sink.ResolveOptionsForBinding(namespace, binding)
	resolved, err := sink.ResolveSink(ctx, c, opts)
	if err != nil {
		kind := familySinkKind(binding.Family)
		return nil, fmt.Errorf("get %s %q in %q: %w", kind, binding.Name, namespace, err)
	}

	return resolved, nil
}

func familySinkKind(family string) string {
	switch family {
	case kollectdevv1alpha1.SinkFamilySnapshot:
		return "KollectSnapshotSink"
	case kollectdevv1alpha1.SinkFamilyDatabase:
		return "KollectDatabaseSink"
	case kollectdevv1alpha1.SinkFamilyEvent:
		return "KollectEventSink"
	default:
		return "KollectSink"
	}
}

// sinkBindingNamespace resolves the namespace for a cluster inventory sink ref: the per-ref
// namespace when set, otherwise the inventory default sink namespace (ADR-0208).
func sinkBindingNamespace(binding kollectdevv1alpha1.InventorySinkBinding, defaultNS string) string {
	if ns := strings.TrimSpace(binding.Ref.Namespace); ns != "" {
		return binding.Ref.Namespace
	}

	return defaultNS
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

// inventorySinkFieldIndex is the cache field-index name for a KollectInventory's sink bindings.
// AR-09: sink-watch mappers do an indexed client.MatchingFields lookup on this index instead of
// listing every inventory in the namespace and filtering in memory, so a single sink event costs
// O(matching) rather than O(all inventories in namespace). Each indexed value is "<family>/<name>".
const inventorySinkFieldIndex = "spec.sinkBindings"

// indexInventorySinkBindings extracts the "<family>/<name>" keys a KollectInventory binds to,
// for registration via FieldIndexer. Returns nil for non-KollectInventory objects.
func indexInventorySinkBindings(obj client.Object) []string {
	inv, ok := obj.(*kollectdevv1alpha1.KollectInventory)
	if !ok {
		return nil
	}

	bindings := inventorySinkBindings(inv)
	keys := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		keys = append(keys, sinkExportKey(binding))
	}

	return keys
}

// clusterInventorySinkFieldIndex is the cache field-index name for a KollectClusterInventory's
// sink bindings. Each indexed value is "<family>/<name>/<effectiveNamespace>": cluster inventory
// refs resolve per-namespace (spec.sinkNamespace, or sink.DefaultSecretNamespace when unset —
// ADR-0208), so the watch mapper must also match the sink object's own namespace.
const clusterInventorySinkFieldIndex = "spec.sinkBindings"

// indexClusterInventorySinkBindings extracts the "<family>/<name>/<effectiveNamespace>" keys a
// KollectClusterInventory binds to. Returns nil for non-KollectClusterInventory objects.
func indexClusterInventorySinkBindings(obj client.Object) []string {
	inv, ok := obj.(*kollectdevv1alpha1.KollectClusterInventory)
	if !ok {
		return nil
	}

	invSinkNS := inv.Spec.SinkNamespace
	if invSinkNS == "" {
		invSinkNS = sink.DefaultSecretNamespace
	}

	bindings := clusterInventorySinkBindings(inv)
	keys := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		keys = append(keys, sinkExportKey(binding)+"/"+sinkBindingNamespace(binding, invSinkNS))
	}

	return keys
}

func sinkExportMinInterval(resolved *sink.ResolvedSink) *metav1.Duration {
	if resolved == nil || resolved.ExportMinInterval == nil {
		return nil
	}
	return resolved.ExportMinInterval.ExportMinInterval
}

// loadClusterInventorySink resolves a cluster inventory sink ref in its effective namespace:
// the per-ref namespace when set, otherwise the inventory default sink namespace. No cluster
// fallback resolution is attempted (ADR-0208).
func loadClusterInventorySink(
	ctx context.Context,
	c client.Client,
	sinkNS string,
	binding kollectdevv1alpha1.InventorySinkBinding,
) (*sink.ResolvedSink, error) {
	return loadResolvedSink(ctx, c, sinkBindingNamespace(binding, sinkNS), binding)
}
