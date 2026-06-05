// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import "testing"

func TestRenderCommitMessage(t *testing.T) {
	t.Parallel()

	got := renderCommitMessage(
		"chore(inventory): export {namespace}/{name} cluster={cluster}",
		CommitContext{Namespace: "team-a", Name: "deployments", Cluster: "prod-eu"},
	)

	if got != "chore(inventory): export team-a/deployments cluster=prod-eu" {
		t.Fatalf("got %q", got)
	}
}
