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
	"github.com/konih/kollect/internal/sink"
)

func TestKollectClusterTargetValidator_ValidateCreate(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = kollectdevv1alpha1.AddToScheme(scheme)
	v := &kollectClusterTargetValidator{client: fake.NewClientBuilder().WithScheme(scheme).Build()}

	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: "platform-deployments",
		},
	})
	if err == nil {
		t.Fatal("expected validation error for missing namespaceSelector")
	}

	_, err = v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: "platform-deployments",
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "platform"},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected valid cluster target: %v", err)
	}
}

func TestKollectClusterTargetValidator_ValidateUpdateDeletion(t *testing.T) {
	t.Parallel()

	v := &kollectClusterTargetValidator{client: fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()}
	now := metav1.Now()
	target := &kollectdevv1alpha1.KollectClusterTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "ct"},
		Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
			ProfileRef: "platform-deployments",
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "platform"},
			},
		},
	}
	deleting := target.DeepCopy()
	deleting.DeletionTimestamp = &now

	if _, err := v.ValidateUpdate(context.Background(), target, deleting); err != nil {
		t.Fatalf("deletion update: %v", err)
	}

	if _, err := v.ValidateDelete(context.Background(), target); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestResolveClusterTargetProfileForWebhook(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	clusterProfile := &kollectdevv1alpha1.KollectClusterProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-deployments"},
		Spec: kollectdevv1alpha1.KollectClusterProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
	}
	namespacedProfile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "team-deployments", Namespace: sink.DefaultSecretNamespace},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterProfile, namespacedProfile).Build()

	got, err := resolveClusterTargetProfileForWebhook(context.Background(), cl, "platform-deployments")
	if err != nil || got.Spec.TargetGVK.Kind != "Deployment" {
		t.Fatalf("cluster profile = %#v err=%v", got, err)
	}

	got, err = resolveClusterTargetProfileForWebhook(context.Background(), cl, "team-deployments")
	if err != nil || got.Spec.TargetGVK.Kind != "StatefulSet" {
		t.Fatalf("namespaced profile = %#v err=%v", got, err)
	}

	if _, err := resolveClusterTargetProfileForWebhook(context.Background(), cl, "missing"); err == nil {
		t.Fatal("expected not found for missing profile")
	}
}
