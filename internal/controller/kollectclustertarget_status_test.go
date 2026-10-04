// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

// statusCountingClient counts Status().Update calls, so a no-op reconcile can be proven to
// issue zero API writes rather than inferred from ResourceVersion churn.
type statusCountingClient struct {
	client.Client
	updates *int
}

func (c *statusCountingClient) Status() client.SubResourceWriter {
	return &statusCountingWriter{SubResourceWriter: c.Client.Status(), updates: c.updates}
}

type statusCountingWriter struct {
	client.SubResourceWriter
	updates *int
}

func (w *statusCountingWriter) Update(
	ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption,
) error {
	*w.updates++

	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

func clusterTargetStatusClient(t *testing.T, ct *kollectdevv1alpha1.KollectClusterTarget) (client.Client, *int) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	base := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ct).
		WithStatusSubresource(ct).
		Build()

	updates := 0

	return &statusCountingClient{Client: base, updates: &updates}, &updates
}

func readyConditionMessage(profile, ns string, matched, count int) string {
	return clusterTargetReadyMessage(
		kollectdevv1alpha1.NamespacedObjectReference{Name: profile, Namespace: ns}, matched, count,
	)
}

func TestSetClusterTargetCondition_skipsNoopWrite(t *testing.T) {
	t.Parallel()

	ct := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct", Generation: 1},
		Status: kollectdevv1alpha1.KollectClusterTargetStatus{
			ObservedGeneration: 1,
			Conditions: []metav1.Condition{{
				Type: conditionReady, Status: metav1.ConditionTrue,
				Reason: reasonCollecting, Message: "same", ObservedGeneration: 1,
			}},
		},
	}

	cl, updates := clusterTargetStatusClient(t, ct)

	live := &kollectdevv1alpha1.KollectClusterTarget{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(ct), live); err != nil {
		t.Fatalf("get: %v", err)
	}

	written, err := setClusterTargetCondition(
		context.Background(), cl, live, 1, &live.Status.Conditions,
		conditionReady, reasonCollecting, "same",
	)
	if err != nil {
		t.Fatalf("setClusterTargetCondition: %v", err)
	}
	if written {
		t.Fatal("setClusterTargetCondition reported a write for an unchanged condition")
	}
	if *updates != 0 {
		t.Fatalf("status writes = %d, want 0", *updates)
	}
}

func TestSetClusterTargetCondition_writesChangedCondition(t *testing.T) {
	t.Parallel()

	ct := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct", Generation: 1},
		Status: kollectdevv1alpha1.KollectClusterTargetStatus{
			ObservedGeneration: 1,
			Conditions: []metav1.Condition{{
				Type: conditionReady, Status: metav1.ConditionTrue,
				Reason: reasonCollecting, Message: "old", ObservedGeneration: 1,
			}},
		},
	}

	cl, updates := clusterTargetStatusClient(t, ct)

	live := &kollectdevv1alpha1.KollectClusterTarget{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(ct), live); err != nil {
		t.Fatalf("get: %v", err)
	}

	written, err := setClusterTargetCondition(
		context.Background(), cl, live, 1, &live.Status.Conditions,
		conditionReady, reasonCollecting, "new",
	)
	if err != nil {
		t.Fatalf("setClusterTargetCondition: %v", err)
	}
	if !written || *updates != 1 {
		t.Fatalf("written=%v updates=%d, want true/1", written, *updates)
	}
}

// TestClusterTargetSetReady_persistsFilterStatusWhenConditionSkipped is the filter-status regression
// guard: the reconcile mutates matched/effective/activeResourceRules and relies on the
// condition write to persist them. When the Ready condition is byte-identical (a namespace
// swap with stable counts) the shared skip must not drop the filter status.
func TestClusterTargetSetReady_persistsFilterStatusWhenConditionSkipped(t *testing.T) {
	t.Parallel()

	const (
		profile = "deployments"
		ns      = "kollect-system"
	)

	ct := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct", Generation: 1},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{Name: profile, Namespace: ns},
		},
		Status: kollectdevv1alpha1.KollectClusterTargetStatus{
			ObservedGeneration: 1,
			CollectionFilterStatus: kollectdevv1alpha1.CollectionFilterStatus{
				MatchedNamespaces:   []string{"a"},
				EffectiveNamespaces: []string{"a"},
				ActiveResourceRules: 1,
			},
			Conditions: []metav1.Condition{{
				Type: conditionReady, Status: metav1.ConditionTrue,
				Reason:             reasonCollecting,
				Message:            readyConditionMessage(profile, ns, 1, 0),
				ObservedGeneration: 1,
			}},
		},
	}

	cl, updates := clusterTargetStatusClient(t, ct)

	live := &kollectdevv1alpha1.KollectClusterTarget{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(ct), live); err != nil {
		t.Fatalf("get: %v", err)
	}

	// Simulate the reconcile: the matched set changed (a→b) but its length and the collected
	// count did not, so the Ready message is byte-identical and the shared writer skips. The
	// predicate is computed before the mutation, exactly as Reconcile does.
	filterChanged := clusterTargetFilterChanged(live, []string{"b"}, []string{"b"}, 1)
	if !filterChanged {
		t.Fatal("fixture did not produce a filter change")
	}
	updateClusterTargetFilterStatus(live, []string{"b"}, []string{"b"}, 1)

	r := &KollectClusterTargetReconciler{Client: cl}
	if err := r.setReady(context.Background(), live, []string{"b"}, filterChanged); err != nil {
		t.Fatalf("setReady: %v", err)
	}

	if *updates != 1 {
		t.Fatalf("status writes = %d, want 1 (filter-status escape hatch)", *updates)
	}

	var stored kollectdevv1alpha1.KollectClusterTarget
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(ct), &stored); err != nil {
		t.Fatalf("get stored: %v", err)
	}
	if len(stored.Status.MatchedNamespaces) != 1 || stored.Status.MatchedNamespaces[0] != "b" {
		t.Fatalf("persisted matchedNamespaces = %v, want [b]", stored.Status.MatchedNamespaces)
	}
}

// TestClusterTargetFilterChanged covers the predicate that decides whether the escape hatch
// fires: a bug making it constant would either drop filter status (false) or defeat the churn
// reduction (true), so it is asserted directly.
func TestClusterTargetFilterChanged(t *testing.T) {
	t.Parallel()

	ct := &kollectdevv1alpha1.KollectClusterTarget{
		Status: kollectdevv1alpha1.KollectClusterTargetStatus{
			CollectionFilterStatus: kollectdevv1alpha1.CollectionFilterStatus{
				MatchedNamespaces:   []string{"a"},
				EffectiveNamespaces: []string{"a"},
				ActiveResourceRules: 1,
			},
		},
	}

	if clusterTargetFilterChanged(ct, []string{"a"}, []string{"a"}, 1) {
		t.Fatal("unchanged filter status reported as changed")
	}
	if !clusterTargetFilterChanged(ct, []string{"b"}, []string{"a"}, 1) {
		t.Fatal("changed matched namespaces not detected")
	}
	if !clusterTargetFilterChanged(ct, []string{"a"}, []string{"b"}, 1) {
		t.Fatal("changed effective namespaces not detected")
	}
	if !clusterTargetFilterChanged(ct, []string{"a"}, []string{"a"}, 2) {
		t.Fatal("changed active resource rules not detected")
	}
}

// TestClusterTargetSetReady_skipsWriteWhenNothingChanged pins the churn reduction: an
// unchanged condition with an unchanged filter set issues no status write at all.
func TestClusterTargetSetReady_skipsWriteWhenNothingChanged(t *testing.T) {
	t.Parallel()

	const (
		profile = "deployments"
		ns      = "kollect-system"
	)

	ct := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct", Generation: 1},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{Name: profile, Namespace: ns},
		},
		Status: kollectdevv1alpha1.KollectClusterTargetStatus{
			ObservedGeneration: 1,
			CollectionFilterStatus: kollectdevv1alpha1.CollectionFilterStatus{
				MatchedNamespaces:   []string{"a"},
				EffectiveNamespaces: []string{"a"},
				ActiveResourceRules: 1,
			},
			Conditions: []metav1.Condition{{
				Type: conditionReady, Status: metav1.ConditionTrue,
				Reason:             reasonCollecting,
				Message:            readyConditionMessage(profile, ns, 1, 0),
				ObservedGeneration: 1,
			}},
		},
	}

	cl, updates := clusterTargetStatusClient(t, ct)

	live := &kollectdevv1alpha1.KollectClusterTarget{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(ct), live); err != nil {
		t.Fatalf("get: %v", err)
	}

	r := &KollectClusterTargetReconciler{Client: cl}
	if err := r.setReady(context.Background(), live, []string{"a"}, false); err != nil {
		t.Fatalf("setReady: %v", err)
	}

	if *updates != 0 {
		t.Fatalf("status writes = %d, want 0 for a no-op reconcile", *updates)
	}
}

// reconcileFilterSwapFixture builds a cluster target whose stored filter status names namespace
// "a" while the only matching namespace is now "b": the counts are stable, so a seeded condition
// whose message depends only on counts stays byte-identical and the shared writer skips it.
func reconcileFilterSwapFixture(
	t *testing.T,
	seeded metav1.Condition,
) (*KollectClusterTargetReconciler, client.Client, *int, *kollectdevv1alpha1.KollectClusterTarget) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1): %v", err)
	}

	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "deployments", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}
	nsB := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "b", Labels: map[string]string{"team": "platform"}},
	}
	ct := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "ct",
			Generation: 1,
			Finalizers: []string{clusterTargetCleanupFinalizer},
		},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{Name: "deployments", Namespace: "kollect-system"},
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "platform"},
			},
		},
		Status: kollectdevv1alpha1.KollectClusterTargetStatus{
			ObservedGeneration: 1,
			CollectionFilterStatus: kollectdevv1alpha1.CollectionFilterStatus{
				MatchedNamespaces:   []string{"a"},
				EffectiveNamespaces: []string{"a"},
			},
			Conditions: []metav1.Condition{seeded},
		},
	}

	base := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(profile, nsB, ct).
		WithStatusSubresource(ct).
		Build()

	updates := 0
	cl := &statusCountingClient{Client: base, updates: &updates}

	return &KollectClusterTargetReconciler{Client: cl, Scheme: scheme}, cl, &updates, ct
}

func assertPersistedFilterStatus(t *testing.T, cl client.Client, ct *kollectdevv1alpha1.KollectClusterTarget, updates int) {
	t.Helper()

	if updates != 1 {
		t.Fatalf("status writes = %d, want 1 (filter-status escape hatch on a skipped condition)", updates)
	}

	var stored kollectdevv1alpha1.KollectClusterTarget
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(ct), &stored); err != nil {
		t.Fatalf("get stored: %v", err)
	}
	if !slices.Equal(stored.Status.MatchedNamespaces, []string{"b"}) ||
		!slices.Equal(stored.Status.EffectiveNamespaces, []string{"b"}) {
		t.Fatalf("persisted matched/effective = %v/%v, want [b]/[b]",
			stored.Status.MatchedNamespaces, stored.Status.EffectiveNamespaces)
	}
}

// TestClusterTargetReconcile_persistsFilterStatusWhenReadySkipped drives the healthy path through
// Reconcile: the matched namespace swaps a→b with stable counts, so the Ready condition is
// byte-identical and skipped, and Reconcile itself must have detected the filter change for the
// status to persist.
func TestClusterTargetReconcile_persistsFilterStatusWhenReadySkipped(t *testing.T) {
	t.Parallel()

	r, cl, updates, ct := reconcileFilterSwapFixture(t, metav1.Condition{
		Type: conditionReady, Status: metav1.ConditionTrue,
		Reason:             reasonCollecting,
		Message:            readyConditionMessage("deployments", "kollect-system", 1, 0),
		ObservedGeneration: 1,
	})

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: ct.Name},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	assertPersistedFilterStatus(t, cl, ct, *updates)
}

// TestClusterTargetReconcile_persistsFilterStatusWhenRegistrationDegradedSkipped is the same guard
// on the InformerRegistrationFailed branch: an identical Degraded condition is skipped, and the
// filter change computed by Reconcile must still reach the API server.
func TestClusterTargetReconcile_persistsFilterStatusWhenRegistrationDegradedSkipped(t *testing.T) {
	t.Parallel()

	kube := kubefake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset is sufficient here
	kube.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("namespace list refused")
	})

	engine, err := collect.NewEngine(nil, kube, collect.NewStore(), collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	r, cl, updates, ct := reconcileFilterSwapFixture(t, metav1.Condition{
		Type: conditionDegraded, Status: metav1.ConditionTrue,
		Reason: "InformerRegistrationFailed",
		Message: "refresh namespace cache before cluster target registration: " +
			"list namespaces: namespace list refused",
		ObservedGeneration: 1,
	})
	r.Engine = engine

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: ct.Name},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	assertPersistedFilterStatus(t, cl, ct, *updates)

	var stored kollectdevv1alpha1.KollectClusterTarget
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(ct), &stored); err != nil {
		t.Fatalf("get stored: %v", err)
	}
	if cond := apimeta.FindStatusCondition(stored.Status.Conditions, conditionDegraded); cond == nil ||
		cond.Reason != "InformerRegistrationFailed" {
		t.Fatalf("Degraded = %+v, want reason InformerRegistrationFailed", cond)
	}
}
