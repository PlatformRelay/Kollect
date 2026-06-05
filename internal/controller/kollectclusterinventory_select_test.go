// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectClusterInventoryReconciler_selectedClusterTargets(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	nsMatch := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "team-a",
			Labels: map[string]string{"team": "platform"},
		},
	}
	nsOther := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "team-b",
			Labels: map[string]string{"team": "other"},
		},
	}
	ctMatch := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ct-match",
			Labels: map[string]string{"tier": "gold"},
		},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: "platform-deployments",
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "platform"},
			},
		},
	}
	ctOther := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct-other"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: "other",
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "other"},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nsMatch, nsOther, ctMatch, ctOther).Build()
	r := &KollectClusterInventoryReconciler{Client: cl}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "platform"},
			},
			TargetSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"tier": "gold"},
			},
			TargetRefs: []string{"ct-match"},
		},
	}

	selected, err := r.selectedClusterTargets(context.Background(), inv)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].Name != "ct-match" {
		t.Fatalf("selected = %#v", selected)
	}
}
