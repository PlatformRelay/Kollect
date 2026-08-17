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

func TestKollectClusterTargetValidator_validateClusterScope(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	clusterScope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
				AllowedNamespaces: []string{"team-a"},
				DeniedNamespaces:  []string{"kube-system"},
			},
		},
	}
	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "deployments", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterScope, profile).Build()
	v := &kollectClusterTargetValidator{client: cl}

	target := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{Name: "deployments", Namespace: "kollect-system"},
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "platform"},
			},
			CollectionFilterSpec: kollectdevv1alpha1.CollectionFilterSpec{
				IncludedNamespaces: []string{"team-a"},
			},
		},
	}
	if err := v.validateClusterScope(context.Background(), target); err != nil {
		t.Fatalf("expected allowed target: %v", err)
	}

	badGVK := target.DeepCopy()
	badGVK.Spec.ProfileRef = kollectdevv1alpha1.NamespacedObjectReference{Name: "deployments", Namespace: "kollect-system"}
	badGVK.Spec.CollectionFilterSpec = kollectdevv1alpha1.CollectionFilterSpec{
		ResourceRules: []kollectdevv1alpha1.ResourceRule{
			{GVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Pod"}},
		},
	}
	if err := v.validateClusterScope(context.Background(), badGVK); err == nil {
		t.Fatal("expected GVK scope violation")
	}

	badNS := target.DeepCopy()
	badNS.Spec.CollectionFilterSpec = kollectdevv1alpha1.CollectionFilterSpec{
		IncludedNamespaces: []string{"kube-system"},
	}
	if err := v.validateClusterScope(context.Background(), badNS); err == nil {
		t.Fatal("expected denied namespace violation")
	}

	missingProfile := target.DeepCopy()
	missingProfile.Spec.ProfileRef.Name = "does-not-exist"
	missingProfile.Spec.CollectionFilterSpec = kollectdevv1alpha1.CollectionFilterSpec{
		IncludedNamespaces: []string{"kube-system"},
	}
	if err := v.validateClusterScope(context.Background(), missingProfile); err == nil {
		t.Fatal("expected denied namespace violation when the profile is missing")
	}

	missingOK := target.DeepCopy()
	missingOK.Spec.ProfileRef.Name = "does-not-exist"
	if err := v.validateClusterScope(context.Background(), missingOK); err != nil {
		t.Fatalf("missing in-scope profile should still admit: %v", err)
	}
}

func TestKollectClusterTargetValidator_missingProfileSpecDerivedChecks(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	clusterScope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "platform"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
					{Group: "apps", Version: "v1", Kind: "Deployment"},
				},
			},
			AllowedStaticRefNamespaces: []string{"kollect-system"},
		},
	}

	// No KollectProfile object: profileRef points at one that does not exist yet.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterScope).Build()
	v := &kollectClusterTargetValidator{client: cl}

	base := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{Name: "not-created-yet", Namespace: "kollect-system"},
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "platform"},
			},
		},
	}
	if err := v.validateClusterScope(context.Background(), base); err != nil {
		t.Fatalf("in-scope target must stay admissible before its profile exists: %v", err)
	}

	// resourceRules GVKs come from the spec, not the profile — CollectRuleGVKs
	// ignores the profile GVK entirely once resourceRules is set, so a missing
	// profile is no reason to skip them.
	deniedRuleGVK := base.DeepCopy()
	deniedRuleGVK.Spec.ResourceRules = []kollectdevv1alpha1.ResourceRule{
		{GVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Secret"}},
	}
	if err := v.validateClusterScope(context.Background(), deniedRuleGVK); err == nil {
		t.Fatal("expected resourceRules GVK violation when the profile is missing")
	}

	// allowedStaticRefNamespaces is spec-derived too; #304 added the check but no
	// fixture exercised it, because allowedStaticRefNamespaces was unset there.
	deniedRefNS := base.DeepCopy()
	deniedRefNS.Spec.ProfileRef.Namespace = "tenant-b"
	if err := v.validateClusterScope(context.Background(), deniedRefNS); err == nil {
		t.Fatal("expected profileRef namespace violation when the profile is missing")
	}
}
