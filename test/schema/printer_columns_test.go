// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package schema

import (
	"os"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// PERF-FIX-05: an operator must be able to read collection scale straight off
// `kubectl get kollecttargets`. That only works if the shipped CRD declares the printer
// columns, and both the kubebuilder CRDs and the Helm chart copy must carry them —
// nothing else in the suite would notice if a regeneration dropped them.
func TestKollectTargetPrinterColumns(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	// Age is load-bearing, not decoration: declaring any additionalPrinterColumns
	// suppresses the apiserver's default AGE column, so dropping this entry in a
	// regeneration would silently remove AGE from `kubectl get kollecttargets`.
	want := map[string]string{
		"Collected": ".status.collectedCount",
		"Updated":   ".status.collectedCountUpdatedAt",
		"Age":       ".metadata.creationTimestamp",
	}

	paths := map[string]string{
		"config/crd/bases": CRDPath(root, "kollect.dev_kollecttargets.yaml"),
		"charts/kollect/crds": root +
			"/charts/kollect/crds/kollect.dev_kollecttargets.yaml",
	}

	for source, path := range paths {
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			got := printerColumns(t, path)
			for name, jsonPath := range want {
				if got[name] != jsonPath {
					t.Fatalf("printer column %q = %q, want %q (columns: %v)", name, got[name], jsonPath, got)
				}
			}
		})
	}
}

// TestKollectClusterTargetPrinterColumns is the TSP-1 cluster parity for the namespaced
// guard above (D3 — one contract, twice): `kubectl get kollectclustertargets` must show
// Collected and Updated on the same status paths, and — because declaring any
// additionalPrinterColumns suppresses the apiserver's default AGE column — the Age entry the
// namespaced kind carries must be mirrored too, or the cluster kind would silently lose it.
func TestKollectClusterTargetPrinterColumns(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	want := map[string]string{
		"Collected": ".status.collectedCount",
		"Updated":   ".status.collectedCountUpdatedAt",
		"Age":       ".metadata.creationTimestamp",
	}

	paths := map[string]string{
		"config/crd/bases": CRDPath(root, "kollect.dev_kollectclustertargets.yaml"),
		"charts/kollect/crds": root +
			"/charts/kollect/crds/kollect.dev_kollectclustertargets.yaml",
	}

	for source, path := range paths {
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			got := printerColumns(t, path)
			for name, jsonPath := range want {
				if got[name] != jsonPath {
					t.Fatalf("printer column %q = %q, want %q (columns: %v)", name, got[name], jsonPath, got)
				}
			}

			// Mirroring is asserted, not just claimed: the cluster column set must equal
			// the namespaced one. The namespaced test pins that set absolutely, so
			// equality here transitively pins the cluster set against name or path drift
			// on either side.
			namespaced := printerColumns(t, CRDPath(root, "kollect.dev_kollecttargets.yaml"))
			for name, jsonPath := range got {
				if namespaced[name] != jsonPath {
					t.Fatalf("cluster printer column %q = %q, want the namespaced %q (mirroring)",
						name, jsonPath, namespaced[name])
				}
			}
		})
	}
}

// printerColumns returns every additionalPrinterColumn declared by a CRD manifest, keyed
// by column name.
func printerColumns(t *testing.T, path string) map[string]string {
	t.Helper()

	//nolint:gosec // G304: path is a committed manifest in this repository.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read crd: %v", err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if unmarshalErr := yaml.Unmarshal(raw, &crd); unmarshalErr != nil {
		t.Fatalf("parse crd: %v", unmarshalErr)
	}

	cols := map[string]string{}
	for i := range crd.Spec.Versions {
		for _, col := range crd.Spec.Versions[i].AdditionalPrinterColumns {
			cols[col.Name] = col.JSONPath
		}
	}

	return cols
}
