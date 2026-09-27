//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitHTTPServer serves a project root as the GIT_PROJECT_ROOT of a git
// http-backend CGI behind a loopback httptest server. Only the integration
// build may dial loopback (netguard/integration_policy.go), which is exactly
// what these tests need: non-file:// remotes are the only topology with a
// persistent warm mirror (mirrorDirFor), and every mirror defect the review
// round reproduced lives there.
//
// The server grants authenticated push unconditionally (REMOTE_USER is always
// set and http.receivepack is enabled on each served repo), so both the git
// CLI engine and the go-git engine can clone/fetch/push anonymously.
type gitHTTPServer struct {
	*httptest.Server
	projectRoot string
}

func startGitHTTPServer(t *testing.T, projectRoot string) *gitHTTPServer {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	gitPath, err := resolveGitExecutable()
	if err != nil {
		t.Skipf("git executable not resolvable: %v", err)
	}

	wrapper := filepath.Join(t.TempDir(), "git-http-backend")
	script := "#!/bin/sh\nexec '" + gitPath + "' http-backend\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o750); err != nil { //nolint:gosec // G302: test fixture wrapper
		t.Fatalf("write cgi wrapper: %v", err)
	}

	handler := &cgi.Handler{
		Path: wrapper,
		Env: []string{
			"GIT_PROJECT_ROOT=" + projectRoot,
			"GIT_HTTP_EXPORT_ALL=1",
			"REMOTE_USER=kollect-integration",
		},
	}

	srv := &gitHTTPServer{Server: httptest.NewServer(handler), projectRoot: projectRoot}
	t.Cleanup(srv.Close)

	return srv
}

// repoURL returns the clone URL of the named bare repository under the
// server's project root.
func (s *gitHTTPServer) repoURL(name string) string {
	return s.URL + "/" + name
}

// seedBareRepo creates a bare repository under the server root with one seed
// commit on main and returns its clone URL.
func seedBareRepo(t *testing.T, srv *gitHTTPServer, name string) string {
	t.Helper()

	remote := filepath.Join(srv.projectRoot, name)
	if out, err := exec.Command("git", "init", "--bare", "-b", "main", remote).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("init bare %s: %s: %v", name, out, err)
	}
	if out, err := exec.Command("git", "--git-dir", remote, "config", "http.receivepack", "true").CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("enable receive-pack: %s: %v", out, err)
	}

	seed := filepath.Join(t.TempDir(), "seed")
	if out, err := exec.Command("git", "clone", srv.repoURL(name), seed).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("clone empty remote: %s: %v", out, err)
	}

	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", seed}, args...)...).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	run("add", "-A")
	run("-c", "user.name=kollect-integration", "-c", "user.email=kollect-integration@example.com", "commit", "-m", "seed")
	run("push", "-u", "origin", "main")

	return srv.repoURL(name)
}

// mirrorIsolate pins the git mirror root to a per-test directory: persistent
// warm mirrors are keyed by clone URL under this root, and tests must not see
// each other's (or a previous run's) mirrors.
func mirrorIsolate(t *testing.T) {
	t.Helper()

	t.Setenv("KOLLECT_GIT_MIRROR_DIR", filepath.Join(t.TempDir(), "mirrors"))
}

// remoteBranchSHA returns the remote tip of refs/heads/<branch> via one
// ls-remote, or "" when the branch does not exist.
func remoteBranchSHA(t *testing.T, url, branch string) string {
	t.Helper()

	out, err := exec.Command("git", "ls-remote", url, "refs/heads/"+branch).CombinedOutput() //nolint:gosec // G204: test fixture
	if err != nil {
		t.Fatalf("ls-remote %s: %s: %v", branch, out, err)
	}

	return remoteSHAFromLsRemote(string(out))
}

// remoteFileExists clones the branch shallowly and reports whether the file
// exists in its tree.
func remoteFileExists(t *testing.T, url, branch, path string) bool {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "verify")
	if out, err := exec.Command("git", "clone", "--branch", branch, "--single-branch", "--depth", "5", url, dir).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("clone %s: %s: %v", branch, out, err)
	}

	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path)))

	return err == nil
}

// remoteLogSubjects returns the commit subjects of a branch's recent history.
func remoteLogSubjects(t *testing.T, url, branch string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "log")
	if out, err := exec.Command("git", "clone", "--branch", branch, "--single-branch", "--depth", "10", url, dir).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture
		t.Fatalf("clone %s: %s: %v", branch, out, err)
	}

	out, err := exec.Command("git", "-C", dir, "log", "--format=%s").CombinedOutput() //nolint:gosec // G204: test fixture inspects its own temp repo
	if err != nil {
		t.Fatalf("log: %v", err)
	}

	return strings.TrimSpace(string(out))
}

// remoteParentSHA returns the parent commit of a branch's remote tip via a
// shallow clone, or "" when the tip has no parent.
func remoteParentSHA(t *testing.T, url, branch string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "parent")
	if out, err := exec.Command("git", "clone", "--branch", branch, "--single-branch", "--depth", "2", url, dir).CombinedOutput(); err != nil { //nolint:gosec // G204: test fixture clone
		t.Fatalf("clone %s: %s: %v", branch, out, err)
	}

	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD^").CombinedOutput() //nolint:gosec // G204: test fixture inspects its own temp repo
	if err != nil {
		t.Fatalf("rev-parse HEAD^: %s: %v", out, err)
	}

	return strings.TrimSpace(string(out))
}
