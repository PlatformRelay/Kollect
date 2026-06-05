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

func TestACLSettingsValidateSubjects(t *testing.T) {
	t.Parallel()

	acl := ACLSettings{
		PublishSubjects:   []string{"inventory/reports"},
		SubscribeSubjects: []string{"inventory/reports"},
	}

	if err := acl.ValidatePublishSubject("inventory/reports"); err != nil {
		t.Fatalf("allowed publish: %v", err)
	}
	if err := acl.ValidatePublishSubject("inventory/other"); err == nil {
		t.Fatal("expected deny for unknown publish subject")
	}
	if err := acl.ValidateSubscribeSubject("inventory/reports"); err != nil {
		t.Fatalf("allowed subscribe: %v", err)
	}
	if err := acl.ValidateSubscribeSubject("inventory/other"); err == nil {
		t.Fatal("expected deny for unknown subscribe subject")
	}

	if err := acl.ValidateClusterID(""); err != nil {
		t.Fatalf("empty allowlist should allow any cluster: %v", err)
	}
}
