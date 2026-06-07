// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestNormalizeEngineConfig_resyncAndMetrics(t *testing.T) {
	t.Parallel()

	cfg := normalizeEngineConfig(EngineConfig{
		ResyncPeriod:          6 * time.Hour,
		MetricsSampleInterval: 10 * time.Second,
		DispatchEnqueueWait:   5 * time.Millisecond,
	})
	if cfg.ResyncPeriod != 6*time.Hour {
		t.Fatalf("ResyncPeriod = %v", cfg.ResyncPeriod)
	}
	if cfg.MetricsSampleInterval != 10*time.Second {
		t.Fatalf("MetricsSampleInterval = %v", cfg.MetricsSampleInterval)
	}
}

func TestIsInformerResync(t *testing.T) {
	t.Parallel()

	old := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name": "demo", "namespace": "default", "resourceVersion": "100",
		},
	}}
	same := old.DeepCopy()
	changed := old.DeepCopy()
	changed.SetResourceVersion("101")

	if !isInformerResync(old, same) {
		t.Fatal("expected resync when resourceVersion unchanged")
	}
	if isInformerResync(old, changed) {
		t.Fatal("expected not resync when resourceVersion changed")
	}
}

func TestEngineMetricsSampleThrottle(t *testing.T) {
	t.Parallel()

	store := NewStore()
	engine, err := NewEngine(nil, nil, store, EngineConfig{MetricsSampleInterval: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	st := targetState{
		target: kollectdevv1alpha1.KollectTarget{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "apps"}},
		profile: kollectdevv1alpha1.KollectProfile{
			Spec: kollectdevv1alpha1.KollectProfileSpec{
				TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			},
		},
	}
	engine.store.Upsert(Item{
		TargetNamespace: "apps", TargetName: "web", Namespace: "apps", Name: "demo",
		UID: "uid-1", Version: "v1", Kind: "Deployment",
		Attributes: map[string]any{"replicas": float64(1)},
	})

	engine.refreshTargetSnapshotMetrics(st, st.target)
	before := len(engine.metricsLastRefresh)
	engine.refreshTargetSnapshotMetrics(st, st.target)
	if len(engine.metricsLastRefresh) != before {
		t.Fatal("expected throttled refresh within sample interval")
	}
}
