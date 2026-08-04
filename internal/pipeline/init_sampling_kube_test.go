// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func deploymentGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
}

func nodeGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}
}

func newInitSamplerDyn(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		deploymentGVR(): "DeploymentList",
		nodeGVR():       "NodeList",
	}, objects...)
}

func unstructuredNamed(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	if namespace != "" {
		u.SetNamespace(namespace)
	}
	u.SetName(name)
	return u
}

func TestNewKubeInitSampler_buildsFromKubeconfig(t *testing.T) {
	t.Parallel()

	path := writeInitTestKubeconfig(t)
	s, err := NewKubeInitSampler(path, "dev")
	if err != nil {
		t.Fatalf("NewKubeInitSampler: %v", err)
	}
	if s == nil || s.dyn == nil {
		t.Fatal("expected non-nil sampler with dynamic client")
	}
}

func TestNewKubeInitSampler_missingKubeconfigErrors(t *testing.T) {
	t.Parallel()

	_, err := NewKubeInitSampler(filepath.Join(t.TempDir(), "missing"), "")
	if err == nil {
		t.Fatal("expected error for missing kubeconfig")
	}
	if !strings.Contains(err.Error(), "build rest config for sampler") {
		t.Fatalf("error wrap: %v", err)
	}
}

func TestSampleRefFromUnstructured(t *testing.T) {
	t.Parallel()

	res := InitResourceInfo{Group: "apps", Version: "v1", Kind: "Deployment", Resource: "deployments"}
	item := unstructuredNamed("apps/v1", "Deployment", "default", "api")
	got := sampleRefFromUnstructured(res, item)
	if got.Namespace != "default" || got.Name != "api" || got.Kind != "Deployment" || got.Resource != "deployments" {
		t.Fatalf("got %+v", got)
	}
}

func TestInitSampleRef_Label_clusterScoped(t *testing.T) {
	t.Parallel()

	ref := InitSampleRef{Group: "", Version: "v1", Kind: "Node", Name: "worker-1"}
	label := ref.Label()
	if !strings.Contains(label, "Node") || !strings.Contains(label, "worker-1") {
		t.Fatalf("label = %q", label)
	}
	if strings.Contains(label, "/") {
		t.Fatalf("cluster-scoped label must not use ns/name form: %q", label)
	}
}

func TestKubeInitSampler_ListSampleCandidates_clusterScoped(t *testing.T) {
	t.Parallel()

	dyn := newInitSamplerDyn(
		unstructuredNamed("v1", "Node", "", "n1"),
		unstructuredNamed("v1", "Node", "", "n2"),
		unstructuredNamed("v1", "Node", "", "n3"),
	)
	s := &KubeInitSampler{dyn: dyn}
	res := InitResourceInfo{Group: "", Version: "v1", Kind: "Node", Resource: "nodes", Namespaced: false}

	got, err := s.ListSampleCandidates(context.Background(), res, nil, 2)
	if err != nil {
		t.Fatalf("ListSampleCandidates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("limit 2 want 2 candidates, got %d", len(got))
	}
	if got[0].Name == "" || got[0].Kind != "Node" {
		t.Fatalf("candidate %+v", got[0])
	}
}

func TestKubeInitSampler_ListSampleCandidates_namespacedAll(t *testing.T) {
	t.Parallel()

	dyn := newInitSamplerDyn(
		unstructuredNamed("apps/v1", "Deployment", "a", "d1"),
		unstructuredNamed("apps/v1", "Deployment", "b", "d2"),
	)
	s := &KubeInitSampler{dyn: dyn}
	res := InitResourceInfo{Group: "apps", Version: "v1", Kind: "Deployment", Resource: "deployments", Namespaced: true}

	// Empty namespaces → cluster-wide NamespaceAll list; limit<=0 uses default cap.
	got, err := s.ListSampleCandidates(context.Background(), res, nil, 0)
	if err != nil {
		t.Fatalf("ListSampleCandidates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	got, err = s.ListSampleCandidates(context.Background(), res, nil, 1)
	if err != nil {
		t.Fatalf("ListSampleCandidates limit=1: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 under limit, got %d", len(got))
	}
}

func TestKubeInitSampler_ListSampleCandidates_perNamespace(t *testing.T) {
	t.Parallel()

	dyn := newInitSamplerDyn(
		unstructuredNamed("apps/v1", "Deployment", "ns-a", "a1"),
		unstructuredNamed("apps/v1", "Deployment", "ns-a", "a2"),
		unstructuredNamed("apps/v1", "Deployment", "ns-b", "b1"),
		unstructuredNamed("apps/v1", "Deployment", "ns-c", "c1"),
	)
	s := &KubeInitSampler{dyn: dyn}
	res := InitResourceInfo{Group: "apps", Version: "v1", Kind: "Deployment", Resource: "deployments", Namespaced: true}

	got, err := s.ListSampleCandidates(context.Background(), res, []string{"ns-a", "ns-b", "ns-c"}, 3)
	if err != nil {
		t.Fatalf("ListSampleCandidates: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want limit 3 across namespaces, got %d: %#v", len(got), got)
	}
	// First two from ns-a, third from ns-b; ns-c skipped once limit hit.
	if got[0].Namespace != "ns-a" || got[2].Namespace != "ns-b" {
		t.Fatalf("namespace order unexpected: %#v", got)
	}
}

func TestKubeInitSampler_GetSampleObject(t *testing.T) {
	t.Parallel()

	nsObj := unstructuredNamed("apps/v1", "Deployment", "default", "api")
	clusterObj := unstructuredNamed("v1", "Node", "", "worker")
	dyn := newInitSamplerDyn(nsObj, clusterObj)
	s := &KubeInitSampler{dyn: dyn}

	obj, err := s.GetSampleObject(context.Background(), InitSampleRef{
		Group: "apps", Version: "v1", Kind: "Deployment", Resource: "deployments",
		Namespace: "default", Name: "api",
	})
	if err != nil {
		t.Fatalf("namespaced get: %v", err)
	}
	if obj["kind"] != "Deployment" {
		t.Fatalf("object = %#v", obj)
	}

	obj, err = s.GetSampleObject(context.Background(), InitSampleRef{
		Group: "", Version: "v1", Kind: "Node", Resource: "nodes", Name: "worker",
	})
	if err != nil {
		t.Fatalf("cluster get: %v", err)
	}
	if obj["kind"] != "Node" {
		t.Fatalf("object = %#v", obj)
	}
}

func TestKubeInitSampler_GetSampleObject_missing(t *testing.T) {
	t.Parallel()

	s := &KubeInitSampler{dyn: newInitSamplerDyn()}
	_, err := s.GetSampleObject(context.Background(), InitSampleRef{
		Group: "apps", Version: "v1", Kind: "Deployment", Resource: "deployments",
		Namespace: "default", Name: "missing",
	})
	if err == nil {
		t.Fatal("expected get error for missing object")
	}
	if !strings.Contains(err.Error(), "get sample") {
		t.Fatalf("error wrap: %v", err)
	}
}

func TestKubeInitSampler_ListSampleCandidates_listError(t *testing.T) {
	t.Parallel()

	resNS := InitResourceInfo{Group: "apps", Version: "v1", Kind: "Deployment", Resource: "deployments", Namespaced: true}
	resCluster := InitResourceInfo{Group: "", Version: "v1", Kind: "Node", Resource: "nodes", Namespaced: false}

	t.Run("perNamespace", func(t *testing.T) {
		t.Parallel()
		dyn := newInitSamplerDyn()
		dyn.PrependReactor("list", "deployments", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("forbidden")
		})
		_, err := (&KubeInitSampler{dyn: dyn}).ListSampleCandidates(context.Background(), resNS, []string{"default"}, 5)
		if err == nil || !strings.Contains(err.Error(), "namespace") {
			t.Fatalf("want namespaced list error, got %v", err)
		}
	})
	t.Run("namespacedAll", func(t *testing.T) {
		t.Parallel()
		dyn := newInitSamplerDyn()
		dyn.PrependReactor("list", "deployments", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("forbidden")
		})
		_, err := (&KubeInitSampler{dyn: dyn}).ListSampleCandidates(context.Background(), resNS, nil, 5)
		if err == nil || !strings.Contains(err.Error(), "for sampling") {
			t.Fatalf("want cluster-wide namespaced list error, got %v", err)
		}
	})
	t.Run("clusterScoped", func(t *testing.T) {
		t.Parallel()
		dyn := newInitSamplerDyn()
		dyn.PrependReactor("list", "nodes", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("forbidden")
		})
		_, err := (&KubeInitSampler{dyn: dyn}).ListSampleCandidates(context.Background(), resCluster, nil, 5)
		if err == nil || !strings.Contains(err.Error(), "for sampling") {
			t.Fatalf("want cluster list error, got %v", err)
		}
	})
}

func TestSamplingNamespaces_and_helpers(t *testing.T) {
	t.Parallel()

	if got := samplingNamespaces(&initDraft{}); got != nil {
		t.Fatalf("empty included want nil, got %v", got)
	}
	got := samplingNamespaces(&initDraft{IncludedNamespaces: []string{"a", "b"}})
	if len(got) != 2 || got[0] != "a" {
		t.Fatalf("got %v", got)
	}

	if sampleAttrType(true) != "boolean" || sampleAttrType(int64(3)) != "number" {
		t.Fatal("sampleAttrType bool/int")
	}
	if sampleAttrType(map[string]any{}) != "object" || sampleAttrType([]any{}) != "array" {
		t.Fatal("sampleAttrType object/array")
	}
	if samplePreviewValue("x", true) != "(redacted)" {
		t.Fatal("sensitive preview")
	}
	long := strings.Repeat("a", 60)
	if !strings.Contains(samplePreviewValue(long, false), "…") {
		t.Fatal("long string preview should truncate")
	}
	if samplePreviewValue(float64(2), false) != "2" {
		t.Fatal("int-like float preview")
	}
	if !skipSampleField("managedFields", "") || skipSampleField("labels", "metadata") {
		t.Fatal("skipSampleField")
	}
}
