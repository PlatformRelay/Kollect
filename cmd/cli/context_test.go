// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func writeFixtureKubeconfig(t *testing.T) string {
	t.Helper()

	cfg := clientcmdapi.NewConfig()
	for _, name := range []string{"prod-eu-1", "prod-us-1", "staging-canary", "dev"} {
		cfg.Contexts[name] = clientcmdapi.NewContext()
		cfg.Clusters[name] = clientcmdapi.NewCluster()
		cfg.AuthInfos[name] = clientcmdapi.NewAuthInfo()
		cfg.Contexts[name].Cluster = name
		cfg.Contexts[name].AuthInfo = name
	}
	cfg.CurrentContext = "dev"

	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := clientcmd.WriteToFile(*cfg, path); err != nil {
		t.Fatalf("write fixture kubeconfig: %v", err)
	}

	return path
}

func TestResolveContexts_noPatternsReturnsCurrentContext(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	got, warnings, err := resolveContexts(path, nil)
	if err != nil {
		t.Fatalf("resolveContexts() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if !reflect.DeepEqual(got, []string{"dev"}) {
		t.Errorf("got %v, want [dev]", got)
	}
}

func TestResolveContexts_explicitListUnion(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	got, _, err := resolveContexts(path, []string{"prod-eu-1", "dev"})
	if err != nil {
		t.Fatalf("resolveContexts() error = %v", err)
	}
	want := []string{"dev", "prod-eu-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveContexts_wildcardMatchesSubset(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	got, _, err := resolveContexts(path, []string{"prod-*"})
	if err != nil {
		t.Fatalf("resolveContexts() error = %v", err)
	}
	want := []string{"prod-eu-1", "prod-us-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveContexts_mixedLiteralAndWildcardDedup(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	got, _, err := resolveContexts(path, []string{"prod-*", "prod-eu-1"})
	if err != nil {
		t.Fatalf("resolveContexts() error = %v", err)
	}
	want := []string{"prod-eu-1", "prod-us-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v (no duplicate prod-eu-1)", got, want)
	}
}

func TestResolveContexts_literalNoMatchIsFatal(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	_, _, err := resolveContexts(path, []string{"typo-ctx"})
	if err == nil {
		t.Fatal("expected error for literal pattern with no match, got nil")
	}
}

func TestResolveContexts_wildcardNoMatchIsWarningNotFatalWhenOtherPatternsMatch(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	got, warnings, err := resolveContexts(path, []string{"prod-*", "removed-*"})
	if err != nil {
		t.Fatalf("resolveContexts() error = %v", err)
	}
	want := []string{"prod-eu-1", "prod-us-1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(warnings) == 0 {
		t.Error("expected a warning for the non-matching wildcard pattern")
	}
}

func TestResolveContexts_allWildcardsNoMatchIsFatal(t *testing.T) {
	t.Parallel()

	path := writeFixtureKubeconfig(t)

	_, _, err := resolveContexts(path, []string{"removed-*"})
	if err == nil {
		t.Fatal("expected error when the final resolved set is empty, got nil")
	}
}
