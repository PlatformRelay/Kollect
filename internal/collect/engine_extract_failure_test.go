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

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
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

// TestEngineDispatchExtractionFailureInvalidatesPriorRow is the REL-01 red proof:
// success → later required-extraction failure must remove the last-good row so
// export/fingerprint no longer present the stale value as current.
func TestEngineDispatchExtractionFailureInvalidatesPriorRow(t *testing.T) {
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

	goodProfile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "replicas", Path: "cel:object.spec.replicas"},
			},
		},
	}
	badProfile := kollectdevv1alpha1.KollectProfile{
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "bad", Path: "cel:1 +"}, // malformed CEL: fails on every resource
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
		profile:             goodProfile,
		effectiveNamespaces: map[string]struct{}{"team-a": {}},
		compiledRules:       rules,
	}
	e.targetsByGVR[gvr] = []string{key}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "web", "namespace": "team-a", "uid": "uid-1",
		},
		"spec": map[string]any{"replicas": int64(3)},
	}}

	e.processDispatch(context.Background(), gvr, obj, false)

	if store.CountForTarget("team-a", "deploys") != 1 {
		t.Fatalf("item count after success = %d, want 1", store.CountForTarget("team-a", "deploys"))
	}
	versionAfterSuccess := store.NamespaceVersion("team-a")
	fpAfterSuccess, err := ItemsFingerprint(store.SnapshotTarget("team-a", "deploys"))
	if err != nil {
		t.Fatalf("ItemsFingerprint after success: %v", err)
	}
	if fpAfterSuccess == "" {
		t.Fatal("expected non-empty fingerprint after successful store")
	}
	envelopeAfterSuccess, err := store.MarshalTargetJSON("team-a", "deploys")
	if err != nil {
		t.Fatalf("MarshalTargetJSON after success: %v", err)
	}
	if len(envelopeAfterSuccess) == 0 {
		t.Fatal("expected non-empty export envelope after success")
	}

	// Later required extraction fails for the same UID (profile now has broken CEL).
	st := e.targets[key]
	st.profile = badProfile
	e.targets[key] = st

	e.processDispatch(context.Background(), gvr, obj, false)

	if store.CountForTarget("team-a", "deploys") != 0 {
		t.Fatalf("item count after extract failure = %d, want 0 (REL-01: invalidate last-good row)",
			store.CountForTarget("team-a", "deploys"))
	}
	versionAfterFailure := store.NamespaceVersion("team-a")
	if versionAfterFailure <= versionAfterSuccess {
		t.Fatalf("NamespaceVersion after failure = %d, want > %d (export consumers must observe removal)",
			versionAfterFailure, versionAfterSuccess)
	}
	fpAfterFailure, err := ItemsFingerprint(store.SnapshotTarget("team-a", "deploys"))
	if err != nil {
		t.Fatalf("ItemsFingerprint after failure: %v", err)
	}
	if fpAfterFailure == fpAfterSuccess {
		t.Fatalf("fingerprint unchanged after failure removal: %q", fpAfterFailure)
	}
	envelopeAfterFailure, err := store.MarshalTargetJSON("team-a", "deploys")
	if err != nil {
		t.Fatalf("MarshalTargetJSON after failure: %v", err)
	}
	items, err := ItemsFromExportPayload(envelopeAfterFailure)
	if err != nil {
		t.Fatalf("ItemsFromExportPayload after failure: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("export items after failure = %d, want 0", len(items))
	}

	count, lastErr := e.ExtractFailures("team-a", "deploys")
	if count != 1 {
		t.Fatalf("extract failure count = %d, want 1 (degradation retained)", count)
	}
	if lastErr == "" {
		t.Fatal("expected non-empty last extraction error after failure")
	}

	// Recovery: valid extraction restores the row and clears failure state.
	st = e.targets[key]
	st.profile = goodProfile
	e.targets[key] = st

	e.processDispatch(context.Background(), gvr, obj, false)

	if store.CountForTarget("team-a", "deploys") != 1 {
		t.Fatalf("item count after recovery = %d, want 1", store.CountForTarget("team-a", "deploys"))
	}
	count, lastErr = e.ExtractFailures("team-a", "deploys")
	if count != 0 {
		t.Fatalf("extract failure count after recovery = %d, want 0", count)
	}
	if lastErr != "" {
		t.Fatalf("last extraction error after recovery = %q, want empty", lastErr)
	}
	if store.NamespaceVersion("team-a") <= versionAfterFailure {
		t.Fatalf("NamespaceVersion after recovery = %d, want > %d",
			store.NamespaceVersion("team-a"), versionAfterFailure)
	}
}
