// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestConfigFromSpec_defaults(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://example.com/inventory.git",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.PushPolicy != PushPolicyCommit || cfg.CloneDepth != 1 || cfg.AuthType != AuthTypeToken {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestConfigFromSpec_gitBlock(t *testing.T) {
	t.Parallel()

	depth := int32(3)
	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://example.com/inventory.git#branch=develop",
		Git: &kollectdevv1alpha1.GitSpec{
			Branch:     "main",
			PushPolicy: kollectdevv1alpha1.GitPushPolicyForcePush,
			CloneDepth: &depth,
			Prune:      true,
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.EffectiveBranch("develop") != "main" || cfg.PushPolicy != PushPolicyForcePush || !cfg.Prune {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestConfigFromSpec_wrongType(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "postgres",
		Endpoint: "https://example.com/inventory.git",
	}, nil)
	if err == nil {
		t.Fatal("expected error for non-git sink type")
	}
}

func TestConfigFromSpec_emptyEndpoint(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: TypeName}, nil)
	if err == nil {
		t.Fatal("expected error for empty endpoint")
	}
}

func TestConfigFromSpec_invalidEndpointURL(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://example.com/%zz",
	}, nil)
	if err == nil {
		t.Fatal("expected error for unparseable endpoint URL")
	}
}

func TestConfigFromSpec_invalidTLSSpec(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://example.com/inventory.git",
		TLS: &kollectdevv1alpha1.TLSSpec{
			CABundle:    []byte("pem"),
			CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca-secret"},
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error for ambiguous TLS spec (caBundle + caSecretRef)")
	}
}

func TestApplyGitSpec_invalidBranch(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://example.com/inventory.git",
		Git:      &kollectdevv1alpha1.GitSpec{Branch: "-bad"},
	}, nil)
	if err == nil {
		t.Fatal("expected error for invalid branch ref")
	}
}

func TestApplyGitSpec_invalidPushPolicy(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://example.com/inventory.git",
		Git:      &kollectdevv1alpha1.GitSpec{PushPolicy: "rebase-and-cry"},
	}, nil)
	if err == nil {
		t.Fatal("expected error for unsupported pushPolicy")
	}
}

func TestApplyGitSpec_commitAndAuthorOverrides(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://example.com/inventory.git",
		Git: &kollectdevv1alpha1.GitSpec{
			CommitMessage:  "custom message",
			CommitBody:     "custom body",
			CommitTrailers: []string{"Reviewed-by: bot"},
			Author:         &kollectdevv1alpha1.GitAuthorSpec{Name: "Bot", Email: "bot@example.com"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.CommitMessage != "custom message" || cfg.CommitBody != "custom body" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if len(cfg.CommitTrailers) != 1 || cfg.CommitTrailers[0] != "Reviewed-by: bot" {
		t.Fatalf("CommitTrailers = %v", cfg.CommitTrailers)
	}
	if cfg.Author.Name != "Bot" || cfg.Author.Email != "bot@example.com" {
		t.Fatalf("Author = %+v", cfg.Author)
	}
}

func TestApplyGitSpec_invalidCloneDepth(t *testing.T) {
	t.Parallel()

	depth := int32(0)
	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     TypeName,
		Endpoint: "https://example.com/inventory.git",
		Git:      &kollectdevv1alpha1.GitSpec{CloneDepth: &depth},
	}, nil)
	if err == nil {
		t.Fatal("expected error for cloneDepth < 1")
	}
}

// Engine-convergence (T09, GTE-1): backend construction accepts go-git (and an omitted engine)
// and rejects every other value naming go-git as the only engine — T04's
// TestConfigFromSpec_rejectsCLIEngineNamingGoGit pins the "cli" case; this covers arbitrary
// invalid values at the same construction path.
func TestApplyGitSpec_engineAcceptsGoGitAndRejectsOthers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		engine  string
		wantErr bool
	}{
		{name: "go-git", engine: kollectdevv1alpha1.GitEngineGoGit},
		{name: "cli", engine: "cli", wantErr: true},
		{name: "invalid", engine: "magic", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
				Type:     TypeName,
				Endpoint: "https://example.com/inventory.git",
				Git:      &kollectdevv1alpha1.GitSpec{Engine: tc.engine},
			}, nil)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ConfigFromSpec(engine=%q) error = nil, want a rejection naming go-git", tc.engine)
				}
				if !strings.Contains(err.Error(), "go-git") {
					t.Fatalf("ConfigFromSpec(engine=%q) error = %q, want it to name go-git as the engine", tc.engine, err)
				}

				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestResolveAuthType_explicit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		auth    *kollectdevv1alpha1.GitAuthSpec
		want    AuthType
		wantErr bool
	}{
		{name: "explicit token", auth: &kollectdevv1alpha1.GitAuthSpec{Type: kollectdevv1alpha1.GitAuthTypeToken}, want: AuthTypeToken},
		{name: "explicit ssh", auth: &kollectdevv1alpha1.GitAuthSpec{Type: kollectdevv1alpha1.GitAuthTypeSSH}, want: AuthTypeSSH},
		{name: "invalid", auth: &kollectdevv1alpha1.GitAuthSpec{Type: "carrier-pigeon"}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveAuthType("https://example.com/r.git", &kollectdevv1alpha1.GitSpec{Auth: tc.auth})
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error for invalid auth type")
				}

				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("resolveAuthType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveAuthType_inferredFromScheme(t *testing.T) {
	t.Parallel()

	got, err := resolveAuthType("ssh://git@example.com/r.git", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != AuthTypeSSH {
		t.Fatalf("resolveAuthType() = %q, want ssh", got)
	}

	got, err = resolveAuthType("https://example.com/r.git", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != AuthTypeToken {
		t.Fatalf("resolveAuthType() = %q, want token", got)
	}
}

func TestResolveAuthType_propagatesParseEndpointError(t *testing.T) {
	t.Parallel()

	_, err := resolveAuthType("https://example.com/%zz", nil)
	if err == nil {
		t.Fatal("expected error from parseEndpoint propagation")
	}
}

func TestParseEndpoint_resolvesCloneURLBranchAndScheme(t *testing.T) {
	t.Parallel()

	cloneURL, branch, scheme, err := parseEndpoint("https://example.com/repo.git#branch=develop")
	if err != nil {
		t.Fatal(err)
	}
	if cloneURL != "https://example.com/repo.git" {
		t.Fatalf("cloneURL = %q", cloneURL)
	}
	if branch != "develop" {
		t.Fatalf("branch = %q, want develop", branch)
	}
	if scheme != "https" {
		t.Fatalf("scheme = %q, want https", scheme)
	}
}

func TestParseEndpoint_invalidURL(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseEndpoint("https://example.com/%zz")
	if err == nil {
		t.Fatal("expected error for unparseable endpoint")
	}
}
