// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectConnectionTestValidator_ValidateCreate(t *testing.T) {
	t.Parallel()

	v := &kollectConnectionTestValidator{}

	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Spec:       kollectdevv1alpha1.KollectConnectionTestSpec{SinkRef: "demo-git"},
	})
	if err != nil {
		t.Fatalf("valid create: %v", err)
	}

	_, err = v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec:       kollectdevv1alpha1.KollectConnectionTestSpec{SinkRef: "other/demo"},
	})
	if err == nil {
		t.Fatal("expected validation error for cross-namespace sinkRef")
	}
}

func TestKollectConnectionTestValidator_validate(t *testing.T) {
	t.Parallel()

	v := &kollectConnectionTestValidator{}

	if err := v.validate(&kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Spec:       kollectdevv1alpha1.KollectConnectionTestSpec{SinkRef: "demo-git"},
	}); err != nil {
		t.Fatalf("valid spec: %v", err)
	}

	if err := v.validate(&kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec:       kollectdevv1alpha1.KollectConnectionTestSpec{SinkRef: "other/demo"},
	}); err == nil {
		t.Fatal("expected validation error for cross-namespace sinkRef")
	}
}

func TestKollectConnectionTestValidator_ValidateUpdateDeletion(t *testing.T) {
	t.Parallel()

	v := &kollectConnectionTestValidator{}
	now := metav1.Now()
	old := &kollectdevv1alpha1.KollectConnectionTest{
		Spec: kollectdevv1alpha1.KollectConnectionTestSpec{SinkRef: "demo"},
	}
	newTest := old.DeepCopy()
	newTest.DeletionTimestamp = &now

	if _, err := v.ValidateUpdate(context.Background(), old, newTest); err != nil {
		t.Fatalf("deletion update: %v", err)
	}

	if _, err := v.ValidateDelete(context.Background(), old); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
