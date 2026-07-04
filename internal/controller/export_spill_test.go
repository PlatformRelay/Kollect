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

	gitSink := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type:             kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{Endpoint: "https://example.com/repo.git"},
		},
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
		[]kollectdevv1alpha1.InventorySinkBinding{
			{Name: "git", Family: kollectdevv1alpha1.SinkFamilySnapshot},
		},
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

	s3Sink := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "s3", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeS3},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(s3Sink).Build()

	gate, err := assessExportSpill(
		context.Background(),
		cl,
		logr.Discard(),
		export.SpillWarnBytes+1,
		validation.MaxExportBytesGlobal(),
		"team-a",
		[]kollectdevv1alpha1.InventorySinkBinding{
			{Name: "s3", Family: kollectdevv1alpha1.SinkFamilySnapshot},
		},
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

	gitSink := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit},
	}
	s3Sink := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "s3", Namespace: "team-a"},
		Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeS3},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gitSink, s3Sink).Build()

	ok, err := hasObjectStoreSink(context.Background(), cl, "team-a", []kollectdevv1alpha1.InventorySinkBinding{
		{Name: "git", Family: kollectdevv1alpha1.SinkFamilySnapshot},
	})
	if err != nil || ok {
		t.Fatalf("git only = ok=%v err=%v", ok, err)
	}

	ok, err = hasObjectStoreSink(context.Background(), cl, "team-a", []kollectdevv1alpha1.InventorySinkBinding{
		{Name: "git", Family: kollectdevv1alpha1.SinkFamilySnapshot},
		{Name: "s3", Family: kollectdevv1alpha1.SinkFamilySnapshot},
	})
	if err != nil || !ok {
		t.Fatalf("with s3 = ok=%v err=%v", ok, err)
	}
}

func TestHasObjectStoreSink_skipsNonSnapshotFamily(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	// database-family bindings are skipped, so hasObjectStoreSink returns false without error
	ok, err := hasObjectStoreSink(context.Background(), cl, "team-a", []kollectdevv1alpha1.InventorySinkBinding{
		{Name: "pg", Family: kollectdevv1alpha1.SinkFamilyDatabase},
	})
	if err != nil || ok {
		t.Fatalf("database binding should be skipped: ok=%v err=%v", ok, err)
	}
}

func TestNoteExportShardWarning_belowThreshold(t *testing.T) {
	t.Parallel()

	conditions := []metav1.Condition{{
		Type:   conditionExportShardWarn,
		Status: metav1.ConditionTrue,
		Reason: reasonExportShardWarn,
	}}
	changed := noteExportShardWarning(&conditions, 1, validation.ExportShardWarnRows-1)
	if changed {
		t.Fatal("expected false for count below threshold")
	}
	for _, c := range conditions {
		if c.Type == conditionExportShardWarn {
			t.Fatal("condition should have been removed")
		}
	}
}

func TestNoteExportShardWarning_atThreshold(t *testing.T) {
	t.Parallel()

	var conditions []metav1.Condition
	changed := noteExportShardWarning(&conditions, 1, validation.ExportShardWarnRows)
	if !changed {
		t.Fatal("expected true for count at threshold")
	}
	found := false
	for _, c := range conditions {
		if c.Type == conditionExportShardWarn && c.Status == metav1.ConditionTrue {
			found = true
		}
	}
	if !found {
		t.Fatal("ExportShardWarning condition not set")
	}
}

func TestRecordSpillGateMetrics_notDegraded(t *testing.T) {
	t.Parallel()

	// must not panic or error on non-degraded gate
	recordSpillGateMetrics(spillGateResult{degraded: false})
}

func TestRecordSpillGateMetrics_payloadTooLarge(t *testing.T) {
	t.Parallel()

	recordSpillGateMetrics(spillGateResult{degraded: true, reason: spillReasonPayloadTooLarge})
}

func TestRecordSpillGateMetrics_spillRequired(t *testing.T) {
	t.Parallel()

	recordSpillGateMetrics(spillGateResult{degraded: true, reason: spillReasonSpillRequired})
}
