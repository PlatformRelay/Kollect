// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package preview

import (
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestRender_postgresDDL(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.DatabaseSinkTypePostgres,
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			Table:  "inventory_items",
			Schema: "public",
		},
	}, "warehouse")
	if out.Postgres == nil || !strings.Contains(out.Postgres.ExpectedDDL, "CREATE TABLE IF NOT EXISTS") {
		t.Fatalf("expected postgres DDL preview, got %#v", out.Postgres)
	}
}

func TestRender_gitCommitSubject(t *testing.T) {
	out := Render(kollectdevv1alpha1.KollectSinkSpec{Type: kollectdevv1alpha1.SnapshotSinkTypeGit}, "git-backup")
	if out.Git == nil || out.Git.SampleCommitSubject == "" {
		t.Fatalf("expected git preview, got %#v", out.Git)
	}
}
