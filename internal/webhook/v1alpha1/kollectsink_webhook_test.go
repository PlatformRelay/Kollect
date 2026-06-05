// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestKollectSinkValidator_rejectsUnknownType(t *testing.T) {
	t.Parallel()

	v := &kollectSinkValidator{}
	ks := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-sink", Namespace: "kollect-system"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "unknown"},
	}

	if _, err := v.validate(ks); err == nil {
		t.Fatal("expected validation error for unknown sink type")
	}
}

func TestKollectSinkValidator_ValidateLifecycle(t *testing.T) {
	t.Parallel()

	v := &kollectSinkValidator{}
	ks := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: kollectdevv1alpha1.SinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	if _, err := v.ValidateCreate(context.Background(), ks); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := v.ValidateUpdate(context.Background(), ks, ks); err != nil {
		t.Fatalf("update: %v", err)
	}

	now := metav1.Now()
	deleting := ks.DeepCopy()
	deleting.DeletionTimestamp = &now
	if _, err := v.ValidateUpdate(context.Background(), ks, deleting); err != nil {
		t.Fatalf("deletion update: %v", err)
	}

	if _, err := v.ValidateDelete(context.Background(), ks); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestKollectSinkValidator_acceptsPostgres(t *testing.T) {
	t.Parallel()

	v := &kollectSinkValidator{}
	ks := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type: kollectdevv1alpha1.SinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory_items",
			},
		},
	}

	if _, err := v.validate(ks); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
