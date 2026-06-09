// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package schema

import (
	"fmt"
	"os"
	"path/filepath"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// SpecFragmentCase maps a committed CRD manifest to a golden OpenAPI fragment file.
type SpecFragmentCase struct {
	CRDFile    string
	GoldenFile string
}

// DefaultCases lists CRD kinds with checked-in spec OpenAPI goldens.
var DefaultCases = []SpecFragmentCase{
	{CRDFile: "kollect.dev_kollectprofiles.yaml", GoldenFile: "kollectprofile.spec.openapi.yaml"},
	{CRDFile: "kollect.dev_kollectclusterprofiles.yaml", GoldenFile: "kollectclusterprofile.spec.openapi.yaml"},
	{CRDFile: "kollect.dev_kollecttargets.yaml", GoldenFile: "kollecttarget.spec.openapi.yaml"},
	{CRDFile: "kollect.dev_kollectinventories.yaml", GoldenFile: "kollectinventory.spec.openapi.yaml"},
	{CRDFile: "kollect.dev_kollectclustertargets.yaml", GoldenFile: "kollectclustertarget.spec.openapi.yaml"},
	{CRDFile: "kollect.dev_kollectclusterinventories.yaml", GoldenFile: "kollectclusterinventory.spec.openapi.yaml"},
	{CRDFile: "kollect.dev_kollectdatabasesinks.yaml", GoldenFile: "kollectdatabasesink.spec.openapi.yaml"},
	{CRDFile: "kollect.dev_kollecteventsinks.yaml", GoldenFile: "kollecteventsink.spec.openapi.yaml"},
}

// ExtractSpecOpenAPIFragment returns the storage-version .spec OpenAPI subtree from a CRD YAML file.
func ExtractSpecOpenAPIFragment(crdPath string) ([]byte, error) {
	//nolint:gosec // G304: crdPath is a committed manifest under config/crd/bases.
	raw, err := os.ReadFile(crdPath)
	if err != nil {
		return nil, fmt.Errorf("read crd %q: %w", crdPath, err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if unmarshalErr := yaml.Unmarshal(raw, &crd); unmarshalErr != nil {
		return nil, fmt.Errorf("parse crd %q: %w", crdPath, unmarshalErr)
	}

	var storage *apiextensionsv1.CustomResourceValidation
	for i := range crd.Spec.Versions {
		v := &crd.Spec.Versions[i]
		if v.Storage {
			storage = v.Schema
			break
		}
	}

	if storage == nil || storage.OpenAPIV3Schema == nil {
		return nil, fmt.Errorf("crd %q: no storage version schema", crdPath)
	}

	specProp, ok := storage.OpenAPIV3Schema.Properties["spec"]
	if !ok {
		return nil, fmt.Errorf("crd %q: openAPIV3Schema.properties.spec missing", crdPath)
	}

	out, err := yaml.Marshal(specProp)
	if err != nil {
		return nil, fmt.Errorf("marshal spec fragment %q: %w", crdPath, err)
	}

	return out, nil
}

// GoldenPath returns the path to a golden file under test/schema/golden.
func GoldenPath(repoRoot, goldenFile string) string {
	return filepath.Join(repoRoot, "test", "schema", "golden", goldenFile)
}

// CRDPath returns the path to a CRD manifest under config/crd/bases.
func CRDPath(repoRoot, crdFile string) string {
	return filepath.Join(repoRoot, "config", "crd", "bases", crdFile)
}
