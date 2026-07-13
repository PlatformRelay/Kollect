// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// COV-90-01: resolveProfileOrDegrade forbidden sub-branch (cluster_scope_enforce.go:59-61).
// The envtest cluster-admin client never returns a Forbidden error on a profile Get, so
// this branch is only reachable with an interceptor fake that injects apierrors.IsForbidden.
// A forbidden profile lookup must degrade with ProfileForbidden AND emit a Warning event —
// distinct from the plain NotFound path which records no event.
func TestResolveProfileOrDegrade_Forbidden_DegradesWithEventAndProfileForbidden(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	ct := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "forbidden-ct", Generation: 1},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{
				Name:      "some-profile",
				Namespace: "tenant-a",
			},
		},
	}

	gr := schema.GroupResource{Group: "kollect.dev", Resource: "kollectprofiles"}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ct).
		WithStatusSubresource(ct).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				fctx context.Context, c client.WithWatch, key client.ObjectKey,
				obj client.Object, opts ...client.GetOption,
			) error {
				if _, ok := obj.(*kollectdevv1alpha1.KollectProfile); ok {
					return apierrors.NewForbidden(gr, key.Name,
						apierrors.NewForbidden(gr, key.Name, nil))
				}
				return c.Get(fctx, key, obj, opts...)
			},
		}).
		Build()

	recorder := record.NewFakeRecorder(5)
	r := &KollectClusterTargetReconciler{
		Client:   cl,
		Recorder: recorder,
	}

	profile, degraded, err := r.resolveProfileOrDegrade(context.Background(), ct)
	if err != nil {
		t.Fatalf("resolveProfileOrDegrade returned unexpected error: %v", err)
	}
	if profile != nil {
		t.Fatalf("resolveProfileOrDegrade profile = %#v, want nil on forbidden", profile)
	}
	if !degraded {
		t.Fatal("resolveProfileOrDegrade degraded = false, want true on a forbidden profile lookup")
	}

	updated := &kollectdevv1alpha1.KollectClusterTarget{}
	if err := cl.Get(context.Background(),
		client.ObjectKey{Name: "forbidden-ct"}, updated); err != nil {
		t.Fatalf("Get cluster target back: %v", err)
	}

	deg := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
	if deg == nil || deg.Status != metav1.ConditionTrue || deg.Reason != reasonProfileForbidden {
		t.Fatalf("Degraded condition = %#v, want True/%s", deg, reasonProfileForbidden)
	}

	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, reasonProfileForbidden) {
			t.Fatalf("event = %q, want it to mention %q", ev, reasonProfileForbidden)
		}
	default:
		t.Fatal("expected a Warning event for the forbidden profile lookup")
	}
}
