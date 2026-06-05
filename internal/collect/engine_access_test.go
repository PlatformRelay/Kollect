// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
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

	e := &Engine{
		store:     store,
		extractor: ext,
		access:    NewAccessChecker(client),
		forbidden: make(map[string]struct{}),
		accessErr: make(map[string]struct{}),
		nsMeta:    map[string]namespaceMeta{"team-a": {}},
		targets: map[string]targetState{
			targetKey("team-a", "deploys"): {
				target:              target,
				profile:             profile,
				effectiveNamespaces: map[string]struct{}{"team-a": {}},
				compiledRules:       rules,
			},
		},
	}

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "web", "namespace": "team-a", "uid": "uid-1",
		},
	}}

	e.dispatch(context.Background(), gvr, obj, false)

	if store.CountForTarget("team-a", "deploys") != 0 {
		t.Fatalf("item count = %d, want 0 when SAR API fails", store.CountForTarget("team-a", "deploys"))
	}
	if !e.HasAccessCheckFailure("team-a", "deploys") {
		t.Fatal("expected access check failure flag after SAR API error")
	}
}
