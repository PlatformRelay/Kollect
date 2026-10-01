// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import "testing"

// RemoteBranchExists surfaces endpoint parsing, clone-URL validation and remote
// listing failures instead of treating them as "branch absent".
func TestRemoteBranchExists_ErrorPaths(t *testing.T) {
	t.Parallel()

	if _, err := RemoteBranchExists(t.Context(), Config{Endpoint: "://bad"}.withDefaults(), Auth{}, "main"); err == nil {
		t.Fatal("malformed endpoint must error")
	}
	if _, err := RemoteBranchExists(t.Context(), Config{Endpoint: "git://example.com/repo.git"}.withDefaults(), Auth{}, "main"); err == nil {
		t.Fatal("unsupported clone URL scheme must error")
	}
	if _, err := RemoteBranchExists(t.Context(), Config{Endpoint: "file:///nonexistent/kollect/missing.git"}.withDefaults(), Auth{}, "main"); err == nil {
		t.Fatal("unreachable remote must error on list")
	}
}
