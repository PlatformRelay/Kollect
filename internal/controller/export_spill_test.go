// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/validation"
)

func TestAssessExportSpill_payloadTooLarge(t *testing.T) {
	t.Parallel()

	gate, err := assessExportSpill(
		context.Background(),
		nil,
		logr.Discard(),
		validation.MaxExportBytesGlobal()+1,
		validation.MaxExportBytesGlobal(),
		"team-a",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !gate.degraded || gate.reason != "PayloadTooLarge" {
		t.Fatalf("gate = %#v", gate)
	}
}

func TestAssessExportSpill_requiresObjectStore(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	gitSink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "git", Endpoint: "https://example.com/repo.git"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gitSink).Build()

	payload := export.SpillWarnBytes + 1
	gate, err := assessExportSpill(
		context.Background(),
		cl,
		logr.Discard(),
		payload,
		validation.MaxExportBytesGlobal(),
		"team-a",
		[]string{"git"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !gate.degraded || gate.reason != "SpillRequired" {
		t.Fatalf("gate = %#v", gate)
	}
}

func TestAssessExportSpill_objectStoreSatisfiesSpill(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	s3Sink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "s3", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SinkTypeS3},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(s3Sink).Build()

	gate, err := assessExportSpill(
		context.Background(),
		cl,
		logr.Discard(),
		export.SpillWarnBytes+1,
		validation.MaxExportBytesGlobal(),
		"team-a",
		[]string{"s3"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if gate.degraded {
		t.Fatalf("expected no degradation with object store, gate = %#v", gate)
	}
}

func TestHasObjectStoreSink(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	gitSink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: "git"},
	}
	s3Sink := &kollectdevv1alpha1.KollectSink{
		ObjectMeta: metav1.ObjectMeta{Name: "s3", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SinkTypeS3},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gitSink, s3Sink).Build()

	ok, err := hasObjectStoreSink(context.Background(), cl, "team-a", []string{"git"})
	if err != nil || ok {
		t.Fatalf("git only = ok=%v err=%v", ok, err)
	}

	ok, err = hasObjectStoreSink(context.Background(), cl, "team-a", []string{"git", "s3"})
	if err != nil || !ok {
		t.Fatalf("with s3 = ok=%v err=%v", ok, err)
	}
}

func TestRecordSpillGateMetrics(t *testing.T) {
	t.Parallel()

	recordSpillGateMetrics(spillGateResult{})
	recordSpillGateMetrics(spillGateResult{degraded: true, reason: "PayloadTooLarge"})
	recordSpillGateMetrics(spillGateResult{degraded: true, reason: "SpillRequired"})
}
