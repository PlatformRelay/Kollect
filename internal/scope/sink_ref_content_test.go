// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package scope

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestValidateInventoryFamilySinkRefs_ErrorNamesOffendingRef asserts that when a
// database or event ref is outside the scope allowlist, the rejection error names
// the offending ref (COV-90-06). The existing suite only covers the snapshot allow
// and deny outcome without inspecting the message content.
func TestValidateInventoryFamilySinkRefs_ErrorNamesOffendingRef(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a-scope"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			DatabaseSinkRefs: []string{"pg-primary"},
			EventSinkRefs:    []string{"nats-bus"},
		},
	}

	cases := []struct {
		name       string
		bindings   []kollectdevv1alpha1.InventorySinkBinding
		offender   string
		familyWord string
	}{
		{
			name: "database ref outside allowlist",
			bindings: []kollectdevv1alpha1.InventorySinkBinding{
				{Name: "pg-replica", Family: kollectdevv1alpha1.SinkFamilyDatabase},
			},
			offender:   "pg-replica",
			familyWord: kollectdevv1alpha1.SinkFamilyDatabase,
		},
		{
			name: "event ref outside allowlist",
			bindings: []kollectdevv1alpha1.InventorySinkBinding{
				{Name: "kafka-topic", Family: kollectdevv1alpha1.SinkFamilyEvent},
			},
			offender:   "kafka-topic",
			familyWord: kollectdevv1alpha1.SinkFamilyEvent,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateInventoryFamilySinkRefs(scope, tc.bindings)
			if err == nil {
				t.Fatalf("expected violation for %q", tc.offender)
			}
			if !strings.Contains(err.Error(), tc.offender) {
				t.Fatalf("error must name offending ref %q, got: %v", tc.offender, err)
			}
			if !strings.Contains(err.Error(), scope.Name) {
				t.Fatalf("error should name scope %q, got: %v", scope.Name, err)
			}
			if !strings.Contains(err.Error(), tc.familyWord) {
				t.Fatalf("error should name family %q, got: %v", tc.familyWord, err)
			}
		})
	}
}

// TestValidateInventoryFamilySinkRefs_NamesSpecificLaterOffender asserts that in a
// multi-binding list where earlier refs are allowed and a specific later ref is the
// offender, the error names *that* later ref and not an earlier allowed one.
func TestValidateInventoryFamilySinkRefs_NamesSpecificLaterOffender(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-b-scope"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			SnapshotSinkRefs: []string{"git-a", "git-b"},
			DatabaseSinkRefs: []string{"pg-a"},
		},
	}

	bindings := []kollectdevv1alpha1.InventorySinkBinding{
		{Name: "git-a", Family: kollectdevv1alpha1.SinkFamilySnapshot},
		{Name: "pg-a", Family: kollectdevv1alpha1.SinkFamilyDatabase},
		{Name: "git-rogue", Family: kollectdevv1alpha1.SinkFamilySnapshot},
	}

	err := ValidateInventoryFamilySinkRefs(scope, bindings)
	if err == nil {
		t.Fatal("expected violation for git-rogue")
	}
	if !strings.Contains(err.Error(), "git-rogue") {
		t.Fatalf("error must name the specific offending ref git-rogue, got: %v", err)
	}
	// The allowed earlier refs must not be reported as the offender.
	for _, allowed := range []string{"git-a", "git-b", "pg-a"} {
		if strings.Contains(err.Error(), allowed) {
			t.Fatalf("error should not name allowed ref %q as offender, got: %v", allowed, err)
		}
	}
}

// TestValidateInventoryFamilySinkRefs_EmptyAllowlistSkipsFamily asserts a family with
// an empty allowlist is unconstrained: any ref of that family is accepted even though
// another family enforces its allowlist.
func TestValidateInventoryFamilySinkRefs_EmptyAllowlistSkipsFamily(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "team-c-scope"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			SnapshotSinkRefs: []string{"git-a"},
			// DatabaseSinkRefs intentionally empty -> unconstrained.
		},
	}

	bindings := []kollectdevv1alpha1.InventorySinkBinding{
		{Name: "git-a", Family: kollectdevv1alpha1.SinkFamilySnapshot},
		{Name: "any-db", Family: kollectdevv1alpha1.SinkFamilyDatabase},
	}

	if err := ValidateInventoryFamilySinkRefs(scope, bindings); err != nil {
		t.Fatalf("database family with empty allowlist should accept any ref: %v", err)
	}
}

// TestValidateClusterInventoryClusterScopeSinkRefs_ErrorNamesOffendingRef asserts the
// cluster-scope family ref validator names the offending ref and the cluster scope.
func TestValidateClusterInventoryClusterScopeSinkRefs_ErrorNamesOffendingRef(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectClusterScope{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-scope"},
		Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
			DatabaseSinkRefs: []string{"warehouse"},
		},
	}

	// allowed ref passes
	if err := ValidateClusterInventoryClusterScopeSinkRefs(scope, []kollectdevv1alpha1.InventorySinkBinding{
		{Name: "warehouse", Family: kollectdevv1alpha1.SinkFamilyDatabase},
	}); err != nil {
		t.Fatalf("allowed cluster sink ref: %v", err)
	}

	err := ValidateClusterInventoryClusterScopeSinkRefs(scope, []kollectdevv1alpha1.InventorySinkBinding{
		{Name: "warehouse", Family: kollectdevv1alpha1.SinkFamilyDatabase},
		{Name: "shadow-db", Family: kollectdevv1alpha1.SinkFamilyDatabase},
	})
	if err == nil {
		t.Fatal("expected cluster-scope sink violation for shadow-db")
	}
	if !strings.Contains(err.Error(), "shadow-db") {
		t.Fatalf("error must name offending ref shadow-db, got: %v", err)
	}
	if !strings.Contains(err.Error(), scope.Name) {
		t.Fatalf("error should name cluster scope %q, got: %v", scope.Name, err)
	}
	if strings.Contains(err.Error(), "warehouse") {
		t.Fatalf("allowed ref warehouse should not be named as offender, got: %v", err)
	}
}

// TestValidateClusterInventoryClusterScopeSinkRefs_NilScopeAllows guards the nil path.
func TestValidateClusterInventoryClusterScopeSinkRefs_NilScopeAllows(t *testing.T) {
	t.Parallel()

	if err := ValidateClusterInventoryClusterScopeSinkRefs(nil, []kollectdevv1alpha1.InventorySinkBinding{
		{Name: "anything", Family: kollectdevv1alpha1.SinkFamilyDatabase},
	}); err != nil {
		t.Fatalf("nil cluster scope should allow: %v", err)
	}
}

// TestValidateSinkRefs_DeprecatedShimNamesRef documents that the deprecated shim still
// surfaces the offending ref by delegating to the snapshot-family validator.
func TestValidateSinkRefs_DeprecatedShimNamesRef(t *testing.T) {
	t.Parallel()

	scope := &kollectdevv1alpha1.KollectScope{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-scope"},
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			SnapshotSinkRefs: []string{"git-a"},
		},
	}

	err := ValidateSinkRefs(scope, []string{"git-a", "git-missing"})
	if err == nil {
		t.Fatal("expected violation for git-missing")
	}
	if !strings.Contains(err.Error(), "git-missing") {
		t.Fatalf("error must name offending ref git-missing, got: %v", err)
	}
}
