// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package samples_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsinstall "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsvalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// kollectAPIGroup is the only API group this repo owns; documents in it must
// resolve to a committed CRD schema.
const kollectAPIGroup = "kollect.dev"

// minValidatedSampleDocs guards against a walk that silently stops finding
// samples — a truncated or broken walk reporting a vacuous green. 57 kollect.dev
// documents exist today and the floor sits just under that.
//
// What the floor does NOT give you is detection of any single directory going
// missing: six of the eleven sample directories hold exactly two documents, so
// deleting one of those still clears 55. Setting the floor to the exact count
// would instead red the build on every legitimate sample removal — friction, not
// safety. This is a tripwire for 57 → ~0, nothing finer. Raise it when the
// corpus grows.
const minValidatedSampleDocs = 55

// nonKollectSampleGroups is a CLOSED allowlist of foreign API groups that
// legitimately appear under config/samples/. A document in any other group fails
// the gate, so a typo in the group ("kolect.dev") cannot slip past as "not ours".
var nonKollectSampleGroups = map[string]struct{}{
	"":                          {}, // core/v1: Secret, ConfigMap, Service, ...
	"apps":                      {},
	"rbac.authorization.k8s.io": {},
	"kustomize.config.k8s.io":   {},
}

// TestSamplesValidateAgainstCRDSchemas is the schema gate for config/samples/.
//
// Every YAML document under config/samples/ that belongs to the kollect.dev API
// group is:
//
//  1. decoded strictly into its registered Go API type (rejects unknown and
//     duplicated fields), and
//  2. validated against the committed openAPIV3Schema in config/crd/bases/,
//     which is what the API server enforces on `kubectl apply`.
//
// Step 2 is load-bearing: the Go types are deliberately more permissive than the
// CRDs in places (see InventorySinkRefList.UnmarshalJSON, which still accepts the
// legacy bare-string sinkRef form the CRD rejects), so typed decoding alone —
// strict or not — cannot catch a schema-invalid sample.
//
// Nothing gets to opt out silently. Every non-empty document must carry an
// apiVersion and a kind; a kollect.dev document must resolve to a committed CRD
// schema; and a document in any group outside nonKollectSampleGroups fails too.
// A typo in the group, the version, the kind, or a missing apiVersion is a
// FAILURE, never a skip — that is what makes the gate non-vacuous.
//
// What this gate does NOT cover:
//
//   - source files, not rendered overlays — a kustomize patch injecting an
//     invalid field would not be caught here;
//   - allowlisted foreign-group documents, which are skipped entirely because the
//     allowlist keys on group alone, so a malformed Secret or ConfigMap under
//     config/samples/ is never validated;
//   - this project's validating webhooks (internal/webhook/v1alpha1) — a sample
//     can satisfy the CRD schema and still be rejected on apply, as
//     config/samples/team-operator/snapshot-sink.yaml did.
func TestSamplesValidateAgainstCRDSchemas(t *testing.T) {
	t.Parallel()

	validators := loadCRDValidators(t)
	strictDecoder := strictKollectDecoder(t)
	root := filepath.Join("..", "..", "config", "samples")

	validated := 0

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !isYAMLFile(path) {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}

		//nolint:gosec // G304: path comes from walking the repo's config/samples tree.
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("%s: read: %v", rel, readErr)

			return nil
		}

		for i, doc := range splitYAMLDocuments(t, rel, data) {
			docName := fmt.Sprintf("%s[%d]", filepath.ToSlash(rel), i)

			gvk, class, classErr := classifyDocument(doc)
			if class == documentEmpty {
				continue
			}

			if class == documentUnidentified {
				t.Errorf("%s: not a Kubernetes object (%v) — every non-empty document"+
					" under config/samples/ must carry apiVersion and kind", docName, classErr)

				continue
			}

			if gvk.Group != kollectAPIGroup {
				if _, allowed := nonKollectSampleGroups[gvk.Group]; !allowed {
					t.Errorf("%s: unknown API group %q (kind %s) — neither %q nor an allowlisted"+
						" foreign group; a typo in the group must not skip the gate",
						docName, gvk.Group, gvk.Kind, kollectAPIGroup)
				}

				continue
			}

			validated++

			t.Run(docName, func(t *testing.T) {
				t.Parallel()
				validateSampleDocument(t, docName, gvk, doc, validators, strictDecoder)
			})
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	if validated < minValidatedSampleDocs {
		t.Fatalf(
			"only %d kollect.dev sample documents validated, expected at least %d — the walk is missing config/samples/",
			validated,
			minValidatedSampleDocs,
		)
	}
}

// validateSampleDocument runs the strict typed decode and the CRD schema check on one document.
func validateSampleDocument(
	t *testing.T,
	docName string,
	gvk schema.GroupVersionKind,
	doc []byte,
	validators map[schema.GroupVersionKind]apiextensionsvalidation.SchemaValidator,
	strictDecoder runtime.Decoder,
) {
	t.Helper()

	if _, _, err := strictDecoder.Decode(doc, nil, nil); err != nil {
		t.Errorf("%s: strict decode into %s failed: %v", docName, gvk.Kind, err)
	}

	validator, ok := validators[gvk]
	if !ok {
		t.Errorf(
			"%s: no CRD schema in config/crd/bases/ for %s — a kollect.dev sample must match a committed CRD",
			docName,
			gvk,
		)

		return
	}

	var unstructured map[string]any
	if err := yaml.Unmarshal(doc, &unstructured); err != nil {
		t.Errorf("%s: parse for schema validation: %v", docName, err)

		return
	}

	if errs := apiextensionsvalidation.ValidateCustomResource(nil, unstructured, validator); len(errs) > 0 {
		t.Errorf("%s: rejected by the %s CRD schema (the API server would reject `kubectl apply` too):\n%s",
			docName, gvk.Kind, formatFieldErrors(errs))
	}
}

// formatFieldErrors renders a field.ErrorList one error per indented line.
func formatFieldErrors(errs field.ErrorList) string {
	lines := make([]string, 0, len(errs))
	for _, e := range errs {
		lines = append(lines, "  - "+e.Error())
	}

	return strings.Join(lines, "\n")
}

// loadCRDValidators builds one OpenAPI validator per served CRD version from config/crd/bases/.
func loadCRDValidators(t *testing.T) map[schema.GroupVersionKind]apiextensionsvalidation.SchemaValidator {
	t.Helper()

	crdScheme := runtime.NewScheme()
	apiextensionsinstall.Install(crdScheme)

	basesDir := filepath.Join("..", "..", "config", "crd", "bases")
	entries, err := os.ReadDir(basesDir)
	if err != nil {
		t.Fatalf("read %s: %v", basesDir, err)
	}

	validators := make(map[schema.GroupVersionKind]apiextensionsvalidation.SchemaValidator)

	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}

		path := filepath.Join(basesDir, entry.Name())

		//nolint:gosec // G304: path is a committed manifest under config/crd/bases.
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read crd %s: %v", path, readErr)
		}

		var external apiextensionsv1.CustomResourceDefinition
		if unmarshalErr := yaml.Unmarshal(raw, &external); unmarshalErr != nil {
			t.Fatalf("parse crd %s: %v", path, unmarshalErr)
		}

		var internal apiextensions.CustomResourceDefinition
		if convErr := crdScheme.Convert(&external, &internal, nil); convErr != nil {
			t.Fatalf("convert crd %s to internal: %v", path, convErr)
		}

		for i := range internal.Spec.Versions {
			version := &internal.Spec.Versions[i]

			props := versionSchemaProps(&internal, version)
			if props == nil {
				t.Fatalf("crd %s: version %s has no openAPIV3Schema", path, version.Name)
			}

			validator, _, validatorErr := apiextensionsvalidation.NewSchemaValidator(props)
			if validatorErr != nil {
				t.Fatalf("crd %s: build validator for %s: %v", path, version.Name, validatorErr)
			}

			gvk := schema.GroupVersionKind{
				Group:   internal.Spec.Group,
				Version: version.Name,
				Kind:    internal.Spec.Names.Kind,
			}
			validators[gvk] = validator
		}
	}

	if len(validators) == 0 {
		t.Fatalf("no CRD schemas loaded from %s", basesDir)
	}

	return validators
}

// versionSchemaProps returns the per-version schema, falling back to the CRD-wide one.
func versionSchemaProps(
	crd *apiextensions.CustomResourceDefinition,
	version *apiextensions.CustomResourceDefinitionVersion,
) *apiextensions.JSONSchemaProps {
	if version.Schema != nil && version.Schema.OpenAPIV3Schema != nil {
		return version.Schema.OpenAPIV3Schema
	}

	if crd.Spec.Validation != nil {
		return crd.Spec.Validation.OpenAPIV3Schema
	}

	return nil
}

// strictKollectDecoder returns a decoder that rejects unknown or duplicated fields.
func strictKollectDecoder(t *testing.T) runtime.Decoder {
	t.Helper()

	apiScheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(apiScheme); err != nil {
		t.Fatalf("add kollect scheme: %v", err)
	}

	return serializer.NewCodecFactory(apiScheme, serializer.EnableStrict).UniversalDeserializer()
}

// splitYAMLDocuments splits a multi-document YAML stream into its documents.
func splitYAMLDocuments(t *testing.T, rel string, data []byte) [][]byte {
	t.Helper()

	reader := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))

	var docs [][]byte

	for {
		doc, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Errorf("%s: split yaml documents: %v", rel, err)

			break
		}

		docs = append(docs, doc)
	}

	return docs
}

// documentClass says whether a YAML document is empty, identifiable, or junk.
type documentClass int

const (
	// documentEmpty is a comment-only or blank document — nothing to validate.
	documentEmpty documentClass = iota
	// documentIdentified carries a parseable apiVersion and kind.
	documentIdentified
	// documentUnidentified has content but no usable apiVersion/kind.
	documentUnidentified
)

// classifyDocument reads apiVersion/kind from one YAML document.
// A document with content but no usable apiVersion/kind is documentUnidentified,
// never silently dropped: that is how a bogus sample would otherwise escape.
func classifyDocument(doc []byte) (schema.GroupVersionKind, documentClass, error) {
	var parsed any
	if err := yaml.Unmarshal(doc, &parsed); err != nil {
		return schema.GroupVersionKind{}, documentUnidentified, fmt.Errorf("parse yaml: %w", err)
	}

	if parsed == nil {
		return schema.GroupVersionKind{}, documentEmpty, nil
	}

	obj, ok := parsed.(map[string]any)
	if !ok {
		return schema.GroupVersionKind{}, documentUnidentified,
			fmt.Errorf("top level is %T, expected a mapping", parsed)
	}

	if len(obj) == 0 {
		return schema.GroupVersionKind{}, documentEmpty, nil
	}

	apiVersion, _ := obj["apiVersion"].(string)
	kind, _ := obj["kind"].(string)

	if apiVersion == "" || kind == "" {
		return schema.GroupVersionKind{}, documentUnidentified,
			fmt.Errorf("apiVersion=%q kind=%q", apiVersion, kind)
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionKind{}, documentUnidentified,
			fmt.Errorf("parse apiVersion %q: %w", apiVersion, err)
	}

	return gv.WithKind(kind), documentIdentified, nil
}

// isYAMLFile reports whether a path carries a YAML extension.
func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	return ext == ".yaml" || ext == ".yml"
}
