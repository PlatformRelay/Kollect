// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestEngineNamespacesForClusterTarget_sorted pins that the namespaces bound
// to a cluster target come back sorted regardless of registration order, and
// that a differently named target is not matched.
func TestEngineNamespacesForClusterTarget_sorted(t *testing.T) {
	t.Parallel()

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "DeploymentList"},
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	engine, err := NewEngine(dyn, nil, NewStore(), EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-binding-profile", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}

	register := func(name, ns string) {
		t.Helper()

		target := &kollectdevv1alpha1.KollectTarget{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectTargetSpec{
				ProfileRef: "cluster-binding-profile",
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{corev1.LabelMetadataName: ns},
				},
			},
		}
		if err := engine.RegisterTarget(ctx, target, profile, RegisterTargetOptions{
			EffectiveNamespaces: []string{ns},
		}); err != nil {
			t.Fatalf("register %s/%s: %v", ns, name, err)
		}
	}

	for _, ns := range []string{"team-c", "team-a", "team-b"} {
		register("shared-target", ns)
	}
	register("other-target", "team-z")

	got := engine.NamespacesForClusterTarget("shared-target")
	want := []string{"team-a", "team-b", "team-c"}
	if !slices.Equal(got, want) {
		t.Fatalf("NamespacesForClusterTarget() = %v, want %v", got, want)
	}
}
