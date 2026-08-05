// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// COV-90-S24: non-deletion ValidateUpdate paths and admission message assertions
// for connection-test, profile, cluster-inventory, and family event sinks.

func TestKollectConnectionTestValidator_ValidateUpdate_nonDeletionRevalidates(t *testing.T) {
	t.Parallel()

	v := &kollectConnectionTestValidator{}
	valid := &kollectdevv1alpha1.KollectConnectionTest{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Spec: kollectdevv1alpha1.KollectConnectionTestSpec{
			SinkRef: kollectdevv1alpha1.ConnectionTestSinkRef{SnapshotSinkRef: "demo-git"},
		},
	}
	if _, err := v.ValidateUpdate(context.Background(), valid, valid); err != nil {
		t.Fatalf("valid non-deletion update: %v", err)
	}

	bad := valid.DeepCopy()
	bad.Spec.SinkRef = kollectdevv1alpha1.ConnectionTestSinkRef{SnapshotSinkRef: "other/demo"}
	_, err := v.ValidateUpdate(context.Background(), valid, bad)
	if err == nil {
		t.Fatal("expected non-deletion update with cross-namespace sinkRef to be rejected")
	}
	if !strings.Contains(err.Error(), "ok") && !strings.Contains(err.Error(), "other/demo") {
		t.Fatalf("error should name offending sinkRef, got: %v", err)
	}
}

func TestKollectProfileValidator_ValidateUpdate_nonDeletionRevalidates(t *testing.T) {
	t.Parallel()

	v := &kollectProfileValidator{}
	valid := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "ok"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "Deployment"},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "name", Path: "$.metadata.name"},
			},
		},
	}
	if _, err := v.ValidateUpdate(context.Background(), valid, valid); err != nil {
		t.Fatalf("valid non-deletion update: %v", err)
	}

	bad := valid.DeepCopy()
	bad.Spec.Attributes = []kollectdevv1alpha1.AttributeSpec{
		{Name: "x", Path: "cel:1 +"},
	}
	_, err := v.ValidateUpdate(context.Background(), valid, bad)
	if err == nil {
		t.Fatal("expected non-deletion update with invalid CEL to be rejected")
	}
	if !strings.Contains(err.Error(), "ok") {
		t.Fatalf("error should name profile ok, got: %v", err)
	}
}

func TestKollectClusterInventoryValidator_ValidateUpdate_nonDeletionRevalidates(t *testing.T) {
	t.Parallel()

	v := testClusterInventoryValidator(t)
	valid := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "a"},
			},
		},
	}
	if _, err := v.ValidateUpdate(context.Background(), valid, valid); err != nil {
		t.Fatalf("valid non-deletion update: %v", err)
	}

	bad := valid.DeepCopy()
	bad.Spec.NamespaceSelector = nil
	_, err := v.ValidateUpdate(context.Background(), valid, bad)
	if err == nil {
		t.Fatal("expected non-deletion update dropping namespaceSelector to be rejected")
	}
	if !strings.Contains(err.Error(), "rollup") {
		t.Fatalf("error should name cluster inventory rollup, got: %v", err)
	}
}

func TestKollectClusterInventoryValidator_scopeFloorViolation_message(t *testing.T) {
	t.Parallel()

	sinkScope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-scope", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				MinExportInterval: &metav1.Duration{Duration: time.Hour},
			},
		},
	}
	v := &kollectClusterInventoryValidator{client: newScopedFakeClient(t, sinkScope)}

	inv := &kollectdevv1alpha1.KollectClusterInventory{
		ObjectMeta: metav1.ObjectMeta{Name: "fast-rollup"},
		Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "a"},
			},
			ExportMinInterval: &metav1.Duration{Duration: time.Second},
		},
	}

	err := v.validate(context.Background(), inv)
	if err == nil {
		t.Fatal("expected scope-floor violation for fast-rollup")
	}
	if !strings.Contains(err.Error(), "fast-rollup") {
		t.Fatalf("error must name offending inventory fast-rollup, got: %v", err)
	}
}

func TestKollectEventSinkValidator_invalidSpec_message(t *testing.T) {
	t.Parallel()

	v := &kollectEventSinkValidator{client: newFakeSinkClient(t)}
	_, err := v.ValidateCreate(context.Background(), &kollectdevv1alpha1.KollectEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "missing-nats", Namespace: "default"},
		Spec:       kollectdevv1alpha1.KollectEventSinkSpec{Type: kollectdevv1alpha1.EventSinkTypeNats},
	})
	if err == nil {
		t.Fatal("expected nats block required")
	}
	if !strings.Contains(err.Error(), "missing-nats") {
		t.Fatalf("error must name offending sink missing-nats, got: %v", err)
	}
}

func TestKollectDatabaseSinkValidator_scopeFloorViolation_message(t *testing.T) {
	t.Parallel()

	teamScope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-scope", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
				MinExportInterval: &metav1.Duration{Duration: time.Hour},
			},
		},
	}
	v := &kollectDatabaseSinkValidator{client: newScopedFakeClient(t, teamScope)}

	sink := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "fast-pg", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory",
			},
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				ExportMinInterval: &metav1.Duration{Duration: time.Second},
			},
		},
	}

	_, err := v.validate(context.Background(), sink)
	if err == nil {
		t.Fatal("expected scope-floor violation for fast-pg")
	}
	if !strings.Contains(err.Error(), "fast-pg") {
		t.Fatalf("error must name offending sink fast-pg, got: %v", err)
	}
}
