// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestKollectTargetValidator_scopeAdmission(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	scopeObj := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-scope", Namespace: "sec-ops"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedNamespaces: []string{"team-a"},
				DeniedNamespaces:  []string{"kube-system"},
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
		},
	}
	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "deployments", Namespace: "sec-ops"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(scopeObj, profile).Build()
	v := &kollectTargetValidator{client: cl}

	valid := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "sec-ops"},
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			ProfileRef: "deployments",
			CollectionFilterSpec: kollectdevv1alpha1.CollectionFilterSpec{
				IncludedNamespaces: []string{"team-a"},
			},
		},
	}
	if err := v.validate(context.Background(), valid); err != nil {
		t.Fatalf("expected accept: %v", err)
	}

	deniedNS := valid.DeepCopy()
	deniedNS.Name = "denied-ns"
	deniedNS.Spec.IncludedNamespaces = []string{"kube-system"}
	if err := v.validate(context.Background(), deniedNS); err == nil {
		t.Fatal("expected reject for denied namespace")
	}

	outOfScope := valid.DeepCopy()
	outOfScope.Name = "out-of-scope"
	outOfScope.Spec.IncludedNamespaces = []string{"team-b"}
	if err := v.validate(context.Background(), outOfScope); err == nil {
		t.Fatal("expected reject for namespace outside allowlist")
	}

	badGVK := valid.DeepCopy()
	badGVK.Name = "bad-gvk"
	badGVK.Spec.ResourceRules = []kollectdevv1alpha1.ResourceRule{
		{GVK: kollectdevv1alpha1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}},
	}
	if err := v.validate(context.Background(), badGVK); err == nil {
		t.Fatal("expected reject for GVK outside scope")
	}
}

func TestKollectTargetValidator_scopeAdmissionMissingProfile(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	scopeObj := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-scope", Namespace: "sec-ops"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedNamespaces: []string{"team-a"},
				DeniedNamespaces:  []string{"kube-system"},
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
		},
	}

	// No KollectProfile object: the Target references one that does not exist yet.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(scopeObj).Build()
	v := &kollectTargetValidator{client: cl}

	base := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "pending", Namespace: "sec-ops"},
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			ProfileRef: "not-created-yet",
			CollectionFilterSpec: kollectdevv1alpha1.CollectionFilterSpec{
				IncludedNamespaces: []string{"team-a"},
			},
		},
	}
	if err := v.validate(context.Background(), base); err != nil {
		t.Fatalf("in-scope target must stay admissible before its profile exists: %v", err)
	}

	deniedNS := base.DeepCopy()
	deniedNS.Spec.IncludedNamespaces = []string{"kube-system"}
	if err := v.validate(context.Background(), deniedNS); err == nil {
		t.Fatal("expected reject for denied namespace when the profile is missing")
	}

	outOfScope := base.DeepCopy()
	outOfScope.Spec.IncludedNamespaces = []string{"team-b"}
	if err := v.validate(context.Background(), outOfScope); err == nil {
		t.Fatal("expected reject for namespace outside allowlist when the profile is missing")
	}

	badGVK := base.DeepCopy()
	badGVK.Spec.ResourceRules = []kollectdevv1alpha1.ResourceRule{
		{GVK: kollectdevv1alpha1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}},
	}
	if err := v.validate(context.Background(), badGVK); err == nil {
		t.Fatal("expected reject for resourceRules GVK outside scope when the profile is missing")
	}
}

func TestKollectTargetValidator_validateWatchMode(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)
	v := &kollectTargetValidator{client: fake.NewClientBuilder().WithScheme(scheme).Build()}

	if err := v.validate(context.Background(), &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "deployment-images", WatchMode: ""},
	}); err != nil {
		t.Fatalf("empty watchMode: %v", err)
	}

	if err := v.validate(context.Background(), &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			ProfileRef: "deployment-images",
			WatchMode:  "Maybe",
		},
	}); err == nil {
		t.Fatal("expected error for invalid watchMode")
	}

	if err := v.validate(context.Background(), &kollectdevv1alpha1.KollectTarget{
		Spec: kollectdevv1alpha1.KollectTargetSpec{ProfileRef: "team-a/deployment-images"},
	}); err == nil {
		t.Fatal("expected error for cross-namespace profileRef")
	}
}

func TestKollectTargetValidator_ValidateLifecycle(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)
	v := &kollectTargetValidator{client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "deployments", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			ProfileRef: "deployment-images",
			WatchMode:  kollectdevv1alpha1.WatchModeAll,
		},
	}

	if _, err := v.ValidateCreate(context.Background(), target); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := v.ValidateUpdate(context.Background(), target, target); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, err := v.ValidateDelete(context.Background(), target); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
