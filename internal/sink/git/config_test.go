// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
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
