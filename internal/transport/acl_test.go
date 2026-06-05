// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import "testing"

func TestACLSettingsFromEnv(t *testing.T) {
	t.Setenv("KOLLECT_TRANSPORT_ACL_ALLOWED_CLUSTERS", "spoke-a, spoke-b")
	t.Setenv("KOLLECT_TRANSPORT_ACL_PUBLISH_SUBJECTS", "inventory/reports")

	got := ACLSettingsFromEnv()
	if len(got.AllowedClusterIDs) != 2 || got.AllowedClusterIDs[0] != "spoke-a" {
		t.Fatalf("AllowedClusterIDs = %#v", got.AllowedClusterIDs)
	}

	if !got.Enabled() {
		t.Fatal("expected Enabled true")
	}
}

func TestACLSettingsValidateClusterID(t *testing.T) {
	acl := ACLSettings{AllowedClusterIDs: []string{"spoke-a"}}

	if err := acl.ValidateClusterID("spoke-a"); err != nil {
		t.Fatalf("allowed cluster: %v", err)
	}

	if err := acl.ValidateClusterID("other"); err == nil {
		t.Fatal("expected deny for unknown cluster")
	}
}
