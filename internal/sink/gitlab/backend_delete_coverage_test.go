// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"testing"

	"github.com/platformrelay/kollect/internal/sink/git"
)

// A deletion with no candidate paths has no work.
func TestBackend_DeleteExport_EmptyPathsIsNoop(t *testing.T) {
	t.Parallel()

	b := &Backend{cfg: Config{Endpoint: "file:///tmp/kollect-gitlab-empty.git"}, auth: git.Auth{}}
	deleted, err := b.DeleteExport(t.Context(), nil)
	if err != nil || deleted != nil {
		t.Fatalf("DeleteExport(nil) = %v/%v, want nil/nil", deleted, err)
	}
}

// A rejected candidate path surfaces as a deletion error.
func TestBackend_DeleteExport_InvalidPathErrors(t *testing.T) {
	t.Parallel()

	b := &Backend{cfg: Config{Endpoint: "file:///tmp/kollect-gitlab-invalid.git"}, auth: git.Auth{}}
	if _, err := b.DeleteExport(t.Context(), []string{"../evil.json"}); err == nil {
		t.Fatal("traversal path must surface as an error")
	}
}

// Outside branchMR mode a successful deletion returns without probing the
// remote for a stranded feature branch.
func TestBackend_DeleteExport_NonMRModeReturns(t *testing.T) {
	remote, _ := seedGitLabTestRemote(t)

	b := &Backend{
		cfg:  Config{Endpoint: "file://" + remote},
		auth: git.Auth{},
	}

	deleted, err := b.DeleteExport(t.Context(), []string{"inventory/team-a/never-exported.json"})
	if err != nil || len(deleted) != 0 {
		t.Fatalf("non-MR no-op = %v/%v, want empty/nil", deleted, err)
	}
}
