// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
)

// The key under test is the API constant kollectdevv1alpha1.AnnotationRequestedAt
// (added by T05, as planned when these tests were written): these tests pin the
// spelling the reconcilers must read.

func newRequestedAtNamespacedHarness(
	t *testing.T,
	annotations map[string]string,
) (*KollectInventoryReconciler, *kollectdevv1alpha1.KollectInventory, *recordingBackend, []collect.Item) {
	t.Helper()

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
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "team-inventory",
			Namespace:   "default",
			Generation:  1,
			Annotations: annotations,
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			ExportMinInterval: &longInterval,
			DatabaseSinkRefs:  kollectdevv1alpha1.InventorySinkRefList{{Name: "sink-a"}},
		},
	}
	sinkObj := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "sink-a", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "items",
			},
		},
	}
	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "default"},
		Data:       map[string][]byte{"dsn": []byte("postgres://example")},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv, pgSecret).
		WithStatusSubresource(inv).
		Build()

	recorder := &recordingBackend{}
	reg := sink.NewRegistry()
	reg.Register("postgres", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectInventoryReconciler{Client: cl, Scheme: scheme, Store: store, Registry: reg}

	return rec, inv, recorder, store.SnapshotNamespace("default")
}

func newRequestedAtClusterHarness(
	t *testing.T,
	annotations map[string]string,
) (*KollectClusterInventoryReconciler, *kollectdevv1alpha1.KollectClusterInventory, *recordingBackend, []collect.Item) {
	t.Helper()

	const sinkNS = sink.DefaultSecretNamespace

	items := []collect.Item{{
		TargetNamespace: "tenant-a", TargetName: "platform-deployments", UID: "uid-0",
		Namespace: "tenant-a", Name: "app", Version: "v1", Kind: "Deployment",
		Attributes: map[string]any{"image": "nginx:1.27"},
	}}

	scheme := clusterRollupScheme(t)

	sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git-platform", Namespace: sinkNS},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:             kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/inventory.git"},
		},
	}

	longInterval := metav1.Duration{Duration: 5 * time.Minute}
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-rollup", Annotations: annotations},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			ExportMinInterval: &longInterval,
			SnapshotSinkRefs:  kollectdevv1alpha1.NewSinkRefList("git-platform"),
			SinkNamespace:     sinkNS,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(sinkObj, inv).
		WithStatusSubresource(sinkObj, inv).
		Build()

	recorder := &recordingBackend{}
	reg := sink.NewRegistry()
	reg.Register("git", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
		return recorder, nil
	})

	rec := &KollectClusterInventoryReconciler{
		Client:   cl,
		Scheme:   scheme,
		Registry: reg,
		Recorder: record.NewFakeRecorder(10),
	}

	return rec, inv, recorder, items
}

// TestKollectInventoryReconciler_requestedAt_changedValueReexports is the
// ERA-1 differential lock on the namespaced export path: after a debounced
// steady state, changing only the kollect.dev/requestedAt value (same
// generation, same checksum) must force exactly one export, and the following
// unchanged reconcile must debounce again.
func TestKollectInventoryReconciler_requestedAt_changedValueReexports(t *testing.T) {
	t.Parallel()

	rec, inv, recorder, items := newRequestedAtNamespacedHarness(t,
		map[string]string{kollectdevv1alpha1.AnnotationRequestedAt: "2026-10-06T10:00:00Z"})

	bg := context.Background()
	invKey := "default/team-inventory"
	const checksum = "fingerprint-a"

	first := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if first.ExportedCount != 1 || first.DebouncedCount != 0 {
		t.Fatalf("first export = %d exported / %d debounced, want 1/0", first.ExportedCount, first.DebouncedCount)
	}

	second := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if second.ExportedCount != 0 || second.DebouncedCount != 1 {
		t.Fatalf("unchanged steady state = %d exported / %d debounced, want 0/1", second.ExportedCount, second.DebouncedCount)
	}

	inv.Annotations[kollectdevv1alpha1.AnnotationRequestedAt] = "2026-10-06T10:05:00Z"
	third := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if third.ExportedCount != 1 || third.DebouncedCount != 0 {
		t.Fatalf("changed requestedAt = %d exported / %d debounced, want 1/0 "+
			"(the annotation is not read, so the debounce still fires)", third.ExportedCount, third.DebouncedCount)
	}

	fourth := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if fourth.ExportedCount != 0 || fourth.DebouncedCount != 1 {
		t.Fatalf("steady state after the forced export = %d exported / %d debounced, want 0/1",
			fourth.ExportedCount, fourth.DebouncedCount)
	}

	if got := len(recorder.exported); got != 2 {
		t.Fatalf("backend export calls = %d, want 2 (first and forced export)", got)
	}
}

// TestKollectInventoryReconciler_requestedAt_absenceToPresent locks the
// absence→present transition: an annotation appearing on an inventory that
// exported without one must bypass the debounce exactly once.
func TestKollectInventoryReconciler_requestedAt_absenceToPresent(t *testing.T) {
	t.Parallel()

	rec, inv, _, items := newRequestedAtNamespacedHarness(t, nil)

	bg := context.Background()
	invKey := "default/team-inventory"
	const checksum = "fingerprint-a"

	first := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if first.ExportedCount != 1 || first.DebouncedCount != 0 {
		t.Fatalf("first export = %d exported / %d debounced, want 1/0", first.ExportedCount, first.DebouncedCount)
	}
	second := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if second.DebouncedCount != 1 {
		t.Fatalf("unchanged steady state = %d debounced, want 1", second.DebouncedCount)
	}

	inv.Annotations = map[string]string{kollectdevv1alpha1.AnnotationRequestedAt: "2026-10-06T10:05:00Z"}
	third := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if third.ExportedCount != 1 || third.DebouncedCount != 0 {
		t.Fatalf("absence→present = %d exported / %d debounced, want 1/0 "+
			"(absence must count as a value)", third.ExportedCount, third.DebouncedCount)
	}

	assertSyncedAsForAnyExport(t, rec, inv, len(items), third)
}

// assertSyncedAsForAnyExport drives updateStatus and locks the "Absence is a
// value, not a wildcard" clause: after the forced export the Synced condition
// and requeue cadence read exactly as for any other successful export, not as
// a special forced-sync marker.
//
// The status write runs on a deep copy: the fake client's status-subresource
// update restores non-status fields (annotations included) from the stored
// object onto the passed one (controller-runtime v0.24.1 versioned_tracker.go
// copyFrom at :240), which would otherwise resurrect an annotation the test
// removed mid-test and break the following steady-state phase.
func assertSyncedAsForAnyExport(
	t *testing.T,
	rec *KollectInventoryReconciler,
	inv *kollectdevv1alpha1.KollectInventory,
	itemCount int,
	outcome perSinkExportOutcome,
) {
	t.Helper()

	invCopy := inv.DeepCopy()
	result, err := rec.updateStatus(context.Background(), invCopy, itemCount, outcome)
	if err != nil {
		t.Fatalf("updateStatus: %v", err)
	}

	synced := apimeta.FindStatusCondition(invCopy.Status.Conditions, kollectdevv1alpha1.ConditionSynced)
	if synced == nil || synced.Status != metav1.ConditionTrue || synced.Reason != "Exported" {
		t.Fatalf("Synced condition after the forced export = %+v, want True/Exported as for any other export", synced)
	}
	if synced.Message != "exported to 1 sink(s)" {
		t.Fatalf("Synced message after the forced export = %q, want \"exported to 1 sink(s)\"", synced.Message)
	}

	if want := 5 * time.Minute; result.RequeueAfter != want {
		t.Fatalf("RequeueAfter after the forced export = %v, want %v (cadence stays as for any other export)",
			result.RequeueAfter, want)
	}
}

// TestKollectInventoryReconciler_requestedAt_presentToAbsence locks the
// present→absence transition: removing the annotation after a recorded export
// must bypass the debounce exactly once.
func TestKollectInventoryReconciler_requestedAt_presentToAbsence(t *testing.T) {
	t.Parallel()

	rec, inv, _, items := newRequestedAtNamespacedHarness(t,
		map[string]string{kollectdevv1alpha1.AnnotationRequestedAt: "2026-10-06T10:00:00Z"})

	bg := context.Background()
	invKey := "default/team-inventory"
	const checksum = "fingerprint-a"

	first := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if first.ExportedCount != 1 || first.DebouncedCount != 0 {
		t.Fatalf("first export = %d exported / %d debounced, want 1/0", first.ExportedCount, first.DebouncedCount)
	}

	inv.Annotations = nil
	second := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if second.ExportedCount != 1 || second.DebouncedCount != 0 {
		t.Fatalf("present→absence = %d exported / %d debounced, want 1/0 "+
			"(removal must count as a change)", second.ExportedCount, second.DebouncedCount)
	}
	assertSyncedAsForAnyExport(t, rec, inv, len(items), second)

	third := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if third.ExportedCount != 0 || third.DebouncedCount != 1 {
		t.Fatalf("steady state after the forced export = %d exported / %d debounced, want 0/1",
			third.ExportedCount, third.DebouncedCount)
	}
}

// TestKollectClusterInventoryReconciler_requestedAt_changedValueReexports is
// the ERA-1 cluster-path parity lock: the cluster export path must bypass the
// debounce exactly once on a requestedAt change, like the namespaced path.
func TestKollectClusterInventoryReconciler_requestedAt_changedValueReexports(t *testing.T) {
	t.Parallel()

	rec, inv, recorder, items := newRequestedAtClusterHarness(t,
		map[string]string{kollectdevv1alpha1.AnnotationRequestedAt: "2026-10-06T10:00:00Z"})

	bg := context.Background()
	invKey := "cluster/platform-rollup"
	const sinkNS = sink.DefaultSecretNamespace
	const checksum = "fingerprint-a"

	first := rec.exportClusterToSinks(bg, logr.Discard(), inv, invKey, sinkNS, items, checksum)
	if first.ExportedCount != 1 || first.DebouncedCount != 0 {
		t.Fatalf("first export = %d exported / %d debounced, want 1/0", first.ExportedCount, first.DebouncedCount)
	}

	second := rec.exportClusterToSinks(bg, logr.Discard(), inv, invKey, sinkNS, items, checksum)
	if second.ExportedCount != 0 || second.DebouncedCount != 1 {
		t.Fatalf("unchanged steady state = %d exported / %d debounced, want 0/1", second.ExportedCount, second.DebouncedCount)
	}

	inv.Annotations[kollectdevv1alpha1.AnnotationRequestedAt] = "2026-10-06T10:05:00Z"
	third := rec.exportClusterToSinks(bg, logr.Discard(), inv, invKey, sinkNS, items, checksum)
	if third.ExportedCount != 1 || third.DebouncedCount != 0 {
		t.Fatalf("changed requestedAt (cluster path) = %d exported / %d debounced, want 1/0 "+
			"(the annotation is not read, so the debounce still fires)", third.ExportedCount, third.DebouncedCount)
	}

	fourth := rec.exportClusterToSinks(bg, logr.Discard(), inv, invKey, sinkNS, items, checksum)
	if fourth.ExportedCount != 0 || fourth.DebouncedCount != 1 {
		t.Fatalf("steady state after the forced export = %d exported / %d debounced, want 0/1",
			fourth.ExportedCount, fourth.DebouncedCount)
	}

	if got := len(recorder.exported); got != 2 {
		t.Fatalf("backend export calls = %d, want 2 (first and forced export)", got)
	}
}

// TestKollectClusterInventoryReconciler_requestedAt_presenceTransitions locks
// both absence transitions on the cluster path, completing the parity claim.
func TestKollectClusterInventoryReconciler_requestedAt_presenceTransitions(t *testing.T) {
	t.Parallel()

	rec, inv, _, items := newRequestedAtClusterHarness(t, nil)

	bg := context.Background()
	invKey := "cluster/platform-rollup"
	const sinkNS = sink.DefaultSecretNamespace
	const checksum = "fingerprint-a"

	first := rec.exportClusterToSinks(bg, logr.Discard(), inv, invKey, sinkNS, items, checksum)
	if first.ExportedCount != 1 || first.DebouncedCount != 0 {
		t.Fatalf("first export = %d exported / %d debounced, want 1/0", first.ExportedCount, first.DebouncedCount)
	}
	second := rec.exportClusterToSinks(bg, logr.Discard(), inv, invKey, sinkNS, items, checksum)
	if second.DebouncedCount != 1 {
		t.Fatalf("unchanged steady state = %d debounced, want 1", second.DebouncedCount)
	}

	inv.Annotations = map[string]string{kollectdevv1alpha1.AnnotationRequestedAt: "2026-10-06T10:05:00Z"}
	third := rec.exportClusterToSinks(bg, logr.Discard(), inv, invKey, sinkNS, items, checksum)
	if third.ExportedCount != 1 || third.DebouncedCount != 0 {
		t.Fatalf("absence→present (cluster path) = %d exported / %d debounced, want 1/0",
			third.ExportedCount, third.DebouncedCount)
	}
	assertClusterSyncedAsForAnyExport(t, rec, inv, third)

	inv.Annotations = nil
	fourth := rec.exportClusterToSinks(bg, logr.Discard(), inv, invKey, sinkNS, items, checksum)
	if fourth.ExportedCount != 1 || fourth.DebouncedCount != 0 {
		t.Fatalf("present→absence (cluster path) = %d exported / %d debounced, want 1/0",
			fourth.ExportedCount, fourth.DebouncedCount)
	}
}

// assertClusterSyncedAsForAnyExport is the cluster-path twin of
// assertSyncedAsForAnyExport: after the forced re-export the Synced condition
// and requeue cadence read exactly as for any other cluster export. Like the
// namespaced helper, the status write runs on a deep copy so the fake client's
// metadata restore cannot resurrect an annotation the test removed mid-test.
func assertClusterSyncedAsForAnyExport(
	t *testing.T,
	rec *KollectClusterInventoryReconciler,
	inv *kollectdevv1alpha1.KollectClusterInventory,
	outcome perSinkExportOutcome,
) {
	t.Helper()

	invCopy := inv.DeepCopy()
	result, err := rec.updateStatus(context.Background(), invCopy, 1, 1, outcome, nil)
	if err != nil {
		t.Fatalf("cluster updateStatus: %v", err)
	}

	synced := apimeta.FindStatusCondition(invCopy.Status.Conditions, kollectdevv1alpha1.ConditionSynced)
	if synced == nil || synced.Status != metav1.ConditionTrue || synced.Reason != "Exported" {
		t.Fatalf("cluster Synced condition after the forced export = %+v, want True/Exported as for any other export", synced)
	}
	if synced.Message != "exported to 1 sink(s)" {
		t.Fatalf("cluster Synced message after the forced export = %q, want \"exported to 1 sink(s)\"", synced.Message)
	}

	if want := 5 * time.Minute; result.RequeueAfter != want {
		t.Fatalf("cluster RequeueAfter after the forced export = %v, want %v (cadence stays as for any other export)",
			result.RequeueAfter, want)
	}
}

// TestKollectInventoryReconciler_requestedAt_changeExportsEverySinkBinding
// locks the SHALL clause "the next reconcile SHALL export to every sink
// binding that would otherwise be debounced": a requestedAt change bypasses
// each binding's debounce independently, not just the first.
func TestKollectInventoryReconciler_requestedAt_changeExportsEverySinkBinding(t *testing.T) {
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
	sinkA := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "sink-a", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-a"},
				Table:       "items",
			},
		},
	}
	sinkB := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "sink-b", Namespace: "default"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg-b"},
				Table:       "items",
			},
		},
	}
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "team-inventory",
			Namespace:   "default",
			Generation:  1,
			Annotations: map[string]string{kollectdevv1alpha1.AnnotationRequestedAt: "2026-10-06T10:00:00Z"},
		},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			ExportMinInterval: &longInterval,
			DatabaseSinkRefs: kollectdevv1alpha1.InventorySinkRefList{
				{Name: "sink-a"},
				{Name: "sink-b"},
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

	rec := &KollectInventoryReconciler{Client: cl, Scheme: scheme, Store: store, Registry: reg}

	bg := context.Background()
	invKey := "default/team-inventory"
	const checksum = "fingerprint-a"
	items := store.SnapshotNamespace("default")

	first := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if first.ExportedCount != 2 || first.DebouncedCount != 0 {
		t.Fatalf("first export = %d exported / %d debounced, want 2/0", first.ExportedCount, first.DebouncedCount)
	}

	second := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if second.ExportedCount != 0 || second.DebouncedCount != 2 {
		t.Fatalf("unchanged steady state = %d exported / %d debounced, want 0/2", second.ExportedCount, second.DebouncedCount)
	}

	inv.Annotations[kollectdevv1alpha1.AnnotationRequestedAt] = "2026-10-06T10:05:00Z"
	third := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if third.ExportedCount != 2 || third.DebouncedCount != 0 {
		t.Fatalf("changed requestedAt = %d exported / %d debounced, want 2/0 "+
			"(every sink binding the change debounces must export)", third.ExportedCount, third.DebouncedCount)
	}

	if got := len(recorder.exported); got != 4 {
		t.Fatalf("backend export calls = %d, want 4 (two steady, two forced)", got)
	}
}

// TestKollectInventoryReconciler_preview_requestedAtChangeNotDebounced locks
// the ERA-1 preview-honesty scenario: after a requestedAt change, the preview
// must not report the affected bindings as debounced, because the next export
// will not debounce them.
func TestKollectInventoryReconciler_preview_requestedAtChangeNotDebounced(t *testing.T) {
	t.Parallel()

	rec, inv, _, items := newRequestedAtNamespacedHarness(t,
		map[string]string{kollectdevv1alpha1.AnnotationRequestedAt: "2026-10-06T10:00:00Z"})

	bg := context.Background()
	invKey := "default/team-inventory"
	const checksum = "fingerprint-a"

	first := rec.exportToSinks(bg, noopLogger{}, inv, invKey, items, checksum)
	if first.ExportedCount != 1 || first.DebouncedCount != 0 {
		t.Fatalf("first export = %d exported / %d debounced, want 1/0", first.ExportedCount, first.DebouncedCount)
	}

	if _, allDebounced := rec.previewAllSinksDebounced(bg, inv, invKey, checksum); !allDebounced {
		t.Fatal("preview = false, want true for an unchanged annotation (steady state)")
	}

	inv.Annotations[kollectdevv1alpha1.AnnotationRequestedAt] = "2026-10-06T10:05:00Z"
	if _, allDebounced := rec.previewAllSinksDebounced(bg, inv, invKey, checksum); allDebounced {
		t.Fatal("preview reports the bindings as debounced after a requestedAt change, " +
			"but the next export will not debounce them")
	}
}
