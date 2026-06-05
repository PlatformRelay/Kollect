// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestResolveSinkExportInterval_precedence(t *testing.T) {
	t.Parallel()

	refOverride := metav1.Duration{Duration: time.Hour}
	sinkDefault := metav1.Duration{Duration: 5 * time.Minute}
	inventoryDefault := 30 * time.Second
	floor := time.Minute

	ref := kollectdevv1alpha1.InventorySinkRef{
		Name:              "audit-git",
		ExportMinInterval: &refOverride,
	}
	sink := &kollectdevv1alpha1.KollectSink{
		Spec: kollectdevv1alpha1.KollectSinkSpec{ExportMinInterval: &sinkDefault},
	}

	if got := ResolveSinkExportInterval(ref, sink, inventoryDefault, floor); got != time.Hour {
		t.Fatalf("ref override = %v, want 1h", got)
	}

	ref.ExportMinInterval = nil
	if got := ResolveSinkExportInterval(ref, sink, inventoryDefault, floor); got != 5*time.Minute {
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
	errs := ValidateIntervalsAgainstScopeFloor(&tooFast, kollectdevv1alpha1.NewSinkRefList("pg"), floor)
	if len(errs) != 1 {
		t.Fatalf("errs = %v", errs)
	}

	ok := metav1.Duration{Duration: 2 * time.Minute}
	if errs := ValidateIntervalsAgainstScopeFloor(&ok, nil, floor); len(errs) != 0 {
		t.Fatalf("valid default errs = %v", errs)
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
