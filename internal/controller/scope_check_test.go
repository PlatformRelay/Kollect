// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestScopeCheckEnforceTarget(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	scopeCR := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-scope", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
		},
	}
	profile := &kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Pod"},
		},
	}
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "deploys", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "demo"},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(scopeCR).Build()
	check := scopeCheck{client: c}

	ok, reason, _ := check.enforceTarget(context.Background(), target, profile)
	if ok || reason != scopeReasonGVKDenied {
		t.Fatalf("enforceTarget = ok=%v reason=%q", ok, reason)
	}
}

func TestScopeCheckSinkReachable(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	verified := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "team-a"},
		Status: kollectdevv1alpha1.KollectSinkStatus{
			Conditions: []metav1.Condition{{
				Type:   kollectdevv1alpha1.ConditionConnectionVerified,
				Status: metav1.ConditionTrue,
			}},
		},
	}
	failed := verified.DeepCopy()
	failed.Name = "bad"
	apimeta.SetStatusCondition(&failed.Status.Conditions, metav1.Condition{
		Type:   kollectdevv1alpha1.ConditionConnectionVerified,
		Status: metav1.ConditionFalse,
	})

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(verified, failed).Build()
	check := scopeCheck{client: c}

	ok, reason, _ := check.sinkReachable(context.Background(), "team-a", "ok")
	if !ok || reason != "ConnectionVerified" {
		t.Fatalf("verified sink: ok=%v reason=%q", ok, reason)
	}

	ok, reason, _ = check.sinkReachable(context.Background(), "team-a", "bad")
	if ok || reason != reasonSinkUnreachable {
		t.Fatalf("failed sink: ok=%v reason=%q", ok, reason)
	}
}

func TestScopeCheckEnforceInventory(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	scopeCR := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-scope", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectScopeSpec{SinkRefs: []string{"allowed-git"}},
	}
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "rollup", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectInventorySpec{SinkRefs: kollectdevv1alpha1.NewSinkRefList("other-git")},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(scopeCR).Build()
	check := scopeCheck{client: c}

	ok, reason, _ := check.enforceInventory(context.Background(), inv)
	if ok || reason != scopeReasonSinkDenied {
		t.Fatalf("enforceInventory = ok=%v reason=%q", ok, reason)
	}
}
