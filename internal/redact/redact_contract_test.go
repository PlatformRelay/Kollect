// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package redact

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// The shared redaction contract (B5 test lock): pinned behaviour for every
// consumer (controller writers, nats connect, future sinks).
func TestText_contract(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		secrets []string
		want    string
	}{
		{
			name: "https userinfo masked, host kept",
			in:   `dial failed for https://user:s3cret@github.com/org/repo`,
			want: `dial failed for https://***@github.com/org/repo`,
		},
		{
			name: "token-only userinfo masked",
			in:   `push rejected for https://ghp_T0KEN@github.com/org/repo`,
			want: `push rejected for https://***@github.com/org/repo`,
		},
		{ //nolint:gosec // G101: fake credential fixture for the redaction contract
			name: "nats scheme userinfo masked",
			in:   `nats connect: parse nats://user:tok@badhost:4222 invalid`,
			want: `nats connect: parse nats://***@badhost:4222 invalid`,
		},
		{ //nolint:gosec // G101: fake credential fixture for the redaction contract
			name: "mongodb scheme userinfo masked",
			in:   `auth failed mongodb://admin:p4ss@mongo:27017/test`,
			want: `auth failed mongodb://***@mongo:27017/test`,
		},
		{
			name: "ssh userinfo masked on status surface",
			in:   `git ls-remote ssh://git@git.example.com/org/repo.git`,
			want: `git ls-remote ssh://***@git.example.com/org/repo.git`,
		},
		{
			name: "special-character password masked",
			in:   `open nats://user:pa(ss)w0rd[].x@host:4222: auth error`,
			want: `open nats://***@host:4222: auth error`,
		},
		{
			name: "multiple occurrences across lines",
			in: "line1 https://a:1@h1/x\n" +
				"line2 nats://b:2@h2\n" +
				"line3 https://c:3@h3/y",
			want: "line1 https://***@h1/x\n" +
				"line2 nats://***@h2\n" +
				"line3 https://***@h3/y",
		},
		{
			name:    "secret values replaced verbatim",
			in:      `auth rejected for user abc123token and again abc123token`,
			secrets: []string{"abc123token"},
			want:    `auth rejected for user *** and again ***`,
		},
		{
			name:    "empty secret skipped, short no-op secret skipped only when empty",
			in:      `plain text`,
			secrets: []string{""},
			want:    `plain text`,
		},
		{
			name: "no credentials passthrough byte-identical",
			in:   `rpc error: code = Unavailable desc = connection refused host=10.0.0.1:4222`,
			want: `rpc error: code = Unavailable desc = connection refused host=10.0.0.1:4222`,
		},
		{
			name: "empty string",
			in:   ``,
			want: ``,
		},
		{
			name: "bare at-sign prose untouched (no scheme)",
			in:   `user contact me@example.com for help`,
			want: `user contact me@example.com for help`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Text(tc.in, tc.secrets...)
			if got != tc.want {
				t.Fatalf("Text() =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

func TestText_noCredentialSurvives(t *testing.T) {
	in := `x https://user:pass@h1/a nats://u:p@h2 mongodb://u:p@h3 gopher://u:p@h4` //nolint:gosec // G101: fake credential fixture
	got := Text(in)

	for _, leak := range []string{"user:pass", "u:p", "user@", "u@"} {
		if strings.Contains(got, leak) {
			t.Fatalf("redacted text still contains %q: %q", leak, got)
		}
	}
}

func TestError_nilPassthrough(t *testing.T) {
	if Error(nil) != nil {
		t.Fatal("Error(nil) must be nil")
	}
}

func TestError_redactsMessageKeepsIdentity(t *testing.T) {
	sentinel := errors.New("boom")
	inner := fmt.Errorf("dial https://user:s3cret@nats:4222: %w", sentinel)
	classified := kollecterrors.Terminal(inner)

	got := Error(classified)

	if strings.Contains(got.Error(), "s3cret") || strings.Contains(got.Error(), "user:") {
		t.Fatalf("message leaked credentials: %q", got.Error())
	}

	if !kollecterrors.IsTerminal(got) {
		t.Error("terminal classification lost through redaction")
	}

	if !errors.Is(got, sentinel) {
		t.Error("errors.Is identity lost through redaction")
	}

	if kollecterrors.ClassOf(got) != kollecterrors.ClassTerminal {
		t.Errorf("ClassOf = %q, want terminal", kollecterrors.ClassOf(got))
	}
}

func TestError_keepsAPIErrorClassification(t *testing.T) {
	nf := apierrors.NewNotFound(schema.GroupResource{Group: "kollect.dev", Resource: "kollectinventories"}, "x")

	got := Error(nf)
	if !apierrors.IsNotFound(got) {
		t.Error("apierrors.IsNotFound lost through redaction")
	}
}
