// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
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
