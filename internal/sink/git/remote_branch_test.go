// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import "testing"

func TestRemoteBranchExists(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	branch := "kollect/default/team"

	exists, err := RemoteBranchExists(t.Context(), cfg, Auth{}, branch)
	if err != nil {
		t.Fatalf("RemoteBranchExists on absent branch: %v", err)
	}
	if exists {
		t.Fatal("branch must not exist before it is pushed")
	}

	runGitC(t, remote, "update-ref", "refs/heads/"+branch, "HEAD")

	exists, err = RemoteBranchExists(t.Context(), cfg, Auth{}, branch)
	if err != nil {
		t.Fatalf("RemoteBranchExists on pushed branch: %v", err)
	}
	if !exists {
		t.Fatal("branch must exist after it is pushed")
	}
}

func TestRemoteBranchExists_rejectsInvalidRef(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote}.withDefaults()

	if _, err := RemoteBranchExists(t.Context(), cfg, Auth{}, "bad ref~name"); err == nil {
		t.Fatal("expected validation error for an invalid branch name")
	}
}
