//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/export"
)

func TestRunExportEnvelope_GitAutoInfersResourceMode(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	DisableBackendPoolForTest()
	t.Cleanup(func() {
		EnableBackendPoolForTest()
		ResetBackendPoolForTest()
	})

	work := t.TempDir()
	remote := filepath.Join(work, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %s: %v", out, err)
	}

	manifest := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"namespace": "team-a"},
	}
	items := []collect.Item{
		{
			Namespace: "team-a", Name: "api", Kind: "Deployment", Version: "v1", UID: "uid-1",
			Attributes: map[string]any{
				"payload": manifest,
				"image":   "nginx",
			},
		},
		{
			Namespace: "team-a", Name: "web", Kind: "Deployment", Version: "v1", UID: "uid-2",
			Attributes: map[string]any{
				"payload": manifest,
				"image":   "nginx",
			},
		},
	}

	envelope, err := export.MarshalEnvelope(items, export.Metadata{Generation: 1, ExportedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}

	err = RunExportEnvelope(ExportEnvelopeRequest{
		Ctx:           t.Context(),
		Registry:      NewRegistry(),
		SinkNamespace: "default",
		SinkName:      "resource-git",
		ObjectPath:    "inventory/team-a/apps.json",
		Envelope:      envelope,
		SinkSpec: kollectdevv1alpha1.KollectSinkSpec{
			Type:     kollectdevv1alpha1.SinkTypeGit,
			Endpoint: "file://" + remote,
		},
	})
	if err != nil {
		t.Fatalf("RunExportEnvelope: %v", err)
	}

	clone := filepath.Join(work, "clone")
	if out, err := exec.Command("git", "clone", "--branch", "main", "--single-branch", "file://"+remote, clone).CombinedOutput(); err != nil { //nolint:gosec // test fixture
		t.Fatalf("clone: %s: %v", out, err)
	}

	for _, rel := range []string{
		"default/team-a/deployment/api.yaml",
		"default/team-a/deployment/web.yaml",
	} {
		data, err := os.ReadFile(filepath.Join(clone, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		content := string(data)
		if !strings.Contains(content, "apiVersion: apps/v1") || !strings.Contains(content, "kind: Deployment") {
			t.Fatalf("%s missing manifest yaml:\n%s", rel, content)
		}
		if strings.Contains(content, "attributes:") {
			t.Fatalf("%s should not contain item envelope:\n%s", rel, content)
		}
	}
}
