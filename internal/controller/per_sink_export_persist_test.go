// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
)

// makeStale bumps the stored object's resourceVersion so a status update of obj conflicts.
func makeStale(t *testing.T, cl client.Client, obj client.Object) {
	t.Helper()

	stored, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		t.Fatalf("DeepCopyObject of %T is not a client.Object", obj)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(obj), stored); err != nil {
		t.Fatalf("Get: %v", err)
	}
	stored.SetLabels(map[string]string{"bump": "1"})
	if err := cl.Update(context.Background(), stored); err != nil {
		t.Fatalf("Update: %v", err)
	}
}

// SDC-5: a debounce must never rest on an export whose record the status write lost. When the
// status update after a successful export fails, the next pass exports that sink again instead
// of debouncing it against an in-memory baseline the persisted entry does not show.
func TestKollectInventory_StatusWriteFailureAfterExport_ReExports(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		conflict      bool
		wantExported  int
		wantDebounced int
	}{
		{name: "status write conflicts", conflict: true, wantExported: 1, wantDebounced: 0},
		// Control: the write succeeds, so the next pass debounces as before.
		{name: "status write succeeds", conflict: false, wantExported: 0, wantDebounced: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
				ObjectMeta: metav1.ObjectMeta{Name: "pg-a", Namespace: "default", UID: "uid-pg-a", Generation: 1},
				Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
					Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
					Postgres: &kollectdevv1alpha1.PostgresSpec{
						DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-a"},
						Table:       "items",
					},
				},
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "pg-a", Namespace: "default"},
				Data:       map[string][]byte{"dsn": []byte("postgres://a")},
			}
			interval := metav1.Duration{Duration: 0}
			inv := &kollectdevv1alpha1.KollectInventory{
				ObjectMeta: metav1.ObjectMeta{Name: "team-inventory", Namespace: "default", Generation: 1, UID: "uid-inv"},
				Spec: kollectdevv1alpha1.KollectInventorySpec{
					ExportMinInterval: &interval,
					DatabaseSinkRefs:  kollectdevv1alpha1.NewSinkRefList("pg-a"),
				},
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(sinkObj, secret, inv).WithStatusSubresource(inv).Build()
			backend := &recordingBackend{}
			reg := sink.NewRegistry()
			reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
				return backend, nil
			})
			rec := &KollectInventoryReconciler{Client: cl, Scheme: scheme, Registry: reg}

			current := &kollectdevv1alpha1.KollectInventory{}
			if err := cl.Get(context.Background(), client.ObjectKeyFromObject(inv), current); err != nil {
				t.Fatal(err)
			}
			items := []collect.Item{{TargetNamespace: "default", TargetName: "web", UID: "u1", Name: "demo", Kind: "Deployment"}}
			invKey := "default/team-inventory"

			first := rec.exportToSinks(context.Background(), noopLogger{}, current, invKey, items, "c2")
			if first.ExportedCount != 1 {
				t.Fatalf("first pass exported %d, want 1", first.ExportedCount)
			}
			if tc.conflict {
				makeStale(t, cl, current)
			}
			if _, err := rec.updateStatus(context.Background(), current, len(items), first); err != nil {
				t.Fatalf("updateStatus: %v", err)
			}

			second := rec.exportToSinks(context.Background(), noopLogger{}, current, invKey, items, "c2")
			if second.ExportedCount != tc.wantExported || second.DebouncedCount != tc.wantDebounced {
				t.Fatalf("second pass = %d exported / %d debounced, want %d/%d",
					second.ExportedCount, second.DebouncedCount, tc.wantExported, tc.wantDebounced)
			}
		})
	}
}

// SDC-5, cluster kind: same rule on the cluster export path.
func TestKollectClusterInventory_StatusWriteFailureAfterExport_ReExports(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		conflict      bool
		wantExported  int
		wantDebounced int
	}{
		{name: "status write conflicts", conflict: true, wantExported: 1, wantDebounced: 0},
		{name: "status write succeeds", conflict: false, wantExported: 0, wantDebounced: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			const sinkNS = sink.DefaultSecretNamespace
			scheme := clusterRollupScheme(t)
			sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
				ObjectMeta: metav1.ObjectMeta{Name: "git-platform", Namespace: sinkNS, UID: "uid-git", Generation: 1},
				Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
					Type:             kollectdevv1alpha1.SnapshotSinkTypeGit,
					SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/inventory.git"},
				},
			}
			interval := metav1.Duration{Duration: 0}
			inv := &kollectdevv1alpha1.KollectClusterInventory{
				ObjectMeta: metav1.ObjectMeta{Name: "platform-rollup", Generation: 1, UID: "uid-cinv"},
				Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
					ExportMinInterval: &interval,
					SnapshotSinkRefs:  kollectdevv1alpha1.NewSinkRefList("git-platform"),
					SinkNamespace:     sinkNS,
				},
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(sinkObj, inv).WithStatusSubresource(sinkObj, inv).Build()
			backend := &recordingBackend{}
			reg := sink.NewRegistry()
			reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
				return backend, nil
			})
			rec := &KollectClusterInventoryReconciler{
				Client: cl, Scheme: scheme, Registry: reg, Recorder: record.NewFakeRecorder(10),
			}

			current := &kollectdevv1alpha1.KollectClusterInventory{}
			if err := cl.Get(context.Background(), client.ObjectKeyFromObject(inv), current); err != nil {
				t.Fatal(err)
			}
			items := []collect.Item{{TargetNamespace: "tenant-a", TargetName: "t", UID: "u1", Namespace: "tenant-a", Name: "app", Kind: "Deployment"}}
			invKey := "cluster/platform-rollup"

			first := rec.exportClusterToSinks(context.Background(), logr.Discard(), current, invKey, sinkNS, items, "c2")
			if first.ExportedCount != 1 {
				t.Fatalf("first pass exported %d, want 1", first.ExportedCount)
			}
			if tc.conflict {
				makeStale(t, cl, current)
			}
			if _, err := rec.updateStatus(context.Background(), current, 1, len(items), first, nil); err != nil {
				t.Fatalf("updateStatus: %v", err)
			}

			second := rec.exportClusterToSinks(context.Background(), logr.Discard(), current, invKey, sinkNS, items, "c2")
			if second.ExportedCount != tc.wantExported || second.DebouncedCount != tc.wantDebounced {
				t.Fatalf("second pass = %d exported / %d debounced, want %d/%d",
					second.ExportedCount, second.DebouncedCount, tc.wantExported, tc.wantDebounced)
			}
		})
	}
}
