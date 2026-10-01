// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"strings"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// K-23 test lock: no writer named by the finding may persist credential-bearing
// error text into a status condition message or an Event message. Each test
// feeds the writer an error whose text embeds a credential-bearing URL, then
// asserts the persisted artifact is redacted and the reason/classification is
// unchanged. Masking of raw secret VALUES is the sink-side contract (only the
// sink knows them); the controller choke-point masks URL userinfo.

const (
	leakURL    = "https://user:s3cr3t@nats.example.com:4222" //nolint:gosec // G101: fake credential fixture for the redaction contract
	leakReason = "ConnectionTestFailed"
)

func leakErr() error {
	return kollecterrors.Terminal(errors.New("dial " + leakURL + " rejected"))
}

func assertNoSecret(t *testing.T, what, msg string) {
	t.Helper()

	for _, leak := range []string{"s3cr3t", "user@"} {
		if strings.Contains(msg, leak) {
			t.Fatalf("%s leaked %q: %q", what, leak, msg)
		}
	}
}

func controllerScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	return scheme
}

func TestRecordWarning_redactsEvent(t *testing.T) {
	t.Parallel()

	rec := record.NewFakeRecorder(4)
	inv := &kollectdevv1alpha1.KollectInventory{ObjectMeta: metav1.ObjectMeta{Name: "i", Namespace: "ns"}}

	recordWarning(rec, inv, leakReason, leakErr().Error())

	select {
	case event := <-rec.Events:
		assertNoSecret(t, "Event message", event)

		if !strings.Contains(event, "https://***@nats.example.com:4222") {
			t.Fatalf("event lost the redacted host routing: %q", event)
		}
	default:
		t.Fatal("no event recorded")
	}
}

func TestRecordNormal_redactsEvent(t *testing.T) {
	t.Parallel()

	rec := record.NewFakeRecorder(4)
	inv := &kollectdevv1alpha1.KollectInventory{ObjectMeta: metav1.ObjectMeta{Name: "i", Namespace: "ns"}}

	recordNormal(rec, inv, "Synced", "done for "+leakURL)

	select {
	case event := <-rec.Events:
		assertNoSecret(t, "Normal Event message", event)
	default:
		t.Fatal("no event recorded")
	}
}

func TestSetSyncedCondition_redactsMessage(t *testing.T) {
	t.Parallel()

	var conditions []metav1.Condition
	setSyncedCondition(&conditions, 1, false, leakReason, leakErr().Error())

	cond := apimeta.FindStatusCondition(conditions, conditionSynced)
	if cond == nil {
		t.Fatal("Synced condition missing")
	}

	assertNoSecret(t, "Synced condition", cond.Message)

	if cond.Reason != leakReason {
		t.Fatalf("reason = %q, want %q", cond.Reason, leakReason)
	}
}

func TestSetSinkReachableCondition_redactsMessage(t *testing.T) {
	t.Parallel()

	var conditions []metav1.Condition
	setSinkReachableFromExport(&conditions, 1, leakErr())

	cond := apimeta.FindStatusCondition(conditions, conditionSinkReachable)
	if cond == nil {
		t.Fatal("SinkReachable condition missing")
	}

	assertNoSecret(t, "SinkReachable condition", cond.Message)

	if cond.Reason != kollectdevv1alpha1.ReasonExportTerminal {
		t.Fatalf("terminal classification lost: reason = %q", cond.Reason)
	}
}

func TestSetSinkExportSynced_redactsMessage(t *testing.T) {
	t.Parallel()

	status := &kollectdevv1alpha1.InventorySinkExportStatus{Name: "snap"}
	setSinkExportSynced(status, 1, false, leakReason, leakErr().Error())

	cond := apimeta.FindStatusCondition(status.Conditions, conditionSinkSynced)
	if cond == nil {
		t.Fatal("per-sink Synced condition missing")
	}

	assertNoSecret(t, "per-sink Synced condition", cond.Message)
}

func TestSetConnectionFailed_redactsMessage(t *testing.T) {
	t.Parallel()

	scheme := controllerScheme(t)
	obj := databaseSinkWithTestAnnotation("redact-me")
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		WithStatusSubresource(obj).
		Build()

	conn := familySinkConnection{client: cl}

	if err := conn.setConnectionFailed(context.Background(), obj, obj.FamilySinkSpec(), &obj.Status.Conditions, leakReason, leakErr().Error()); err != nil {
		t.Fatalf("setConnectionFailed: %v", err)
	}

	for _, condType := range []string{kollectdevv1alpha1.ConditionConnectionVerified, conditionDegraded} {
		cond := apimeta.FindStatusCondition(obj.Status.Conditions, condType)
		if cond == nil {
			t.Fatalf("%s condition missing", condType)
		}

		assertNoSecret(t, condType+" condition", cond.Message)
	}
}

func TestSetConnectionVerified_redactsMessage(t *testing.T) {
	t.Parallel()

	scheme := controllerScheme(t)
	obj := databaseSinkWithTestAnnotation("redact-ok")
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		WithStatusSubresource(obj).
		Build()

	conn := familySinkConnection{client: cl}
	common := &kollectdevv1alpha1.SinkCommonFields{}

	err := conn.setConnectionVerified(
		context.Background(), obj, obj.Spec.ToKollectSinkSpec(), common, &obj.Status.Conditions,
		"connected to "+leakURL,
	)
	if err != nil {
		t.Fatalf("setConnectionVerified: %v", err)
	}

	cond := apimeta.FindStatusCondition(obj.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
	if cond == nil {
		t.Fatal("ConnectionVerified condition missing")
	}

	assertNoSecret(t, "ConnectionVerified condition", cond.Message)
}

func TestSetProbeFailed_redactsMessage(t *testing.T) {
	t.Parallel()

	scheme := controllerScheme(t)
	test := &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{Name: "probe", Namespace: "ns", Generation: 1},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(test).
		WithStatusSubresource(test).
		Build()

	r := &KollectConnectionTestReconciler{Client: cl, Scheme: scheme}
	if _, err := r.setProbeFailed(context.Background(), test, leakReason, leakErr().Error()); err != nil {
		t.Fatalf("setProbeFailed: %v", err)
	}

	var got kollectdevv1alpha1.KollectConnectionTest
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ns", Name: "probe"}, &got); err != nil {
		t.Fatalf("Get probe: %v", err)
	}

	for _, condType := range []string{kollectdevv1alpha1.ConditionConnectionVerified, conditionReady} {
		cond := apimeta.FindStatusCondition(got.Status.Conditions, condType)
		if cond == nil {
			t.Fatalf("%s condition missing", condType)
		}

		assertNoSecret(t, condType+" condition", cond.Message)
	}
}

func TestSetInventoryDegraded_redactsMessage(t *testing.T) {
	t.Parallel()

	scheme := controllerScheme(t)
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "ns", Generation: 1},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(inv).
		WithStatusSubresource(inv).
		Build()

	r := &KollectInventoryReconciler{Client: cl, Scheme: scheme}
	if _, err := r.setInventoryDegraded(context.Background(), inv, 0, leakReason, leakErr().Error()); err != nil {
		t.Fatalf("setInventoryDegraded: %v", err)
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ns", Name: "inv"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	for _, condType := range []string{conditionDegraded, conditionSynced} {
		cond := apimeta.FindStatusCondition(got.Status.Conditions, condType)
		if cond == nil {
			t.Fatalf("%s condition missing", condType)
		}

		assertNoSecret(t, condType+" condition", cond.Message)
	}
}

func TestClusterSetDegraded_redactsMessage(t *testing.T) {
	t.Parallel()

	scheme := controllerScheme(t)
	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "cinv", Generation: 1},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(inv).
		WithStatusSubresource(inv).
		Build()

	r := &KollectClusterInventoryReconciler{Client: cl, Scheme: scheme}
	if _, err := r.setDegraded(context.Background(), inv, leakReason, leakErr().Error()); err != nil {
		t.Fatalf("setDegraded: %v", err)
	}

	var got kollectdevv1alpha1.KollectClusterInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "cinv"}, &got); err != nil {
		t.Fatalf("Get cluster inventory: %v", err)
	}

	for _, condType := range []string{conditionDegraded, conditionSynced} {
		cond := apimeta.FindStatusCondition(got.Status.Conditions, condType)
		if cond == nil {
			t.Fatalf("%s condition missing", condType)
		}

		assertNoSecret(t, condType+" condition", cond.Message)
	}
}

// updateStatus's all-sinks-failed branch writes Degraded and Ready directly from
// outcome.ExportErr instead of going through setSyncedCondition, so it needs its
// own redaction lock.
func TestUpdateStatus_failedWithNoExport_redactsMessage(t *testing.T) {
	t.Parallel()

	scheme := controllerScheme(t)
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "ns", Generation: 1},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("git"),
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(inv).
		WithStatusSubresource(inv).
		Build()

	r := &KollectInventoryReconciler{Client: cl, Scheme: scheme}
	if _, err := r.updateStatus(context.Background(), inv, 0, perSinkExportOutcome{
		FailedCount:    1,
		DebouncedCount: 1,
		ExportErr:      leakErr(),
	}); err != nil {
		t.Fatalf("updateStatus: %v", err)
	}

	var got kollectdevv1alpha1.KollectInventory
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ns", Name: "inv"}, &got); err != nil {
		t.Fatalf("Get inventory: %v", err)
	}

	for _, condType := range []string{conditionDegraded, conditionReady} {
		cond := apimeta.FindStatusCondition(got.Status.Conditions, condType)
		if cond == nil {
			t.Fatalf("%s condition missing", condType)
		}

		if cond.Reason != reasonExportFailed {
			t.Fatalf("%s reason = %q, want %q", condType, cond.Reason, reasonExportFailed)
		}

		assertNoSecret(t, condType+" condition", cond.Message)
	}
}

// The KollectTarget and KollectClusterTarget Degraded writers carry scope-deny
// (KollectScope lookup failure) and informer-registration error text, so their
// shared condition writers are choke-points too.
func TestTargetSetDegraded_redactsMessage(t *testing.T) {
	t.Parallel()

	scheme := controllerScheme(t)
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "t", Namespace: "ns", Generation: 1},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(target).
		WithStatusSubresource(target).
		Build()

	r := &KollectTargetReconciler{Client: cl, Scheme: scheme}
	if err := r.setDegraded(context.Background(), target, scopeReasonLookupFailed, leakErr().Error()); err != nil {
		t.Fatalf("setDegraded: %v", err)
	}

	key := types.NamespacedName{Namespace: "ns", Name: "t"}

	var got kollectdevv1alpha1.KollectTarget
	if err := cl.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("Get target: %v", err)
	}

	for _, condType := range []string{conditionDegraded, conditionSinkReachable} {
		cond := apimeta.FindStatusCondition(got.Status.Conditions, condType)
		if cond == nil {
			t.Fatalf("%s condition missing", condType)
		}

		assertNoSecret(t, condType+" condition", cond.Message)
	}

	// A repeated deny with the same raw text must be recognised as unchanged:
	// the skip check compares against the redacted, persisted message, so no
	// second status write may happen.
	before := got.ResourceVersion
	if err := r.setDegraded(context.Background(), &got, scopeReasonLookupFailed, leakErr().Error()); err != nil {
		t.Fatalf("setDegraded (repeat): %v", err)
	}

	var again kollectdevv1alpha1.KollectTarget
	if err := cl.Get(context.Background(), key, &again); err != nil {
		t.Fatalf("Get target (repeat): %v", err)
	}

	if again.ResourceVersion != before {
		t.Fatalf("repeat deny rewrote status: resourceVersion %s -> %s", before, again.ResourceVersion)
	}
}

func TestClusterTargetSetDegraded_redactsMessage(t *testing.T) {
	t.Parallel()

	scheme := controllerScheme(t)
	ct := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct", Generation: 1},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ct).
		WithStatusSubresource(ct).
		Build()

	r := &KollectClusterTargetReconciler{Client: cl, Scheme: scheme}
	if err := r.setDegraded(context.Background(), ct, "InformerRegistrationFailed", leakErr().Error()); err != nil {
		t.Fatalf("setDegraded: %v", err)
	}

	var got kollectdevv1alpha1.KollectClusterTarget
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "ct"}, &got); err != nil {
		t.Fatalf("Get cluster target: %v", err)
	}

	cond := apimeta.FindStatusCondition(got.Status.Conditions, conditionDegraded)
	if cond == nil {
		t.Fatal("Degraded condition missing")
	}

	assertNoSecret(t, "Degraded condition", cond.Message)
}

// A recovered panic value is free-form text (often a wrapped error), so the
// ReconcilePanic Event must go through the same choke-point as every other
// Event this package emits.
func TestGuardReconcile_redactsPanicEvent(t *testing.T) {
	t.Parallel()

	rec := record.NewFakeRecorder(4)
	inv := &kollectdevv1alpha1.KollectInventory{ObjectMeta: metav1.ObjectMeta{Name: "i", Namespace: "ns"}}

	// Requeue-on-panic is locked by reconcile_guard_test.go; this test is
	// about the Event text only.
	if _, err := guardReconcile(context.Background(), rec, inv, func() (ctrl.Result, error) {
		panic(leakErr())
	}); err != nil {
		t.Fatalf("guardReconcile returned %v, want nil after recovery", err)
	}

	select {
	case event := <-rec.Events:
		if !strings.Contains(event, "ReconcilePanic") {
			t.Fatalf("event lost its reason: %q", event)
		}

		assertNoSecret(t, "ReconcilePanic Event message", event)
	default:
		t.Fatal("no event recorded")
	}
}
