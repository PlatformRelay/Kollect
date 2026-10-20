// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hermeticGitConfig is the global git configuration every git process started by this package's
// tests reads (fixture commands, the CLI engine, and the git-upload-pack/git-receive-pack binaries
// go-git's file transport executes).
//
// It turns off automatic maintenance. By default every push into a fixture remote makes
// git-receive-pack run `git maintenance run --auto --detach`, and every client-side fetch or push
// does the same in its own repository. The detached child outlives the git command that started it
// and keeps working in the fixture repository after the test has returned, so t.TempDir's
// RemoveAll can race with it and fail with "unlinkat .../remote.git: directory not empty"
// (FLAKE-SINKGIT-TEMPDIR). What the child does depends on the git version and its maintenance
// strategy; no test here depends on it, so none is started. maintenance.auto is the switch current
// git checks before it starts any auto maintenance (TestHermeticGitConfig_noAutoMaintenance fails
// without it); the gc and receive keys cover older git, whose receive-pack ran `gc --auto` itself.
//
// The file replaces the developer's ~/.gitconfig as well, so local hooks, aliases or a different
// init.defaultBranch cannot change what these tests observe.
const hermeticGitConfig = `[maintenance]
	auto = false
	autoDetach = false
[gc]
	auto = 0
	autoDetach = false
[receive]
	autoGC = false
`

func TestMain(m *testing.M) {
	os.Exit(runWithHermeticGitConfig(m))
}

func runWithHermeticGitConfig(m *testing.M) int {
	dir, err := os.MkdirTemp("", "kollect-sink-git-config-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "hermetic git config: %v\n", err)

		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(path, []byte(hermeticGitConfig), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "hermetic git config: %v\n", err)

		return 1
	}
	if err := os.Setenv("GIT_CONFIG_GLOBAL", path); err != nil {
		fmt.Fprintf(os.Stderr, "hermetic git config: %v\n", err)

		return 1
	}

	return m.Run()
}

// TestHermeticGitConfig_noAutoMaintenance pins the reason for TestMain: a commit and a push into a
// fixture remote, the operations that start background maintenance by default, start no
// `git maintenance` or `git gc` child. GIT_TRACE2_EVENT is inherited by every git process of the
// fixture, git-receive-pack included, so the trace sees the remote side too.
func TestHermeticGitConfig_noAutoMaintenance(t *testing.T) {
	skipWithoutGit(t)
	trace := filepath.Join(t.TempDir(), "trace2.json")
	t.Setenv("GIT_TRACE2_EVENT", trace)

	createBareRemoteWithMainCommit(t)

	data, err := os.ReadFile(trace) //nolint:gosec // G304: trace file under t.TempDir
	if err != nil {
		t.Fatalf("read trace2 events: %v", err)
	}
	if !strings.Contains(string(data), `"receive-pack"`) {
		t.Fatalf("trace2 events show no receive-pack run; the check saw nothing:\n%s", data)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, `"event":"child_start"`) {
			continue
		}
		if strings.Contains(line, `"maintenance"`) || strings.Contains(line, `"gc"`) {
			t.Errorf("fixture git started background maintenance: %s", line)
		}
	}
}
