// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateConnectionTestSpec(t *testing.T) {
	t.Parallel()

	errs := ValidateConnectionTestSpec(&kollectdevv1alpha1.KollectConnectionTestSpec{
		SinkRef:    "demo-git",
		ProfileRef: "demo-profile",
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	errs = ValidateConnectionTestSpec(&kollectdevv1alpha1.KollectConnectionTestSpec{
		SinkRef:    "demo-git",
		ProfileRef: "other/profile",
	})
	if len(errs) == 0 {
		t.Fatal("expected cross-namespace profileRef error")
	}

	errs = ValidateConnectionTestSpec(nil)
	if len(errs) != 0 {
		t.Fatalf("nil spec: %v", errs)
	}
}

func TestConnectionTestInvalid(t *testing.T) {
	t.Parallel()

	err := ConnectionTestInvalid("probe", ValidateConnectionTestSpec(&kollectdevv1alpha1.KollectConnectionTestSpec{
		SinkRef: "ns/sink",
	})) // cross-namespace ref form
	if err == nil || !strings.Contains(err.Error(), "probe") {
		t.Fatalf("error = %v", err)
	}
}
