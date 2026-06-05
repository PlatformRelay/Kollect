// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	kollecterrors "github.com/konih/kollect/internal/errors"
)

func TestCheckInventorySinksReachable_connectionFailed(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	sink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-sink", Namespace: "default"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "git"},
		Status: kollectdevv1alpha1.KollectSinkStatus{
			Conditions: []metav1.Condition{{
				Type:    kollectdevv1alpha1.ConditionConnectionVerified,
				Status:  metav1.ConditionFalse,
				Message: "TLS handshake failed",
			}},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sink).WithStatusSubresource(sink).Build()

	ok, reason, _ := checkInventorySinksReachable(context.Background(), cl, "default", []string{"bad-sink"})
	if ok {
		t.Fatal("expected sinks unreachable")
	}
	if reason != reasonSinkUnreachable {
		t.Fatalf("reason = %q, want %s", reason, reasonSinkUnreachable)
	}
}

func TestCheckTargetNamespaceSinksReachable_noInventory(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	ok, reason, _ := checkTargetNamespaceSinksReachable(context.Background(), cl, "team-a")
	if !ok {
		t.Fatal("expected reachable when no inventory sinkRefs")
	}
	if reason != "NoSinksInNamespace" {
		t.Fatalf("reason = %q, want NoSinksInNamespace", reason)
	}
}

func TestSetSinkReachableFromExport(t *testing.T) {
	t.Parallel()

	var conds []metav1.Condition
	setSinkReachableFromExport(&conds, 3, nil)

	c := apimeta.FindStatusCondition(conds, kollectdevv1alpha1.ConditionSinkReachable)
	if c == nil || c.Status != metav1.ConditionTrue || c.Reason != "ExportSucceeded" {
		t.Fatalf("success condition: %+v", c)
	}

	setSinkReachableFromExport(&conds, 3, kollecterrors.Terminal(context.DeadlineExceeded))
	c = apimeta.FindStatusCondition(conds, kollectdevv1alpha1.ConditionSinkReachable)
	if c == nil || c.Status != metav1.ConditionFalse || c.Reason != kollectdevv1alpha1.ReasonExportTerminal {
		t.Fatalf("terminal export condition: %+v", c)
	}
}

func TestShouldClearTestConnectionAnnotation(t *testing.T) {
	t.Parallel()

	sink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				kollectdevv1alpha1.AnnotationTestConnection: "true",
			},
		},
	}
	if !shouldClearTestConnectionAnnotation(sink) {
		t.Fatal("expected clear when annotation set and spec.connectionTest false")
	}

	sink.Spec.ConnectionTest = true
	if shouldClearTestConnectionAnnotation(sink) {
		t.Fatal("expected no clear when spec.connectionTest true")
	}
}
