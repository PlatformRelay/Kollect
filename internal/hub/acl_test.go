// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub_test

import (
	"testing"

	"github.com/konih/kollect/internal/hub"
)

func TestValidateClusterACL(t *testing.T) {
	t.Parallel()

	if err := hub.ValidateClusterACL("spoke-a", nil, false); err != nil {
		t.Fatalf("open allowlist: %v", err)
	}

	if err := hub.ValidateClusterACL("spoke-a", []string{"spoke-a", "spoke-b"}, true); err != nil {
		t.Fatalf("registered cluster: %v", err)
	}

	if err := hub.ValidateClusterACL("rogue", []string{"spoke-a"}, true); err == nil {
		t.Fatal("expected unregistered cluster error")
	}

	if err := hub.ValidateClusterACL("spoke-a", nil, true); err == nil {
		t.Fatal("expected fail-closed rejection when enforced with empty allowlist")
	}
}
