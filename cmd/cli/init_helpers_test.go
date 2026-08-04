// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/platformrelay/kollect/internal/pipeline"
)

func writeInitKubeconfig(t *testing.T) string {
	t.Helper()

	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["dev"] = &clientcmdapi.Cluster{Server: "https://example.invalid:6443"}
	cfg.AuthInfos["dev"] = clientcmdapi.NewAuthInfo()
	cfg.Contexts["dev"] = &clientcmdapi.Context{Cluster: "dev", AuthInfo: "dev"}
	cfg.CurrentContext = "dev"

	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := clientcmd.WriteToFile(*cfg, path); err != nil {
		t.Fatalf("write init kubeconfig: %v", err)
	}
	return path
}

func mustLoadConfig(t *testing.T, dir string) (pipeline.LoadResult, error) {
	t.Helper()
	return pipeline.LoadConfig(dir)
}

func listYAML(t *testing.T, dir string) ([]string, error) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func writeMarker(t *testing.T, dir, name, content string) error {
	t.Helper()
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // test helper
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}
