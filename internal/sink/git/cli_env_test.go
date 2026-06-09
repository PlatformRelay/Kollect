// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestNewCLIEnv_forceBasicAuthHeader(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Endpoint:       "https://example.com/r.git",
		ForceBasicAuth: true,
		Engine:         GitEngineCLI,
	}
	auth := Auth{Username: "user", Password: "pass"}

	cli, err := newCLIEnv(cfg, auth, AuthTypeToken)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.cleanup()

	if len(cli.configEnvArgs) != 2 {
		t.Fatalf("configEnvArgs = %v", cli.configEnvArgs)
	}

	found := false
	for _, e := range cli.extraEnv {
		if strings.HasPrefix(e, envAuthHeader+"=") {
			found = true
		}
	}
	if !found {
		t.Fatalf("extraEnv = %v", cli.extraEnv)
	}
}

func TestForceBasicAuthFromEnv(t *testing.T) {
	t.Setenv(envForceBasicAuth, "true")
	if !forceBasicAuthFromEnv() {
		t.Fatal("expected true")
	}
}

func TestConfigFromSpec_forceBasicAuthEnv(t *testing.T) {
	t.Setenv(envForceBasicAuth, "1")

	cfg, err := ConfigFromSpec(minimalGitSpec("https://example.com/r.git"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.ForceBasicAuth {
		t.Fatal("expected ForceBasicAuth from env")
	}
}

func minimalGitSpec(endpoint string) kollectdevv1alpha1.KollectSinkSpec {
	return kollectdevv1alpha1.KollectSinkSpec{Type: TypeName, Endpoint: endpoint}
}

func TestCfgNeedsCLISSH(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cfg      Config
		authType AuthType
		want     bool
	}{
		{
			name: "non-cli engine",
			cfg: Config{
				Endpoint: "ssh://git@example.com/repo.git",
				Engine:   GitEngineGoGit,
			},
			authType: AuthTypeToken,
			want:     false,
		},
		{
			name: "cli ssh endpoint",
			cfg: Config{
				Endpoint: "ssh://git@example.com/repo.git",
				Engine:   GitEngineCLI,
			},
			authType: AuthTypeToken,
			want:     true,
		},
		{
			name: "cli with ssh auth",
			cfg: Config{
				Endpoint: "https://example.com/repo.git",
				Engine:   GitEngineCLI,
			},
			authType: AuthTypeSSH,
			want:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := cfgNeedsCLISSH(tc.cfg, tc.authType); got != tc.want {
				t.Fatalf("cfgNeedsCLISSH() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplyCLIEnvAndPrependGitArgs(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("git", "status") //nolint:gosec // G204: test command wiring only
	cli := &cliEnv{
		extraEnv:      []string{"KOLLECT_TEST_FLAG=1"},
		configEnvArgs: []string{"--config-env", "http.extraHeader=KOLLECT_GIT_AUTH_HEADER"},
	}

	applyCLIEnv(cmd, cli)
	if len(cmd.Env) == 0 {
		t.Fatal("expected command env to be populated")
	}
	found := false
	for _, e := range cmd.Env {
		if e == "KOLLECT_TEST_FLAG=1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("env = %v, missing injected var", cmd.Env)
	}

	args := cli.prependGitArgs("push", "origin", "main")
	if len(args) < 4 || args[0] != "--config-env" {
		t.Fatalf("prependGitArgs() = %v", args)
	}
}

func TestBuildGitSSHCommand_InsecureWithoutKey(t *testing.T) {
	t.Parallel()

	cmd, cleanup, err := buildGitSSHCommand(Auth{}, SSHConfig{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("buildGitSSHCommand() error = %v", err)
	}
	if cmd != "ssh -o StrictHostKeyChecking=no" {
		t.Fatalf("command = %q", cmd)
	}
	if cleanup != nil {
		t.Fatal("cleanup should be nil when no temp files are created")
	}
}

func TestBuildGitSSHCommand_WithKeyAndKnownHostsCleansUpFiles(t *testing.T) {
	t.Parallel()

	privateKey := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n-----END OPENSSH PRIVATE KEY-----")
	knownHosts := []byte("github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey")

	cmd, cleanup, err := buildGitSSHCommand(Auth{SSHPrivateKey: privateKey}, SSHConfig{KnownHosts: knownHosts})
	if err != nil {
		t.Fatalf("buildGitSSHCommand() error = %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup must be set when temp files are created")
	}
	if !strings.Contains(cmd, "IdentitiesOnly=yes") || !strings.Contains(cmd, "UserKnownHostsFile=") {
		t.Fatalf("command = %q", cmd)
	}

	parts := strings.Split(cmd, " ")
	var keyPath, knownHostsPath string
	for i := 0; i < len(parts); i++ {
		if parts[i] == "-i" && i+1 < len(parts) {
			keyPath = parts[i+1]
		}
		if strings.HasPrefix(parts[i], "UserKnownHostsFile=") {
			knownHostsPath = strings.TrimPrefix(parts[i], "UserKnownHostsFile=")
		}
	}
	if keyPath == "" || knownHostsPath == "" {
		t.Fatalf("could not parse temp paths from command %q", cmd)
	}
	if _, err := os.Stat(filepath.Clean(keyPath)); err != nil {
		t.Fatalf("key file missing before cleanup: %v", err)
	}
	if _, err := os.Stat(filepath.Clean(knownHostsPath)); err != nil {
		t.Fatalf("known_hosts missing before cleanup: %v", err)
	}

	cleanup()

	if _, err := os.Stat(filepath.Clean(keyPath)); !os.IsNotExist(err) {
		t.Fatalf("key file should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Clean(knownHostsPath)); !os.IsNotExist(err) {
		t.Fatalf("known_hosts file should be removed, stat err=%v", err)
	}
}
