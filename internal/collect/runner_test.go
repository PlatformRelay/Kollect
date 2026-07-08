// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var secretGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}

func staticSecretMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	m.Add(schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, meta.RESTScopeNamespace)

	return m
}

func newFakeDynClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()

	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{secretGVR: "SecretList"}, objects...)
}

func unstructuredSecret(namespace, name string, annotations map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("Secret")
	u.SetNamespace(namespace)
	u.SetName(name)
	u.SetUID(types.UID("uid-" + namespace + "-" + name))
	if annotations != nil {
		u.SetAnnotations(annotations)
	}

	return u
}

func testProfile(attrs ...kollectdevv1alpha1.AttributeSpec) kollectdevv1alpha1.KollectProfile {
	p := kollectdevv1alpha1.KollectProfile{}
	p.Name = "test-profile"
	p.Spec.TargetGVK = kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Secret"}
	p.Spec.Attributes = attrs

	return p
}

func testTarget(namespace, name, profileRef string) kollectdevv1alpha1.KollectTarget {
	tgt := kollectdevv1alpha1.KollectTarget{}
	tgt.Namespace = namespace
	tgt.Name = name
	tgt.Spec.ProfileRef = profileRef

	return tgt
}

func TestRunner_successfulExtract(t *testing.T) {
	t.Parallel()

	obj := unstructuredSecret("default", "my-release", map[string]string{"chart": "myapp-1.2.3"})
	dyn := newFakeDynClient(obj)
	kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})

	r, err := NewRunnerWithMapper(dyn, kube, staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	profile := testProfile(kollectdevv1alpha1.AttributeSpec{Name: "chart", Path: "$.metadata.annotations.chart"})
	target := testTarget("default", "t1", "test-profile")

	result, err := r.Run(context.Background(), []kollectdevv1alpha1.KollectProfile{profile}, []kollectdevv1alpha1.KollectTarget{target})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1", result.ItemCount)
	}
	if len(result.SkippedTargets) != 0 || len(result.Errors) != 0 {
		t.Fatalf("expected no skips/errors, got skips=%v errors=%v", result.SkippedTargets, result.Errors)
	}

	items := r.Store().SnapshotTarget("default", "t1")
	if len(items) != 1 {
		t.Fatalf("expected 1 stored item, got %d", len(items))
	}
	if items[0].Attributes["chart"] != "myapp-1.2.3" {
		t.Errorf("Attributes[chart] = %v, want myapp-1.2.3", items[0].Attributes["chart"])
	}
}

func TestRunner_forbiddenGVKIsSkipped(t *testing.T) {
	t.Parallel()

	dyn := newFakeDynClient()
	dyn.PrependReactor("list", "secrets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "", fmt.Errorf("denied"))
	})
	kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})

	r, err := NewRunnerWithMapper(dyn, kube, staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	profile := testProfile()
	target := testTarget("default", "t1", "test-profile")

	result, err := r.Run(context.Background(), []kollectdevv1alpha1.KollectProfile{profile}, []kollectdevv1alpha1.KollectTarget{target})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.SkippedTargets) != 1 || result.SkippedTargets[0].Reason != "forbidden" {
		t.Fatalf("expected 1 forbidden skip, got %v", result.SkippedTargets)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no fatal errors, got %v", result.Errors)
	}
}

func TestRunner_gvkNotFoundInCluster(t *testing.T) {
	t.Parallel()

	dyn := newFakeDynClient()
	kube := kubefake.NewSimpleClientset()

	emptyMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})

	r, err := NewRunnerWithMapper(dyn, kube, emptyMapper, nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	profile := testProfile()
	target := testTarget("default", "t1", "test-profile")

	result, err := r.Run(context.Background(), []kollectdevv1alpha1.KollectProfile{profile}, []kollectdevv1alpha1.KollectTarget{target})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.SkippedTargets) != 1 || result.SkippedTargets[0].Reason != "gvk-not-found" {
		t.Fatalf("expected 1 gvk-not-found skip, got %v", result.SkippedTargets)
	}
}

func TestRunner_nameFilterAppliedClientSide(t *testing.T) {
	t.Parallel()

	a := unstructuredSecret("default", "helm-secret-A", nil)
	b := unstructuredSecret("default", "helm-secret-B", nil)
	dyn := newFakeDynClient(a, b)
	kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})

	r, err := NewRunnerWithMapper(dyn, kube, staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	profile := testProfile()
	target := testTarget("default", "t1", "test-profile")
	target.Spec.Names = []string{"helm-secret-A"}

	result, err := r.Run(context.Background(), []kollectdevv1alpha1.KollectProfile{profile}, []kollectdevv1alpha1.KollectTarget{target})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1", result.ItemCount)
	}

	items := r.Store().SnapshotTarget("default", "t1")
	if len(items) != 1 || items[0].Name != "helm-secret-A" {
		t.Fatalf("expected only helm-secret-A, got %+v", items)
	}
}

func TestRunner_namespaceSelector(t *testing.T) {
	t.Parallel()

	appSecret := unstructuredSecret("app-ns", "s1", nil)
	sysSecret := unstructuredSecret("system-ns", "s2", nil)
	dyn := newFakeDynClient(appSecret, sysSecret)
	kube := kubefake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns", Labels: map[string]string{"collect": "true"}}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "system-ns"}},
	)

	r, err := NewRunnerWithMapper(dyn, kube, staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	profile := testProfile()
	target := testTarget("default", "t1", "test-profile")
	target.Spec.NamespaceSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"collect": "true"}}

	result, err := r.Run(context.Background(), []kollectdevv1alpha1.KollectProfile{profile}, []kollectdevv1alpha1.KollectTarget{target})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1 (only app-ns matches selector)", result.ItemCount)
	}

	items := r.Store().SnapshotTarget("default", "t1")
	if len(items) != 1 || items[0].Namespace != "app-ns" {
		t.Fatalf("expected only app-ns item, got %+v", items)
	}
}

func TestRunner_excludedNamespaces(t *testing.T) {
	t.Parallel()

	appSecret := unstructuredSecret("app", "s1", nil)
	kubeSystemSecret := unstructuredSecret("kube-system", "s2", nil)
	dyn := newFakeDynClient(appSecret, kubeSystemSecret)
	kube := kubefake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
	)

	r, err := NewRunnerWithMapper(dyn, kube, staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	profile := testProfile()
	target := testTarget("default", "t1", "test-profile")
	target.Spec.ExcludedNamespaces = []string{"kube-system"}

	result, err := r.Run(context.Background(), []kollectdevv1alpha1.KollectProfile{profile}, []kollectdevv1alpha1.KollectTarget{target})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1 (kube-system excluded)", result.ItemCount)
	}

	items := r.Store().SnapshotTarget("default", "t1")
	if len(items) != 1 || items[0].Namespace != "app" {
		t.Fatalf("expected only app-namespace item, got %+v", items)
	}
}

func TestRunner_scrubKeys(t *testing.T) {
	t.Parallel()

	// Scrubber redacts sensitive keys *nested inside* a map/slice attribute value (ADR-0303);
	// it does not match on the top-level attribute name. Extract the whole annotations map so
	// this test exercises the real Scrubber.ScrubAttributes call the runner wires in.
	obj := unstructuredSecret("default", "s1", map[string]string{"password": "super-secret-value"})
	dyn := newFakeDynClient(obj)
	kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})

	r, err := NewRunnerWithMapper(dyn, kube, staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	profile := testProfile(kollectdevv1alpha1.AttributeSpec{Name: "meta", Path: "$.metadata.annotations"})
	target := testTarget("default", "t1", "test-profile")

	if _, err := r.Run(context.Background(), []kollectdevv1alpha1.KollectProfile{profile}, []kollectdevv1alpha1.KollectTarget{target}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	items := r.Store().SnapshotTarget("default", "t1")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	meta, ok := items[0].Attributes["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected meta attribute to be a map, got %T", items[0].Attributes["meta"])
	}
	if meta["password"] == "super-secret-value" {
		t.Error("expected nested password key to be scrubbed, got raw value")
	}
}

func TestRunner_multipleTargetsSameGVK(t *testing.T) {
	t.Parallel()

	aSecret := unstructuredSecret("ns-a", "s1", nil)
	bSecret := unstructuredSecret("ns-b", "s2", nil)
	dyn := newFakeDynClient(aSecret, bSecret)
	kube := kubefake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-a"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-b"}},
	)

	r, err := NewRunnerWithMapper(dyn, kube, staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	profile := testProfile()
	targetA := testTarget("default", "target-a", "test-profile")
	targetA.Spec.IncludedNamespaces = []string{"ns-a"}
	targetB := testTarget("default", "target-b", "test-profile")
	targetB.Spec.IncludedNamespaces = []string{"ns-b"}

	result, err := r.Run(context.Background(),
		[]kollectdevv1alpha1.KollectProfile{profile},
		[]kollectdevv1alpha1.KollectTarget{targetA, targetB})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ItemCount != 2 {
		t.Fatalf("ItemCount = %d, want 2", result.ItemCount)
	}

	itemsA := r.Store().SnapshotTarget("default", "target-a")
	itemsB := r.Store().SnapshotTarget("default", "target-b")
	if len(itemsA) != 1 || itemsA[0].Namespace != "ns-a" {
		t.Errorf("target-a items = %+v", itemsA)
	}
	if len(itemsB) != 1 || itemsB[0].Namespace != "ns-b" {
		t.Errorf("target-b items = %+v", itemsB)
	}
}
