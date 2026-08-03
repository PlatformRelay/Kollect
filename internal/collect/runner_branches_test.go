// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestNewRunner_buildsDiscoveryMapper(t *testing.T) {
	t.Parallel()

	dyn := newFakeDynClient()
	kube := kubefake.NewSimpleClientset()

	// Construction must not contact the API server: the deferred discovery mapper
	// is lazy, so a non-routable host is fine here.
	r, err := NewRunner(&rest.Config{Host: "https://127.0.0.1:1"}, dyn, kube, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if r.Store() == nil {
		t.Fatal("NewRunner() returned runner without store")
	}
}

func TestNewRunner_invalidRestConfigErrors(t *testing.T) {
	t.Parallel()

	bad := &rest.Config{Host: "https://exa mple.com"} // space makes the URL unparsable

	_, err := NewRunner(bad, newFakeDynClient(), kubefake.NewSimpleClientset(), nil)
	if err == nil || !strings.Contains(err.Error(), "build discovery client") {
		t.Fatalf("err = %v, want wrapped discovery-client build error", err)
	}
}

func TestExtractionFailure_Error(t *testing.T) {
	t.Parallel()

	namespaced := ExtractionFailure{
		Target: "default/t1", Namespace: "team-a", Name: "obj", UID: "u1", Reason: "attribute \"x\": boom",
	}
	want := "target default/t1: extract team-a/obj (uid=u1): attribute \"x\": boom"
	if got := namespaced.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	clusterScoped := ExtractionFailure{Target: "default/t1", Name: "node-1", UID: "u2", Reason: "r"}
	wantCluster := "target default/t1: extract node-1 (uid=u2): r"
	if got := clusterScoped.Error(); got != wantCluster {
		t.Fatalf("Error() = %q, want %q", got, wantCluster)
	}
}

func TestRunner_profileNotFoundSkips(t *testing.T) {
	t.Parallel()

	r, err := NewRunnerWithMapper(newFakeDynClient(), kubefake.NewSimpleClientset(), staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	target := testTarget("default", "t1", "missing-profile")

	result, err := r.Run(context.Background(), nil, []kollectdevv1alpha1.KollectTarget{target})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.SkippedTargets) != 1 || result.SkippedTargets[0].Reason != "profile-not-found" {
		t.Fatalf("SkippedTargets = %v, want one profile-not-found skip", result.SkippedTargets)
	}
	if result.SkippedTargets[0].Name != "default/t1" {
		t.Fatalf("skip name = %q, want default/t1", result.SkippedTargets[0].Name)
	}
	if !result.Degraded() {
		t.Fatal("profile-not-found run must be degraded")
	}
}

func TestRunner_transientListErrorSkips(t *testing.T) {
	t.Parallel()

	dyn := newFakeDynClient()
	dyn.PrependReactor("list", "secrets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("etcdserver: leader changed")
	})
	kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})

	r, err := NewRunnerWithMapper(dyn, kube, staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	result, err := r.Run(context.Background(),
		[]kollectdevv1alpha1.KollectProfile{testProfile()},
		[]kollectdevv1alpha1.KollectTarget{testTarget("default", "t1", "test-profile")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.SkippedTargets) != 1 || result.SkippedTargets[0].Reason != "transient" {
		t.Fatalf("SkippedTargets = %v, want one transient skip", result.SkippedTargets)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("Errors = %v, want none for a per-namespace list failure", result.Errors)
	}
}

func TestRunner_invalidNamespaceSelectorIsFatalPerTarget(t *testing.T) {
	t.Parallel()

	kube := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})

	r, err := NewRunnerWithMapper(newFakeDynClient(), kube, staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	target := testTarget("default", "t1", "test-profile")
	target.Spec.NamespaceSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "k", Operator: "Bogus"}},
	}

	result, err := r.Run(context.Background(),
		[]kollectdevv1alpha1.KollectProfile{testProfile()},
		[]kollectdevv1alpha1.KollectTarget{target})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("Errors = %v, want exactly one fatal per-target error", result.Errors)
	}

	msg := result.Errors[0].Error()
	for _, want := range []string{"target default/t1", "resolve namespaces", "parse namespaceSelector"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error = %q, want it to contain %q", msg, want)
		}
	}
	if !result.Degraded() {
		t.Fatal("run with fatal per-target errors must be degraded")
	}
}

func TestRunner_namespaceListErrorIsFatalPerTarget(t *testing.T) {
	t.Parallel()

	kube := kubefake.NewSimpleClientset()
	kube.PrependReactor("list", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("apiserver overloaded")
	})

	r, err := NewRunnerWithMapper(newFakeDynClient(), kube, staticSecretMapper(), nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	result, err := r.Run(context.Background(),
		[]kollectdevv1alpha1.KollectProfile{testProfile()},
		[]kollectdevv1alpha1.KollectTarget{testTarget("default", "t1", "test-profile")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Error(), "list namespaces") {
		t.Fatalf("Errors = %v, want one wrapped list-namespaces error", result.Errors)
	}
}

var nodesGVR = schema.GroupVersionResource{Version: "v1", Resource: "nodes"}

func TestRunner_clusterScopedResourceListsWithoutNamespace(t *testing.T) {
	t.Parallel()

	node := &unstructured.Unstructured{}
	node.SetAPIVersion("v1")
	node.SetKind("Node")
	node.SetName("node-1")
	node.SetUID("uid-node-1")

	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{nodesGVR: "NodeList"}, node)

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "Node"}, meta.RESTScopeRoot)

	// No namespaces exist in the cluster: a cluster-scoped target must not care.
	r, err := NewRunnerWithMapper(dyn, kubefake.NewSimpleClientset(), mapper, nil)
	if err != nil {
		t.Fatalf("NewRunnerWithMapper() error = %v", err)
	}

	profile := kollectdevv1alpha1.KollectProfile{}
	profile.Name = "nodes-profile"
	profile.Spec.TargetGVK = kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Node"}

	result, err := r.Run(context.Background(),
		[]kollectdevv1alpha1.KollectProfile{profile},
		[]kollectdevv1alpha1.KollectTarget{testTarget("default", "t1", "nodes-profile")})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1", result.ItemCount)
	}
	if result.Degraded() {
		t.Fatalf("cluster-scoped run unexpectedly degraded: %+v", result)
	}

	items := r.Store().SnapshotTarget("default", "t1")
	if len(items) != 1 || items[0].Name != "node-1" || items[0].Namespace != "" {
		t.Fatalf("items = %+v, want single cluster-scoped node-1 with empty namespace", items)
	}
}

func TestRedactExtractionReason(t *testing.T) {
	t.Parallel()

	if got := redactExtractionReason(nil); got != "extraction failed" {
		t.Fatalf("redactExtractionReason(nil) = %q, want fallback", got)
	}

	long := fmt.Errorf("attribute %q: %s", "x", strings.Repeat("a", 500))
	got := redactExtractionReason(long)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("long reason not truncated with ellipsis: %q", got)
	}
	if len(got) != 256+len("…") {
		t.Fatalf("truncated reason length = %d, want %d", len(got), 256+len("…"))
	}

	short := fmt.Errorf("attribute %q: missing", "x")
	if got := redactExtractionReason(short); got != short.Error() {
		t.Fatalf("short reason altered: %q", got)
	}
}

func TestSortExtractionFailures_fullKeyOrder(t *testing.T) {
	t.Parallel()

	failures := []ExtractionFailure{
		{Target: "b/t", Namespace: "ns", Name: "n", UID: "1"},
		{Target: "a/t", Namespace: "ns-z", Name: "n", UID: "1"},
		{Target: "a/t", Namespace: "ns-a", Name: "z", UID: "1"},
		{Target: "a/t", Namespace: "ns-a", Name: "a", UID: "2"},
		{Target: "a/t", Namespace: "ns-a", Name: "a", UID: "1"},
	}

	sortExtractionFailures(failures)

	wantOrder := []string{"1", "2", "1", "1", "1"}
	wantNames := []string{"a", "a", "z", "n", "n"}
	for i := range failures {
		if failures[i].Name != wantNames[i] || failures[i].UID != wantOrder[i] {
			t.Fatalf("order[%d] = %+v, want name=%q uid=%q (target,ns,name,uid tie-break)",
				i, failures[i], wantNames[i], wantOrder[i])
		}
	}
	if failures[0].Target != "a/t" || failures[4].Target != "b/t" {
		t.Fatalf("target order wrong: %+v", failures)
	}
}

func TestLabelSelectorString(t *testing.T) {
	t.Parallel()

	if got := labelSelectorString(nil); got != "" {
		t.Fatalf("labelSelectorString(nil) = %q, want empty", got)
	}

	invalid := &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "k", Operator: "Bogus"}},
	}
	if got := labelSelectorString(invalid); got != "" {
		t.Fatalf("labelSelectorString(invalid) = %q, want empty fail-safe", got)
	}

	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"zeta": "2", "alpha": "1", "mid": "3"}}
	want := "alpha=1,mid=3,zeta=2"
	for i := 0; i < 5; i++ {
		if got := labelSelectorString(sel); got != want {
			t.Fatalf("labelSelectorString() = %q, want deterministic sorted %q (iteration %d)", got, want, i)
		}
	}
}
