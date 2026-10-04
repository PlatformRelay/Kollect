// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// statusCountingClient counts Status().Update calls, so a no-op reconcile can be proven to
// issue zero API writes (D5) rather than inferred from ResourceVersion churn.
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
	return fmt.Sprintf(
		"profileRef %q in namespace %q resolved; %d namespace(s) matched; collecting %d resource(s)",
		profile, ns, matched, count,
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

// TestClusterTargetSetReady_persistsFilterStatusWhenConditionSkipped is the D5 regression
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

	// Simulate the reconcile: the filter set changed but the Ready message (counts) did not.
	updateClusterTargetFilterStatus(live, []string{"a", "b"}, []string{"a", "b"}, 2)
	filterChanged := true

	r := &KollectClusterTargetReconciler{Client: cl}
	if err := r.setReady(context.Background(), live, []string{"a", "b"}, filterChanged); err != nil {
		t.Fatalf("setReady: %v", err)
	}

	if *updates != 1 {
		t.Fatalf("status writes = %d, want 1 (filter-status escape hatch)", *updates)
	}

	var stored kollectdevv1alpha1.KollectClusterTarget
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(ct), &stored); err != nil {
		t.Fatalf("get stored: %v", err)
	}
	if len(stored.Status.MatchedNamespaces) != 2 || stored.Status.ActiveResourceRules != 2 {
		t.Fatalf("persisted filter status = %v / %d, want [a b] / 2",
			stored.Status.MatchedNamespaces, stored.Status.ActiveResourceRules)
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
