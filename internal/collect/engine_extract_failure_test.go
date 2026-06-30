// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func allowAllAccessClient() *fake.Clientset {
	client := fake.NewSimpleClientset() //nolint:staticcheck
	client.PrependReactor(
		"create", "selfsubjectaccessreviews",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			review.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}

			return true, review, nil
		})

	return client
}

func TestEngineDispatchExtractionErrorMarksExtractFailure(t *testing.T) {
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

	profile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "bad", Path: "cel:1 +"}, // malformed CEL: fails to compile on every resource
			},
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
		access:       NewAccessChecker(allowAllAccessClient()),
		forbidden:    make(map[string]struct{}),
		accessErr:    make(map[string]struct{}),
		extractErr:   make(map[string]*extractFailureState),
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
		t.Fatalf("item count = %d, want 0 when extraction fails", store.CountForTarget("team-a", "deploys"))
	}

	count, lastErr := e.ExtractFailures("team-a", "deploys")
	if count != 1 {
		t.Fatalf("extract failure count = %d, want 1", count)
	}
	if lastErr == "" {
		t.Fatal("expected non-empty last extraction error message")
	}

	// A second, distinct resource failing extraction should bump the count to 2.
	obj2 := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "api", "namespace": "team-a", "uid": "uid-2",
		},
	}}
	e.processDispatch(context.Background(), gvr, obj2, false)

	count, _ = e.ExtractFailures("team-a", "deploys")
	if count != 2 {
		t.Fatalf("extract failure count after second failing resource = %d, want 2", count)
	}
}

func TestEngineDispatchExtractionSuccessClearsExtractFailure(t *testing.T) {
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
		store:     store,
		extractor: ext,
		access:    NewAccessChecker(allowAllAccessClient()),
		forbidden: make(map[string]struct{}),
		accessErr: make(map[string]struct{}),
		extractErr: map[string]*extractFailureState{
			key: {resources: map[string]struct{}{"uid-1": {}}, lastErr: "attribute \"bad\": boom"},
		},
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

	count, lastErr := e.ExtractFailures("team-a", "deploys")
	if count != 0 {
		t.Fatalf("extract failure count = %d, want 0 after successful extraction", count)
	}
	if lastErr != "" {
		t.Fatalf("last extraction error = %q, want empty after recovery", lastErr)
	}
}
