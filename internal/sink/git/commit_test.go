// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"strings"
	"testing"
	"time"
)

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

func TestRenderCommit_defaultTemplate(t *testing.T) {
	t.Parallel()

	cfg := Config{CommitMessage: defaultCommitMessage}.withDefaults()
	ctx := CommitContext{
		Namespace:  "apps",
		Name:       "web",
		Cluster:    "prod",
		ItemCount:  42,
		Checksum:   "abcdef0123456789",
		ExportedAt: time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC),
		Path:       "inventory/apps/web.json",
	}

	got := renderCommit(cfg, ctx)
	wantSubject := "chore(prod/apps/web): export 42 items @ abcdef012345"
	if got.Subject != wantSubject {
		t.Fatalf("subject = %q, want %q", got.Subject, wantSubject)
	}
}

func TestRenderCommit_bodyAndTrailers(t *testing.T) {
	t.Parallel()

	cfg := Config{
		CommitMessage:  "update {name}",
		CommitBody:     "items: {itemCount}\nchecksum: {checksumShort}",
		CommitTrailers: []string{"Kollect-Cluster: {cluster}"},
	}
	ctx := CommitContext{
		Namespace: "ns",
		Name:      "inv",
		Cluster:   "c1",
		ItemCount: 3,
		Checksum:  "deadbeefcafebabe",
	}

	got := renderCommit(cfg, ctx)
	if got.Subject != "update inv" {
		t.Fatalf("subject = %q", got.Subject)
	}
	if !strings.Contains(got.Body, "items: 3") {
		t.Fatalf("body = %q", got.Body)
	}
	if len(got.Trailers) != 1 || got.Trailers[0] != "Kollect-Cluster: c1" {
		t.Fatalf("trailers = %v", got.Trailers)
	}
}

func TestCommitContextFromExport(t *testing.T) {
	t.Parallel()

	envelope := []byte(`{
		"schemaVersion":"kollect.dev/v1alpha1",
		"checksum":"abc123",
		"generation":7,
		"itemCount":5,
		"exportedAt":"2026-06-07T10:00:00Z",
		"cluster":"spoke-a",
		"items":[]
	}`)

	ctx := CommitContextFromExport(envelope, "inventory/team-a/demo.json", "fallback", "git-sink")
	if ctx.Namespace != "team-a" || ctx.Name != "demo" {
		t.Fatalf("inventory = %s/%s", ctx.Namespace, ctx.Name)
	}
	if ctx.Cluster != "spoke-a" {
		t.Fatalf("cluster = %q", ctx.Cluster)
	}
	if ctx.Generation != 7 || ctx.ItemCount != 5 || ctx.Checksum != "abc123" {
		t.Fatalf("meta gen=%d count=%d checksum=%q", ctx.Generation, ctx.ItemCount, ctx.Checksum)
	}
	if ctx.SinkName != "git-sink" {
		t.Fatalf("sink = %q", ctx.SinkName)
	}
}
