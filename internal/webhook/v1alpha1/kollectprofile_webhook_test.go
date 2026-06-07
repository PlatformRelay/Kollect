// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectProfileValidator_ValidateCreate(t *testing.T) {
	t.Parallel()

	v := &kollectProfileValidator{}

	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Deployment"},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "x", Path: "cel:1 +"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for invalid path")
	}

	_, err = v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Deployment"},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "image", Path: `$.spec.template.spec.containers[0].image`},
				{Name: "n", Path: "cel:1 + 1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected valid profile: %v", err)
	}
}

func TestKollectProfileValidator_resourceExportSecretGuard(t *testing.T) {
	t.Parallel()

	v := &kollectProfileValidator{}
	secret := kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Secret"}

	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "secret-snapshot"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: secret,
			Export:    &kollectdevv1alpha1.ExportSpec{Mode: kollectdevv1alpha1.ExportModeResource},
		},
	})
	if err == nil {
		t.Fatal("expected Secret resource export to be rejected without opt-in annotation")
	}

	_, err = v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "secret-snapshot",
			Annotations: map[string]string{kollectdevv1alpha1.AllowFullResourceExportAnnotation: "true"},
		},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: secret,
			Export:    &kollectdevv1alpha1.ExportSpec{Mode: kollectdevv1alpha1.ExportModeResource},
		},
	})
	if err != nil {
		t.Fatalf("expected Secret resource export to pass with annotation: %v", err)
	}
}

func TestKollectProfileValidator_ValidateUpdateDelete(t *testing.T) {
	t.Parallel()

	v := &kollectProfileValidator{}
	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Deployment"},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "name", Path: "$.metadata.name"},
			},
		},
	}

	now := metav1.Now()
	deleting := profile.DeepCopy()
	deleting.DeletionTimestamp = &now
	if _, err := v.ValidateUpdate(context.Background(), profile, deleting); err != nil {
		t.Fatalf("deletion update: %v", err)
	}

	if _, err := v.ValidateDelete(context.Background(), profile); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
