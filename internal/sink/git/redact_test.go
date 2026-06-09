// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"net"
	"os/exec"
	"strings"
	"testing"
)

func TestRedactCredentials_userTokenURL(t *testing.T) {
	t.Parallel()

	in := "fatal: unable to access 'https://x-access-token:fake-sekret-123@git.example.com/org/repo.git/': " +
		"The requested URL returned error: 403"
	got := redactCredentials(in, "fake-sekret-123")

	if strings.Contains(got, "fake-sekret-123") {
		t.Fatalf("secret leaked: %q", got)
	}

	if strings.Contains(got, "x-access-token:") {
		t.Fatalf("userinfo leaked: %q", got)
	}

	if !strings.Contains(got, "git.example.com/org/repo.git") {
		t.Fatalf("host dropped, error no longer actionable: %q", got)
	}

	if !strings.Contains(got, "403") {
		t.Fatalf("failure reason dropped: %q", got)
	}

	if !strings.Contains(got, "https://***@git.example.com") {
		t.Fatalf("expected masked userinfo: %q", got)
	}
}

func TestRedactCredentials_tokenOnlyURL(t *testing.T) {
	t.Parallel()

	got := redactCredentials("error: https://fake-token-456@git.example.com/repo.git rejected")
	if strings.Contains(got, "fake-token-456") {
		t.Fatalf("token leaked: %q", got)
	}

	if !strings.Contains(got, "https://***@git.example.com/repo.git") {
		t.Fatalf("expected masked URL: %q", got)
	}
}

func TestRedactCredentials_multipleOccurrencesMultiline(t *testing.T) {
	t.Parallel()

	in := "remote: error for https://bot:fake-pw-1@git.example.com/a.git\n" +
		"fatal: unable to access 'http://bot:fake-pw-1@mirror.example.org/b.git/': failed\n" +
		"hint: check HTTPS://bot:fake-pw-1@git.example.com/a.git"
	got := redactCredentials(in, "fake-pw-1")

	if strings.Contains(got, "fake-pw-1") || strings.Contains(got, "bot:") {
		t.Fatalf("credentials leaked: %q", got)
	}

	if strings.Count(got, "***@") != 3 {
		t.Fatalf("expected 3 masked URLs, got: %q", got)
	}

	if !strings.Contains(got, "mirror.example.org/b.git") {
		t.Fatalf("host dropped: %q", got)
	}
}

func TestRedactCredentials_passthrough(t *testing.T) {
	t.Parallel()

	for _, in := range []string{
		"fatal: Could not read from remote repository ssh://git@git.example.com/org/repo.git",
		"fatal: unable to access 'https://git.example.com/org/repo.git/': could not resolve host",
		"fatal: repository 'file:///tmp/repo.git' does not exist",
		"error: object path inventory/a@b.json invalid",
	} {
		if got := redactCredentials(in); got != in {
			t.Fatalf("expected passthrough for %q, got %q", in, got)
		}
	}
}

func TestRedactCredentials_secretValuesOutsideURLs(t *testing.T) {
	t.Parallel()

	got := redactCredentials("trace: header authorization: basic fake-sekret-789", "", "fake-sekret-789")
	if strings.Contains(got, "fake-sekret-789") {
		t.Fatalf("secret value leaked: %q", got)
	}

	if !strings.Contains(got, "***") {
		t.Fatalf("expected placeholder: %q", got)
	}
}

func TestRedactionSecrets_skipsEmpty(t *testing.T) {
	t.Parallel()

	if got := redactionSecrets(Auth{}); len(got) != 0 {
		t.Fatalf("expected no secrets, got %v", got)
	}

	got := redactionSecrets(Auth{Token: " fake-tok-1 ", Password: "fake-pw-2"}) //nolint:gosec // G101: fake test credential
	for _, want := range []string{" fake-tok-1 ", "fake-tok-1", "fake-pw-2"} {
		found := false
		for _, s := range got {
			if s == want {
				found = true
			}
		}

		if !found {
			t.Fatalf("missing secret %q in %v", want, got)
		}
	}
}

func TestCLIEnvRedact_nilSafe(t *testing.T) {
	t.Parallel()

	var cli *cliEnv
	got := cli.redact("https://u:fake-pw-3@git.example.com/r.git failed")
	if strings.Contains(got, "fake-pw-3") {
		t.Fatalf("nil cliEnv leaked credentials: %q", got)
	}
}

func TestRunGitOutput_redactsCredentials(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	cmd := exec.Command("sh", "-c",
		`echo "fatal: unable to access 'https://x-access-token:fake-sekret-123@git.example.com/org/repo.git/': 403" >&2; exit 1`)
	cli := &cliEnv{secrets: redactionSecrets(Auth{Token: "fake-sekret-123"})} //nolint:gosec // G101: fake test credential

	err := runGitOutput(cmd, "push", cli)
	if err == nil {
		t.Fatal("expected error")
	}

	msg := err.Error()
	if strings.Contains(msg, "fake-sekret-123") || strings.Contains(msg, "x-access-token:") {
		t.Fatalf("credentials leaked into error: %q", msg)
	}

	if !strings.Contains(msg, "git.example.com/org/repo.git") || !strings.Contains(msg, "403") {
		t.Fatalf("error lost actionable context: %q", msg)
	}
}

// TestLsRemoteUncached_redactsEmbeddedCredentials exercises the real
// `git ls-remote` failure path with credentials embedded via embedInURL,
// asserting the wrapped error never carries the token (EC-P1-02).
func TestLsRemoteUncached_redactsEmbeddedCredentials(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	cfg := Config{Endpoint: "https://" + ln.Addr().String() + "/org/repo.git"}
	auth := Auth{Username: "x-access-token", Token: "fake-sekret-123"} //nolint:gosec // G101: fake test credential

	lsErr := lsRemoteUncached(context.Background(), cfg.withDefaults(), auth)
	if lsErr == nil {
		t.Fatal("expected ls-remote failure against closing listener")
	}

	msg := lsErr.Error()
	if strings.Contains(msg, "fake-sekret-123") {
		t.Fatalf("token leaked into error: %q", msg)
	}

	if strings.Contains(msg, "x-access-token:") {
		t.Fatalf("userinfo leaked into error: %q", msg)
	}
}
