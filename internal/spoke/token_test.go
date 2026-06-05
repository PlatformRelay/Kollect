// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import "testing"

func TestServiceAccountTokenFromEnv(t *testing.T) {
	t.Setenv(envSpokeToken, "test-token")

	token, err := serviceAccountToken()
	if err != nil {
		t.Fatal(err)
	}
	if token != "test-token" {
		t.Fatalf("token = %q", token)
	}
}
