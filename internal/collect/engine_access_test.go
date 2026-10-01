// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"fmt"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestEngineDispatchSARAPIErrorMarksAccessFailure(t *testing.T) {
	t.Parallel()

	store := NewStore()
	ext, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}
	rules, err := CompileResourceRules(nil, ext.celEnv)
	if err != nil {
		t.Fatal(err)
	}

	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor(
		"create", "selfsubjectaccessreviews",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("apiserver unavailable")
		},
	)

	profile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}
	target := kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "deploys"},
	}

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	key := targetKey("team-a", "deploys")

	e := &Engine{
		store:        store,
		extractor:    ext,
		access:       NewAccessChecker(client),
		forbidden:    make(map[string]struct{}),
		accessErr:    make(map[string]struct{}),
		nsMeta:       map[string]namespaceMeta{"team-a": {}},
		targets:      make(map[string]targetState),
		targetsByGVR: make(map[schema.GroupVersionResource][]string),
	}
	e.targets[key] = targetState{
		target:              target,
		profile:             profile,
		effectiveNamespaces: map[string]struct{}{"team-a": {}},
		compiledRules:       rules,
	}
	e.targetsByGVR[gvr] = []string{key}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "web", "namespace": "team-a", "uid": "uid-1",
		},
	}}

	e.processDispatch(context.Background(), gvr, obj, false)

	if store.CountForTarget("team-a", "deploys") != 0 {
		t.Fatalf("item count = %d, want 0 when SAR API fails", store.CountForTarget("team-a", "deploys"))
	}
	if !e.HasAccessCheckFailure("team-a", "deploys") {
		t.Fatal("expected access check failure flag after SAR API error")
	}
}

func TestEngineDispatchDeniedListDropsStoredRow(t *testing.T) {
	t.Parallel()

	store := NewStore()
	store.Upsert(Item{
		TargetNamespace: "team-a",
		TargetName:      "deploys",
		Namespace:       "team-a",
		Name:            "web",
		UID:             "uid-1",
	})
	ext, err := NewExtractor()
	if err != nil {
		t.Fatal(err)
	}
	rules, err := CompileResourceRules(nil, ext.celEnv)
	if err != nil {
		t.Fatal(err)
	}

	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor(
		"create", "selfsubjectaccessreviews",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: false}

			return true, review, nil
		},
	)

	profile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}
	target := kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "deploys"},
	}

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	key := targetKey("team-a", "deploys")

	e := &Engine{
		store:        store,
		extractor:    ext,
		access:       NewAccessChecker(client),
		forbidden:    make(map[string]struct{}),
		accessErr:    make(map[string]struct{}),
		nsMeta:       map[string]namespaceMeta{"team-a": {}},
		targets:      make(map[string]targetState),
		targetsByGVR: make(map[schema.GroupVersionResource][]string),
	}
	e.targets[key] = targetState{
		target:              target,
		profile:             profile,
		effectiveNamespaces: map[string]struct{}{"team-a": {}},
		compiledRules:       rules,
	}
	e.targetsByGVR[gvr] = []string{key}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "web", "namespace": "team-a", "uid": "uid-1",
		},
	}}

	e.processDispatch(context.Background(), gvr, obj, false)

	if store.CountForTarget("team-a", "deploys") != 0 {
		t.Fatalf("item count = %d, want 0 after list access is denied", store.CountForTarget("team-a", "deploys"))
	}
	if !e.HasForbiddenScope("team-a", "deploys") {
		t.Fatal("expected forbidden scope after a denied list check")
	}
}
