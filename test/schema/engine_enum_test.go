// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package schema

import (
	"os"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// Engine-convergence red (T04, GTE-1): the generated CRD schema enum for spec.git.engine must
// list only go-git. Red until T09 removes the cli enum value and regenerates both committed
// copies (config/crd/bases is the controller-gen output; charts/kollect/crds is the chart's
// shipped copy — neither may drift from the other).
func TestKollectSnapshotSinkGitEngineEnumIsGoGitOnly(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)

	paths := map[string]string{
		"config/crd/bases":    CRDPath(root, "kollect.dev_kollectsnapshotsinks.yaml"),
		"charts/kollect/crds": root + "/charts/kollect/crds/kollect.dev_kollectsnapshotsinks.yaml",
	}

	for source, path := range paths {
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			engineEnums := gitEngineEnums(t, path)
			if len(engineEnums) == 0 {
				t.Fatal("no CRD version declares spec.git.engine (want the field present with enum [go-git])")
			}

			for version, enum := range engineEnums {
				if len(enum) != 1 || enum[0] != "go-git" {
					t.Fatalf("CRD version %s: spec.git.engine enum = %v, want exactly [go-git]", version, enum)
				}
			}
		})
	}
}

// gitEngineEnums returns the spec.git.engine enum per CRD version that declares it.
func gitEngineEnums(t *testing.T, path string) map[string][]string {
	t.Helper()

	raw, err := os.ReadFile(path) //nolint:gosec // G304: path is a committed manifest in this repository.
	if err != nil {
		t.Fatalf("read crd: %v", err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if unmarshalErr := yaml.Unmarshal(raw, &crd); unmarshalErr != nil {
		t.Fatalf("parse crd: %v", unmarshalErr)
	}

	enums := map[string][]string{}
	for i := range crd.Spec.Versions {
		version := &crd.Spec.Versions[i]
		schema := version.Schema
		if schema == nil || schema.OpenAPIV3Schema == nil {
			continue
		}

		engine, ok := schema.OpenAPIV3Schema.Properties["spec"].Properties["git"].Properties["engine"]
		if !ok {
			continue
		}

		values := make([]string, 0, len(engine.Enum))
		for _, value := range engine.Enum {
			var decoded string
			if decodeErr := yaml.Unmarshal(value.Raw, &decoded); decodeErr != nil {
				t.Fatalf("CRD version %s: decode spec.git.engine enum value: %v", version.Name, decodeErr)
			}
			values = append(values, decoded)
		}

		enums[version.Name] = values
	}

	return enums
}
