// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/platformrelay/kollect/internal/version"
)

func TestRootCmd_versionFlagPrintsVersionString(t *testing.T) {
	t.Parallel()

	root, _ := buildRoot()

	var out bytes.Buffer

	root.SetOut(&out)
	root.SetArgs([]string{"--version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(--version) error = %v", err)
	}

	if got := out.String(); !strings.Contains(got, version.String()) {
		t.Errorf("--version output = %q, want it to contain %q", got, version.String())
	}
}

func TestNewRootCmd_setsVersion(t *testing.T) {
	t.Parallel()

	root := newRootCmd()

	if root.Version != version.String() {
		t.Errorf("root.Version = %q, want %q", root.Version, version.String())
	}
}
