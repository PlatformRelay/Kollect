// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// scriptedDiscovery overrides ServerGroupsAndResources so ListResources can be
// exercised without a live API server (nil lists, partial-error, slash names).
type scriptedDiscovery struct {
	fakediscovery.FakeDiscovery
	lists []*metav1.APIResourceList
	err   error
}

func (d *scriptedDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, d.lists, d.err
}

func TestNewKubeInitDiscoverer_buildsFromKubeconfig(t *testing.T) {
	t.Parallel()

	path := writeInitTestKubeconfig(t)
	d, err := NewKubeInitDiscoverer(path, "dev")
	if err != nil {
		t.Fatalf("NewKubeInitDiscoverer: %v", err)
	}
	if d == nil || d.discovery == nil || d.client == nil {
		t.Fatal("expected non-nil discoverer with discovery + client")
	}
}

func TestNewKubeInitDiscoverer_missingKubeconfigErrors(t *testing.T) {
	t.Parallel()

	_, err := NewKubeInitDiscoverer(filepath.Join(t.TempDir(), "no-such-kubeconfig"), "")
	if err == nil {
		t.Fatal("expected error for missing kubeconfig")
	}
	if !strings.Contains(err.Error(), "build rest config") {
		t.Fatalf("error should wrap rest-config failure, got: %v", err)
	}
}

func TestSplitInitGV(t *testing.T) {
	t.Parallel()

	group, version := splitInitGV("apps/v1")
	if group != "apps" || version != "v1" {
		t.Fatalf("apps/v1 -> (%q,%q)", group, version)
	}
	group, version = splitInitGV("v1")
	if group != "" || version != "v1" {
		t.Fatalf("v1 -> (%q,%q)", group, version)
	}
}

func TestInitResourceHasSlash(t *testing.T) {
	t.Parallel()

	if !initResourceHasSlash("pods/status") {
		t.Fatal("pods/status must have slash")
	}
	if initResourceHasSlash("pods") {
		t.Fatal("pods must not have slash")
	}
}

func TestKubeInitDiscoverer_ListResources(t *testing.T) {
	t.Parallel()

	disco := &scriptedDiscovery{
		FakeDiscovery: fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}},
		lists: []*metav1.APIResourceList{
			nil, // skipped
			{
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{
					{Name: "pods", Kind: "Pod", Namespaced: true, Verbs: []string{"list", "get"}},
					{Name: "pods/status", Kind: "Pod", Namespaced: true, Verbs: []string{"get"}},
					{Name: "namespaces", Kind: "Namespace", Namespaced: false, Verbs: []string{"list"}},
				},
			},
			{
				GroupVersion: "apps/v1",
				APIResources: []metav1.APIResource{
					{Name: "deployments", Kind: "Deployment", Namespaced: true, Verbs: []string{"list"}},
					{Name: "deployments", Kind: "Deployment", Namespaced: true, Verbs: []string{"watch"}}, // dedupe
				},
			},
		},
	}
	d := &KubeInitDiscoverer{discovery: disco, client: kubefake.NewSimpleClientset()}

	got, err := d.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 resources (slash + dedupe skipped), got %d: %#v", len(got), got)
	}
	// Sorted by Label().
	if got[0].Kind != "Deployment" || got[0].Group != "apps" {
		t.Fatalf("first want Deployment apps/v1, got %+v", got[0])
	}
	if got[1].Kind != "Namespace" || got[1].Group != "" {
		t.Fatalf("second want core Namespace, got %+v", got[1])
	}
	if got[2].Kind != "Pod" || got[2].Resource != "pods" {
		t.Fatalf("third want Pod, got %+v", got[2])
	}
	if len(got[2].Verbs) != 2 || got[2].Verbs[0] != "list" {
		t.Fatalf("verbs must be copied, got %v", got[2].Verbs)
	}
}

func TestKubeInitDiscoverer_ListResources_errorWhenEmpty(t *testing.T) {
	t.Parallel()

	disco := &scriptedDiscovery{
		FakeDiscovery: fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}},
		err:           errors.New("forbidden"),
	}
	d := &KubeInitDiscoverer{discovery: disco, client: kubefake.NewSimpleClientset()}

	_, err := d.ListResources(context.Background())
	if err == nil {
		t.Fatal("expected error when discovery fails with no lists")
	}
	if !strings.Contains(err.Error(), "server groups/resources") {
		t.Fatalf("error wrap: %v", err)
	}
}

func TestKubeInitDiscoverer_ListResources_partialErrorKeepsLists(t *testing.T) {
	t.Parallel()

	disco := &scriptedDiscovery{
		FakeDiscovery: fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}},
		err:           errors.New("partial"),
		lists: []*metav1.APIResourceList{{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true, Verbs: []string{"list"}},
			},
		}},
	}
	d := &KubeInitDiscoverer{discovery: disco, client: kubefake.NewSimpleClientset()}

	got, err := d.ListResources(context.Background())
	if err != nil {
		t.Fatalf("partial discovery error with lists must not fail: %v", err)
	}
	if len(got) != 1 || got[0].Kind != "ConfigMap" {
		t.Fatalf("got %#v", got)
	}
}

func TestKubeInitDiscoverer_ListNamespaces(t *testing.T) {
	t.Parallel()

	client := kubefake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "zeta"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "alpha"}},
	)
	d := &KubeInitDiscoverer{
		discovery: &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}},
		client:    client,
	}

	got, err := d.ListNamespaces(context.Background())
	if err != nil {
		t.Fatalf("ListNamespaces: %v", err)
	}
	if len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Fatalf("want sorted [alpha zeta], got %v", got)
	}
}

func TestKubeInitDiscoverer_ListNamespaces_error(t *testing.T) {
	t.Parallel()

	client := kubefake.NewSimpleClientset()
	client.PrependReactor("list", "namespaces", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("rbac denied")
	})
	d := &KubeInitDiscoverer{
		discovery: &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}},
		client:    client,
	}

	_, err := d.ListNamespaces(context.Background())
	if err == nil {
		t.Fatal("expected list namespaces error")
	}
	if !strings.Contains(err.Error(), "list namespaces") {
		t.Fatalf("error wrap: %v", err)
	}
}
