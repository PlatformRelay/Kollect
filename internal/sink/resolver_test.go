// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestResolveSink_snapshotNamespaced(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	snap := &kollectdevv1alpha1.KollectSnapshotSink{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
			Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
			SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
				Endpoint: "https://example.com/repo.git",
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(snap).Build()

	resolved, err := ResolveSink(context.Background(), cl, ResolveOptions{
		Namespace: "team-a",
		Name:      "git",
		Family:    kollectdevv1alpha1.SinkFamilySnapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Spec.Type != kollectdevv1alpha1.SnapshotSinkTypeGit {
		t.Fatalf("type = %q", resolved.Spec.Type)
	}
	if resolved.Namespace != "team-a" {
		t.Fatalf("resolved = %#v", resolved)
	}
}

func TestResolveSink_databaseNamespaced(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	db := &kollectdevv1alpha1.KollectDatabaseSink{
		ObjectMeta: metav1.ObjectMeta{Name: "warehouse", Namespace: "kollect-system"},
		Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
			Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
			Postgres: &kollectdevv1alpha1.PostgresSpec{
				DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
				Table:       "inventory",
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).Build()

	resolved, err := ResolveSink(context.Background(), cl, ResolveOptions{
		Namespace: "kollect-system",
		Name:      "warehouse",
		Family:    kollectdevv1alpha1.SinkFamilyDatabase,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Namespace != "kollect-system" || resolved.Family != kollectdevv1alpha1.SinkFamilyDatabase {
		t.Fatalf("resolved = %#v", resolved)
	}
}

func TestResolveOptionsForBinding(t *testing.T) {
	t.Parallel()

	opts := ResolveOptionsForBinding("team-a", kollectdevv1alpha1.InventorySinkBinding{
		Name:   "pg",
		Family: kollectdevv1alpha1.SinkFamilyDatabase,
	})
	if opts.Namespace != "team-a" || opts.Name != "pg" || opts.Family != kollectdevv1alpha1.SinkFamilyDatabase {
		t.Fatalf("opts = %#v", opts)
	}
}

func TestSinkNamespaceForResolved(t *testing.T) {
	t.Parallel()

	if got := SinkNamespaceForResolved(nil, "fallback"); got != "fallback" {
		t.Fatalf("nil resolved = %q", got)
	}
	if got := SinkNamespaceForResolved(&ResolvedSink{Namespace: "team-a"}, "fallback"); got != "team-a" {
		t.Fatalf("namespaced = %q", got)
	}
	if got := SinkNamespaceForResolved(&ResolvedSink{}, "fallback"); got != "fallback" {
		t.Fatalf("empty = %q", got)
	}
}

func TestResolveSink_guardErrors(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	// nil client guard.
	if _, err := ResolveSink(context.Background(), nil, ResolveOptions{Name: "x"}); err == nil {
		t.Fatal("nil client: expected error")
	}

	// empty name guard.
	if _, err := ResolveSink(context.Background(), cl, ResolveOptions{Family: kollectdevv1alpha1.SinkFamilySnapshot}); err == nil {
		t.Fatal("empty name: expected error")
	}

	// unknown family default branch.
	if _, err := ResolveSink(context.Background(), cl, ResolveOptions{
		Name:   "x",
		Family: "bogus-family",
	}); err == nil {
		t.Fatal("unknown family: expected error")
	}
}

func TestResolveSink_anyFamilyNotFound(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	// No sink of any family exists → resolveSinkAnyFamily exhausts all families
	// and returns the last not-found error (unresolved ref path).
	if _, err := ResolveSink(context.Background(), cl, ResolveOptions{
		Namespace: "team-a",
		Name:      "does-not-exist",
	}); err == nil {
		t.Fatal("missing sink across all families: expected not-found error")
	}
}

func TestResolveSinkAnyFamily(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	ev := &kollectdevv1alpha1.KollectEventSink{
		ObjectMeta: metav1.ObjectMeta{Name: "stream", Namespace: "team-a"},
		Spec: kollectdevv1alpha1.KollectEventSinkSpec{
			Type: kollectdevv1alpha1.EventSinkTypeNats,
			Nats: &kollectdevv1alpha1.NatsSpec{Subject: "inventory.>"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ev).Build()

	resolved, err := ResolveSink(context.Background(), cl, ResolveOptions{
		Namespace: "team-a",
		Name:      "stream",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Family != kollectdevv1alpha1.SinkFamilyEvent {
		t.Fatalf("family = %q", resolved.Family)
	}
}
