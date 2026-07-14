// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"sync"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestResolveBindingMaxExportBytes proves the override-not-clamp semantics: a
// per-binding ceiling replaces the fallback wholesale, whether smaller or larger.
func TestResolveBindingMaxExportBytes(t *testing.T) {
	t.Parallel()

	fallback := int64(1000)

	if got := ResolveBindingMaxExportBytes(nil, fallback); got != fallback {
		t.Fatalf("nil override = %d, want fallback %d", got, fallback)
	}

	smaller := int64(500)
	if got := ResolveBindingMaxExportBytes(&smaller, fallback); got != smaller {
		t.Fatalf("smaller override = %d, want %d", got, smaller)
	}

	// Override may exceed the fallback — must NOT be clamped down to fallback.
	larger := int64(5000)
	if got := ResolveBindingMaxExportBytes(&larger, fallback); got != larger {
		t.Fatalf("larger override = %d, want %d (override, not clamp)", got, larger)
	}

	// Non-positive overrides fall back.
	zero := int64(0)
	if got := ResolveBindingMaxExportBytes(&zero, fallback); got != fallback {
		t.Fatalf("zero override = %d, want fallback %d", got, fallback)
	}
}

// TestValidateInventorySinkRefs_maxExportBytes proves per-ref maxExportBytes is
// validated against the operator GLOBAL cap (not the inventory-wide value): a
// binding above the inventory value but below global is accepted; above global rejected.
func TestValidateInventorySinkRefs_maxExportBytes(t *testing.T) {
	t.Parallel()

	SetMaxExportBytesGlobal(1000)
	t.Cleanup(func() { SetMaxExportBytesGlobal(defaultMaxExportBytesGlobal) })

	// Accepted: positive, at or below global.
	underGlobal := int64(800)
	refs := kollectdevv1alpha1.InventorySinkRefList{
		{Name: "audit-git", MaxExportBytes: &underGlobal},
	}
	if errs := ValidateInventorySinkRefs(refs, nil); len(errs) != 0 {
		t.Fatalf("under-global maxExportBytes rejected: %v", errs)
	}

	// Rejected: above the global cap.
	overGlobal := int64(2000)
	refs = kollectdevv1alpha1.InventorySinkRefList{
		{Name: "audit-git", MaxExportBytes: &overGlobal},
	}
	if errs := ValidateInventorySinkRefs(refs, nil); len(errs) != 1 {
		t.Fatalf("over-global maxExportBytes errs = %v, want 1", errs)
	}

	// Rejected: non-positive when set.
	zero := int64(0)
	refs = kollectdevv1alpha1.InventorySinkRefList{
		{Name: "audit-git", MaxExportBytes: &zero},
	}
	if errs := ValidateInventorySinkRefs(refs, nil); len(errs) != 1 {
		t.Fatalf("zero maxExportBytes errs = %v, want 1", errs)
	}
}

// TestMaxExportBytesGlobal_concurrentAccess guards the global operator cap against
// data races: it is read on every admission validation yet may be reconfigured at
// runtime. Regressing the atomic backing (e.g. back to a plain int64) makes this
// fail under `go test -race`.
func TestMaxExportBytesGlobal_concurrentAccess(t *testing.T) {
	t.Cleanup(func() { SetMaxExportBytesGlobal(defaultMaxExportBytesGlobal) })

	over := int64(1 << 40)
	refs := kollectdevv1alpha1.InventorySinkRefList{
		{Name: "audit-git", MaxExportBytes: &over},
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); SetMaxExportBytesGlobal(1000) }()
		go func() {
			defer wg.Done()
			_ = ValidateInventorySinkRefs(refs, nil)
			_ = MaxExportBytesGlobal()
		}()
	}
	wg.Wait()
}
