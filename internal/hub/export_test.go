// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"context"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/hub"
	"github.com/konih/kollect/internal/sink"
)

type recordingBackend struct {
	mu       sync.Mutex
	exported int
}

func (r *recordingBackend) Type() string { return "recording" }

func (r *recordingBackend) Export(_ context.Context, _ []byte, _ string) error {
	r.mu.Lock()
	r.exported++
	r.mu.Unlock()

	return nil
}

func (r *recordingBackend) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.exported
}

func TestExporterParallelFanOut(t *testing.T) {
	t.Parallel()

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "spoke-a",
		TargetName:      "team-inventory",
		Namespace:       "apps",
		Name:            "web",
		UID:             "uid-1",
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

	pgSink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "hub-postgres", Namespace: "platform"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: "postgres",
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}
	kafkaSink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "hub-kafka", Namespace: "platform"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "kafka"},
	}
	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "platform"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pgSink, kafkaSink, pgSecret).
		Build()

	pgRecorder := &recordingBackend{}
	kafkaRecorder := &recordingBackend{}
	reg := sink.NewRegistry()
	reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return pgRecorder, nil
	})
	reg.Register("kafka", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return kafkaRecorder, nil
	})

	exporter := &hub.Exporter{
		Store:    store,
		Client:   cl,
		Registry: reg,
		Config: hub.ExportConfig{
			ExportNamespace: "platform",
			SinkRefs:        []string{"hub-postgres", "hub-kafka"},
		},
	}

	report := hub.SpokeReport{
		Cluster: "spoke-a",
		InventoryRef: hub.InventoryRef{
			Namespace: "team-a",
			Name:      "team-inventory",
		},
	}

	if err := exporter.ExportAfterMerge(context.Background(), report); err != nil {
		t.Fatalf("ExportAfterMerge: %v", err)
	}

	if pgRecorder.count() != 1 {
		t.Fatalf("postgres exports = %d, want 1", pgRecorder.count())
	}

	if kafkaRecorder.count() != 1 {
		t.Fatalf("kafka exports = %d, want 1", kafkaRecorder.count())
	}
}
