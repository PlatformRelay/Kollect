// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestResolveSinkExportInterval_precedence(t *testing.T) {
	t.Parallel()

	refOverride := metav1.Duration{Duration: time.Hour}
	inventoryDefault := 30 * time.Second
	floor := time.Minute

	ref := kollectdevv1alpha1.InventorySinkRef{
		Name:              "audit-git",
		ExportMinInterval: &refOverride,
	}
	sinkInterval := &metav1.Duration{Duration: 5 * time.Minute}

	if got := ResolveSinkExportInterval(ref, sinkInterval, inventoryDefault, floor); got != time.Hour {
		t.Fatalf("ref override = %v, want 1h", got)
	}

	ref.ExportMinInterval = nil
	if got := ResolveSinkExportInterval(ref, sinkInterval, inventoryDefault, floor); got != 5*time.Minute {
		t.Fatalf("sink default = %v, want 5m", got)
	}

	if got := ResolveSinkExportInterval(ref, nil, inventoryDefault, floor); got != time.Minute {
		t.Fatalf("scope floor = %v, want 1m (30s clamped)", got)
	}

	zero := metav1.Duration{Duration: 0}
	ref.ExportMinInterval = &zero
	if got := ResolveSinkExportInterval(ref, nil, inventoryDefault, floor); got != floor {
		t.Fatalf("zero ref with floor = %v, want %v", got, floor)
	}
}

func TestValidateIntervalsAgainstScopeFloor(t *testing.T) {
	t.Parallel()

	floor := time.Minute
	tooFast := metav1.Duration{Duration: 10 * time.Second}
	errs := ValidateIntervalsAgainstScopeFloor(&tooFast, []kollectdevv1alpha1.InventorySinkRefList{kollectdevv1alpha1.NewSinkRefList("pg")}, floor)
	if len(errs) != 1 {
		t.Fatalf("errs = %v", errs)
	}

	ok := metav1.Duration{Duration: 2 * time.Minute}
	if errs := ValidateIntervalsAgainstScopeFloor(&ok, nil, floor); len(errs) != 0 {
		t.Fatalf("valid default errs = %v", errs)
	}
}

func TestInventoryDefaultInterval(t *testing.T) {
	t.Parallel()

	custom := metav1.Duration{Duration: 2 * time.Minute}
	spec := &kollectdevv1alpha1.KollectInventorySpec{ExportMinInterval: &custom}
	if got := InventoryDefaultInterval(spec, time.Hour); got != 2*time.Minute {
		t.Fatalf("inventory interval = %v", got)
	}
	if got := InventoryDefaultInterval(nil, time.Hour); got != time.Hour {
		t.Fatalf("fallback = %v", got)
	}
	if got := InventoryDefaultInterval(&kollectdevv1alpha1.KollectInventorySpec{}, 0); got != DefaultExportMinInterval {
		t.Fatalf("default = %v", got)
	}
}

func TestClusterInventoryDefaultInterval(t *testing.T) {
	t.Parallel()

	custom := metav1.Duration{Duration: 10 * time.Minute}
	spec := &kollectdevv1alpha1.KollectClusterInventorySpec{ExportMinInterval: &custom}
	if got := ClusterInventoryDefaultInterval(spec, 0); got != 10*time.Minute {
		t.Fatalf("cluster interval = %v", got)
	}
}

func TestRequeueAfterForZeroInterval(t *testing.T) {
	t.Parallel()

	if got := RequeueAfterForZeroInterval(time.Minute); got != time.Minute {
		t.Fatalf("positive interval = %v", got)
	}
	if got := RequeueAfterForZeroInterval(0); got != ZeroIntervalWatchdog {
		t.Fatalf("zero interval = %v", got)
	}
}

func TestValidateDurationInterval(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec").Child("exportMinInterval")
	if errs := ValidateDurationInterval(-time.Second, path); len(errs) != 1 {
		t.Fatalf("negative errs = %v", errs)
	}
	if errs := ValidateDurationInterval(MaxExportInterval+time.Second, path); len(errs) != 1 {
		t.Fatalf("over max errs = %v", errs)
	}
	if errs := ValidateDurationInterval(time.Minute, path); len(errs) != 0 {
		t.Fatalf("valid errs = %v", errs)
	}
}

func TestValidateInventorySinkRefs_duplicates(t *testing.T) {
	t.Parallel()

	refs := kollectdevv1alpha1.InventorySinkRefList{
		{Name: "git"},
		{Name: "git"},
	}
	errs := ValidateInventorySinkRefs(refs, nil)
	if len(errs) != 1 {
		t.Fatalf("duplicate errs = %v", errs)
	}
}

func TestScopeMinExportInterval(t *testing.T) {
	t.Parallel()

	if got := ScopeMinExportInterval(nil); got != 0 {
		t.Fatalf("nil scope = %v, want 0", got)
	}
	if got := ScopeMinExportInterval(&kollectdevv1alpha1.KollectScope{}); got != 0 {
		t.Fatalf("unset interval = %v, want 0", got)
	}

	floor := metav1.Duration{Duration: 90 * time.Second}
	scope := &kollectdevv1alpha1.KollectScope{
		Spec: kollectdevv1alpha1.KollectScopeSpec{
			ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{MinExportInterval: &floor},
		},
	}
	if got := ScopeMinExportInterval(scope); got != 90*time.Second {
		t.Fatalf("scope floor = %v, want 90s", got)
	}
}

func TestScopeCeilingMinExportInterval(t *testing.T) {
	t.Parallel()

	if got := ScopeCeilingMinExportInterval(nil); got != 0 {
		t.Fatalf("nil ceiling = %v, want 0", got)
	}
	if got := ScopeCeilingMinExportInterval(&kollectdevv1alpha1.ScopeCeilingSpec{}); got != 0 {
		t.Fatalf("unset ceiling = %v, want 0", got)
	}

	floor := metav1.Duration{Duration: 2 * time.Minute}
	ceiling := &kollectdevv1alpha1.ScopeCeilingSpec{MinExportInterval: &floor}
	if got := ScopeCeilingMinExportInterval(ceiling); got != 2*time.Minute {
		t.Fatalf("ceiling floor = %v, want 2m", got)
	}
}

func TestValidateSinkIntervalAgainstScopeFloor(t *testing.T) {
	t.Parallel()

	floor := time.Minute

	if errs := ValidateSinkIntervalAgainstScopeFloor(nil, 0); len(errs) != 0 {
		t.Fatalf("no floor errs = %v", errs)
	}
	if errs := ValidateSinkIntervalAgainstScopeFloor(nil, floor); len(errs) != 0 {
		t.Fatalf("nil interval errs = %v", errs)
	}

	tooFast := &metav1.Duration{Duration: 10 * time.Second}
	if errs := ValidateSinkIntervalAgainstScopeFloor(tooFast, floor); len(errs) != 1 {
		t.Fatalf("below floor errs = %v, want 1", errs)
	}

	ok := &metav1.Duration{Duration: 5 * time.Minute}
	if errs := ValidateSinkIntervalAgainstScopeFloor(ok, floor); len(errs) != 0 {
		t.Fatalf("above floor errs = %v", errs)
	}
}

func TestValidateIntervalsAgainstScopeFloor_refBelowFloor(t *testing.T) {
	t.Parallel()

	floor := time.Minute
	refs := kollectdevv1alpha1.InventorySinkRefList{
		{Name: "pg", ExportMinInterval: &metav1.Duration{Duration: 5 * time.Second}},
	}
	errs := ValidateIntervalsAgainstScopeFloor(nil, []kollectdevv1alpha1.InventorySinkRefList{refs}, floor)
	if len(errs) != 1 {
		t.Fatalf("ref below floor errs = %v, want 1", errs)
	}

	// A non-positive floor disables enforcement entirely.
	if errs := ValidateIntervalsAgainstScopeFloor(nil, []kollectdevv1alpha1.InventorySinkRefList{refs}, 0); len(errs) != 0 {
		t.Fatalf("disabled floor errs = %v", errs)
	}
}

func TestInventorySinkRefListJSON(t *testing.T) {
	t.Parallel()

	var refs kollectdevv1alpha1.InventorySinkRefList
	if err := refs.UnmarshalJSON([]byte(`["postgres","audit-git"]`)); err != nil {
		t.Fatalf("unmarshal strings: %v", err)
	}
	if got := refs.Names(); len(got) != 2 || got[0] != "postgres" {
		t.Fatalf("names = %v", got)
	}

	payload := `[{"name":"pg","exportMinInterval":"1h"}]`
	if err := refs.UnmarshalJSON([]byte(payload)); err != nil {
		t.Fatalf("unmarshal objects: %v", err)
	}
	if refs[0].ExportMinInterval == nil || refs[0].ExportMinInterval.Duration != time.Hour {
		t.Fatalf("object ref = %+v", refs[0])
	}
}

func TestValidateInventorySinkRefNamespace(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec").Child("sinkRefs").Index(0).Child("namespace")

	// empty namespace → no error
	if errs := validateInventorySinkRefNamespace("", path, false); len(errs) != 0 {
		t.Fatalf("empty namespace: %v", errs)
	}

	// namespace not allowed → forbidden
	errs := validateInventorySinkRefNamespace("team-a", path, false)
	if len(errs) != 1 || errs[0].Type != field.ErrorTypeForbidden {
		t.Fatalf("disallowed namespace: %v", errs)
	}

	// invalid DNS label → invalid
	errs = validateInventorySinkRefNamespace("NOT_VALID", path, true)
	if len(errs) != 1 || errs[0].Type != field.ErrorTypeInvalid {
		t.Fatalf("bad dns label: %v", errs)
	}

	// valid allowed namespace → no error
	if errs := validateInventorySinkRefNamespace("team-b", path, true); len(errs) != 0 {
		t.Fatalf("valid allowed namespace: %v", errs)
	}
}
