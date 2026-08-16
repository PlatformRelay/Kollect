// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

// The namespace refresh in syncEngineTargets is a compensating control: RegisterTarget no
// longer refreshes when the caller supplies EffectiveNamespaces, and this controller
// always does. If the refresh fails, registering anyway would mean collecting against a
// stale namespace cache — namespace watch opt-outs invisible, objects collected that an
// operator explicitly excluded, and no error to notice. Failing the sync instead surfaces
// it as Degraded/InformerRegistrationFailed and retries.
func TestSyncEngineTargetsFailsWhenTheNamespaceRefreshFails(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	kube := kubefake.NewSimpleClientset() //nolint:staticcheck // SimpleClientset is sufficient here
	listErr := errors.New("namespace list refused")
	kube.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, listErr
	})

	// A working dynamic client on purpose: registration must be prevented by the refresh
	// failure itself, not by the informer being unable to start. Without this the test
	// would "fail" on a nil-client panic under mutation and prove nothing.
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			{Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		},
	)

	engine, err := collect.NewEngine(dyn, kube, collect.NewStore(), collect.EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	engineCtx, cancelEngine := context.WithCancel(context.Background())
	t.Cleanup(cancelEngine)
	if startErr := engine.Start(engineCtx); startErr != nil {
		t.Fatalf("Start: %v", startErr)
	}

	r := &KollectClusterTargetReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
		Engine: engine,
	}

	ct := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "fleet"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{Name: "apps", Namespace: "kollect-system"},
		},
	}
	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "apps", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
		},
	}

	syncErr := r.syncEngineTargets(context.Background(), ct, profile, []string{"team-a"}, collect.ScopeCeiling{})
	if syncErr == nil {
		t.Fatal("syncEngineTargets returned nil; a failed namespace refresh must not register against a stale cache")
	}
	if !errors.Is(syncErr, listErr) {
		t.Fatalf("syncEngineTargets error = %v, want it to wrap %v", syncErr, listErr)
	}
	if !strings.Contains(syncErr.Error(), "refresh namespace cache") {
		t.Fatalf("error %q should name the failed step so the Degraded message is actionable", syncErr)
	}

	// Nothing was registered, so a later successful reconcile starts clean rather than
	// leaving a target bound to namespaces resolved from a cache that never loaded.
	if got := engine.NamespacesForClusterTarget(ct.Name); len(got) != 0 {
		t.Fatalf("registered namespaces = %v, want none", got)
	}
}
