// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	kollecterrors "github.com/konih/kollect/internal/errors"
	"github.com/konih/kollect/internal/sink"
)

type failingBackend struct {
	err error
}

func (f *failingBackend) Type() string { return "failing" }

func (f *failingBackend) Capabilities() sink.Capabilities {
	return sink.SnapshotStoreCapabilities()
}

func (f *failingBackend) Export(_ context.Context, _ []byte, _ string) error {
	return f.err
}

func TestKollectInventoryReconciler_exportToSinks_continuesOnPartialFailure(t *testing.T) {
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

	sinkOK := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "sink-ok", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: "postgres",
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-ok"},
				Table:       "items",
			},
		},
	}
	sinkFail := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "sink-fail", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: "postgres",
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-fail"},
				Table:       "fail",
			},
		},
	}
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			SinkRefs: kollectdevv1alpha1.InventorySinkRefList{
				{Name: "sink-fail"},
				{Name: "sink-ok"},
			},
		},
	}
	secretOK := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-ok", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://ok")},
	}
	secretFail := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-fail", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://fail")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkOK, sinkFail, inv, secretOK, secretFail).
		Build()

	recorder := &recordingBackend{}
	exportErr := kollecterrors.Transient(errors.New("sink unavailable"))
	reg := sink.NewRegistry()
	reg.Register("postgres", func(spec kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		if spec.Postgres != nil && spec.Postgres.Table == "fail" {
			return &failingBackend{err: exportErr}, nil
		}
		return recorder, nil
	})

	rec := &KollectInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Store:    store,
		Registry: reg,
	}

	outcome := rec.exportToSinks(
		context.Background(),
		noopLogger{},
		inv,
		"default/team-inventory",
		store.SnapshotNamespace("default"),
		"checksum",
	)
	if outcome.ExportedCount != 1 || outcome.FailedCount != 1 {
		t.Fatalf("outcome = exported %d failed %d, want 1/1", outcome.ExportedCount, outcome.FailedCount)
	}
	if outcome.ExportErr == nil {
		t.Fatal("expected last export error")
	}
	if len(outcome.SinkExports) != 2 {
		t.Fatalf("sinkExports = %d, want 2", len(outcome.SinkExports))
	}

	okSynced := apimeta.FindStatusCondition(outcome.SinkExports[1].Conditions, conditionSinkSynced)
	if okSynced == nil || okSynced.Status != metav1.ConditionTrue {
		t.Fatalf("ok sink condition = %#v", okSynced)
	}
	failSynced := apimeta.FindStatusCondition(outcome.SinkExports[0].Conditions, conditionSinkSynced)
	if failSynced == nil || failSynced.Reason != reasonExportFailed {
		t.Fatalf("failed sink condition = %#v", failSynced)
	}
	if len(recorder.exported) != 1 {
		t.Fatalf("backend export calls = %d, want 1 (must continue after first sink failure)", len(recorder.exported))
	}
}
