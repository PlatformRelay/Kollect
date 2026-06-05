// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package samples_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/validation"
)

func TestSampleProfilesValidate(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "config", "samples")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read samples dir: %v", err)
	}

	s := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add scheme: %v", err)
	}

	for _, ent := range entries {
		if ent.IsDir() || !strings.HasPrefix(ent.Name(), "kollect_v1alpha1_kollectprofile") {
			continue
		}

		path := filepath.Join(root, ent.Name())
		//nolint:gosec // G304: path is under repo config/samples only
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		var profile kollectdevv1alpha1.KollectProfile
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)
		if err := decoder.Decode(&profile); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}

		if profile.Kind != "KollectProfile" {
			t.Fatalf("%s: expected KollectProfile, got %s", path, profile.Kind)
		}

		if errs := validation.ValidateProfile(&profile); len(errs) > 0 {
			t.Fatalf("%s: validation failed: %v", path, errs)
		}
	}
}

func TestSampleKindsDecode(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "config", "samples")
	patterns := []string{
		"kollect_v1alpha1_kollectsink.yaml",
		"kollect_v1alpha1_kollectsink_postgres.yaml",
		"kollect_v1alpha1_kollectsink_kafka.yaml",
		"kollect_v1alpha1_kollecttarget.yaml",
		"kollect_v1alpha1_kollectinventory.yaml",
	}

	for _, name := range patterns {
		//nolint:gosec // G304: path is under repo config/samples only
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)
		var raw map[string]any
		if err := decoder.Decode(&raw); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}

		kind, _ := raw["kind"].(string)
		if kind == "" {
			t.Fatalf("%s: missing kind", name)
		}
	}
}
