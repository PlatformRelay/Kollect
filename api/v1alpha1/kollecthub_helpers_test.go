// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import "testing"

func TestRemoteClusterKey(t *testing.T) {
	t.Parallel()

	ref := RemoteClusterRef{Name: "spoke-a"}
	if got := RemoteClusterKey(ref); got != "kollect-system/spoke-a" {
		t.Fatalf("key = %q", got)
	}

	ref.Namespace = "platform"
	if got := RemoteClusterKey(ref); got != "platform/spoke-a" {
		t.Fatalf("key = %q", got)
	}
}
