// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/operator"
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
		ObjectMeta: metav1.ObjectMeta{Name: "deployments", Namespace: operator.DefaultSecretNamespace},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterScope, profile).Build()
	v := &kollectClusterTargetValidator{client: cl}

	target := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: "deployments",
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
	badGVK.Spec.ProfileRef = "deployments"
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
}
