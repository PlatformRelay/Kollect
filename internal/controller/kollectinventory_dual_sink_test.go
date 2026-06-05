// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink"
)

func TestKollectInventoryReconciler_exportToSinks_dualSinkIndependentDebounce(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "default",
		TargetName:      "web",
		UID:             "uid-1",
		Namespace:       "default",
		Name:            "demo",
		Version:         "v1",
		Kind:            "Deployment",
	})

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	longInterval := metav1.Duration{Duration: 5 * time.Minute}
	sinkA := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "sink-a", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: "postgres",
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-a"},
				Table:       "items",
			},
		},
	}
	sinkB := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "sink-b", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: "postgres",
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-b"},
				Table:       "items",
			},
		},
	}
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			ExportMinInterval: &longInterval,
			SinkRefs: kollectdevv1alpha1.InventorySinkRefList{
				{Name: "sink-a"},
				{Name: "sink-b", ExportMinInterval: &metav1.Duration{Duration: time.Minute}},
			},
		},
	}
	secretA := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-a", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://a")},
	}
	secretB := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-b", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://b")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkA, sinkB, inv, secretA, secretB).
		Build()

	recorder := &recordingBackend{}
	reg := sink.NewRegistry()
	reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Registry: reg,
	}

	items := store.SnapshotNamespace("default")
	checksum := "stable-checksum"
	invKey := "default/team-inventory"

	first := rec.exportToSinks(context.Background(), noopLogger{}, inv, invKey, items, checksum)
	if first.ExportedCount != 2 || first.DebouncedCount != 0 {
		t.Fatalf("first export = exported %d debounced %d, want 2/0", first.ExportedCount, first.DebouncedCount)
	}
	if len(first.SinkExports) != 2 {
		t.Fatalf("sinkExports = %d, want 2", len(first.SinkExports))
	}

	second := rec.exportToSinks(context.Background(), noopLogger{}, inv, invKey, items, checksum)
	if second.ExportedCount != 0 || second.DebouncedCount != 2 {
		t.Fatalf("second export = exported %d debounced %d, want 0/2", second.ExportedCount, second.DebouncedCount)
	}

	for _, exportStatus := range second.SinkExports {
		synced := apimeta.FindStatusCondition(exportStatus.Conditions, conditionSinkSynced)
		if synced == nil || synced.Reason != kollectdevv1alpha1.ReasonDebounced {
			t.Fatalf("sink %q condition = %#v, want Debounced", exportStatus.Name, synced)
		}
	}

	if len(recorder.exported) != 2 {
		t.Fatalf("backend export calls = %d, want 2 (debounce must not re-export)", len(recorder.exported))
	}
}

type noopLogger struct{}

func (noopLogger) Error(_ error, _ string, _ ...any) {}
