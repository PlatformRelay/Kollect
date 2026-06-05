// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink"
)

func TestShouldTestConnection(t *testing.T) {
	t.Parallel()

	trueVal := true
	falseVal := false

	tests := []struct {
		name string
		sink kollectdevv1alpha1.KollectSink
		want bool
	}{
		{
			name: "spec connectionTest true",
			sink: kollectdevv1alpha1.KollectSink{
				Spec: kollectdevv1alpha1.KollectSinkSpec{ConnectionTest: &trueVal},
			},
			want: true,
		},
		{
			name: "spec connectionTest false",
			sink: kollectdevv1alpha1.KollectSink{
				Spec: kollectdevv1alpha1.KollectSinkSpec{ConnectionTest: &falseVal},
			},
			want: false,
		},
		{
			name: "annotation true",
			sink: kollectdevv1alpha1.KollectSink{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						kollectdevv1alpha1.AnnotationTestConnection: "true",
					},
				},
				Spec: kollectdevv1alpha1.KollectSinkSpec{ConnectionTest: &falseVal},
			},
			want: true,
		},
		{
			name: "annotation TRUE case insensitive",
			sink: kollectdevv1alpha1.KollectSink{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						kollectdevv1alpha1.AnnotationTestConnection: "TRUE",
					},
				},
				Spec: kollectdevv1alpha1.KollectSinkSpec{ConnectionTest: &falseVal},
			},
			want: true,
		},
		{
			name: "default probe enabled",
			sink: kollectdevv1alpha1.KollectSink{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldTestConnection(&tt.sink); got != tt.want {
				t.Fatalf("shouldTestConnection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunConnectionTest_unsupportedType(t *testing.T) {
	t.Parallel()

	_, err := sink.RunConnectionTest(t.Context(), kollectdevv1alpha1.KollectSinkSpec{Type: "unknown"}, sink.BuildContext{})
	if err == nil {
		t.Fatal("expected error for unsupported sink type")
	}
}

func TestKollectSinkReconciler_skipsWithoutProbe(t *testing.T) {
	t.Parallel()

	falseVal := false
	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "quiet", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type:           "git",
			Endpoint:       "https://example.com/repo.git",
			ConnectionTest: &falseVal,
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build()
	r := &KollectSinkReconciler{Client: c, Scheme: scheme}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: sinkObj.Namespace, Name: sinkObj.Name},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func TestKollectSinkReconciler_connectionFailed(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad",
			Namespace: "team-a",
			Annotations: map[string]string{
				kollectdevv1alpha1.AnnotationTestConnection: "true",
			},
		},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     "git",
			Endpoint: "://invalid",
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(sinkObj).
		WithObjects(sinkObj).
		Build()

	r := &KollectSinkReconciler{Client: c, Scheme: scheme}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: sinkObj.Namespace, Name: sinkObj.Name},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var got kollectdevv1alpha1.KollectSink
	nn := types.NamespacedName{Namespace: sinkObj.Namespace, Name: sinkObj.Name}
	if err := c.Get(context.Background(), nn, &got); err != nil {
		t.Fatal(err)
	}

	cond := apimeta.FindStatusCondition(got.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		t.Fatalf("ConnectionVerified = %+v", cond)
	}
}

func TestKollectSinkReconciler_clearTestConnectionAnnotation(t *testing.T) {
	t.Parallel()

	falseVal := false
	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "anno",
			Namespace: "team-a",
			Annotations: map[string]string{
				kollectdevv1alpha1.AnnotationTestConnection: "true",
			},
		},
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			Type:           "git",
			Endpoint:       "https://example.com/x.git",
			ConnectionTest: &falseVal,
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sinkObj).Build()
	r := &KollectSinkReconciler{Client: c, Scheme: scheme}

	if err := r.clearTestConnectionAnnotation(context.Background(), sinkObj); err != nil {
		t.Fatal(err)
	}

	var got kollectdevv1alpha1.KollectSink
	nn := types.NamespacedName{Namespace: sinkObj.Namespace, Name: sinkObj.Name}
	if err := c.Get(context.Background(), nn, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Annotations[kollectdevv1alpha1.AnnotationTestConnection]; ok {
		t.Fatal("annotation should be cleared")
	}
}

func TestKollectSinkReconciler_setConnectionVerified(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sinkObj := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "verified", Namespace: "team-a", Generation: 1},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "git"},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(sinkObj).
		WithObjects(sinkObj).
		Build()

	r := &KollectSinkReconciler{Client: c, Scheme: scheme}
	if _, err := r.setConnectionVerified(context.Background(), sinkObj, "verified"); err != nil {
		t.Fatal(err)
	}

	cond := apimeta.FindStatusCondition(sinkObj.Status.Conditions, kollectdevv1alpha1.ConditionConnectionVerified)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("ConnectionVerified = %+v", cond)
	}
}

func TestSetTLSInsecureCondition(t *testing.T) {
	t.Parallel()

	r := &KollectSinkReconciler{}
	insecure := &kollectdevv1alpha1.KollectSink{
		Spec: kollectdevv1alpha1.KollectSinkSpec{
			TLS: &kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true},
		},
	}
	r.setTLSInsecureCondition(insecure)

	cond := apimeta.FindStatusCondition(insecure.Status.Conditions, kollectdevv1alpha1.ConditionTLSInsecure)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("TLSInsecure = %+v", cond)
	}

	secure := &kollectdevv1alpha1.KollectSink{
		Status: kollectdevv1alpha1.KollectSinkStatus{
			Conditions: []metav1.Condition{{
				Type:   kollectdevv1alpha1.ConditionTLSInsecure,
				Status: metav1.ConditionTrue,
			}},
		},
	}
	r.setTLSInsecureCondition(secure)
	if apimeta.FindStatusCondition(secure.Status.Conditions, kollectdevv1alpha1.ConditionTLSInsecure) != nil {
		t.Fatal("expected TLSInsecure removed when secure")
	}
}
