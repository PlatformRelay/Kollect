// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package schema

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// API-HTTPDOC-01.
//
// `http` and `azureblob` are RESERVED KollectSnapshotSink type names: admission
// rejects them because no backend implementation ships
// (internal/validation/family_sink.go). controller-gen copies the Go doc comment
// on KollectSnapshotSinkSpec.HTTP verbatim into every generated CRD description,
// so a comment phrased like a feature announcement publishes a falsehood in the
// cluster's own schema -- `kubectl explain` then contradicts
// docs/examples/connection-test.md, which states the truth and is itself gated.
//
// Nothing bound the Go comment, the four generated artifacts, and the public
// docs together, so the two sides drifted apart and only one side was gated.
// This test is that binding.
const (
	reservedHTTPField = "http"

	// The exact false claim this story removed. Reintroducing it in the doc
	// comment would silently republish it through every generator below.
	forbiddenHTTPClaim = "configures webhook snapshot export"
)

// Every phrase the generated description must carry, lowercased.
var requiredHTTPDescriptionPhrases = []string{"reserved", "rejected by admission"}

func readRepoFile(t *testing.T, root string, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{root}, parts...)...)

	//nolint:gosec // G304: path is composed from repo-relative literals in this test.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(raw)
}

// squash collapses every run of whitespace to a single space. CRD descriptions
// are re-folded by the YAML writer at arbitrary column widths, so a literal
// substring check on the raw file can fail purely because the folder split
// between two words.
func squash(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// snapshotSinkSpecProps returns the storage-version spec properties of the
// KollectSnapshotSink CRD at path.
func snapshotSinkSpecProps(t *testing.T, path string) map[string]apiextensionsv1.JSONSchemaProps {
	t.Helper()

	//nolint:gosec // G304: path is a committed CRD manifest.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if unmarshalErr := yaml.Unmarshal(raw, &crd); unmarshalErr != nil {
		t.Fatalf("parse %s: %v", path, unmarshalErr)
	}

	for i := range crd.Spec.Versions {
		version := &crd.Spec.Versions[i]
		if !version.Storage || version.Schema == nil || version.Schema.OpenAPIV3Schema == nil {
			continue
		}

		spec, ok := version.Schema.OpenAPIV3Schema.Properties["spec"]
		if !ok {
			t.Fatalf("%s: openAPIV3Schema.properties.spec missing", path)
		}

		return spec.Properties
	}

	t.Fatalf("%s: no storage version schema", path)

	return nil
}

// goldenSpecProps returns the spec properties recorded in a golden OpenAPI
// fragment, which is a bare JSONSchemaProps rather than a whole CRD.
func goldenSpecProps(t *testing.T, path string) map[string]apiextensionsv1.JSONSchemaProps {
	t.Helper()

	//nolint:gosec // G304: path is a committed golden under test/schema/golden.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var props apiextensionsv1.JSONSchemaProps
	if unmarshalErr := yaml.Unmarshal(raw, &props); unmarshalErr != nil {
		t.Fatalf("parse %s: %v", path, unmarshalErr)
	}

	return props.Properties
}

// httpFieldDoc returns the Go doc comment on KollectSnapshotSinkSpec.HTTP with
// the kubebuilder marker lines removed -- i.e. exactly the text controller-gen
// turns into the CRD description.
func httpFieldDoc(t *testing.T, root string) string {
	t.Helper()

	path := filepath.Join(root, "api", "v1alpha1", "kollectsnapshotsink_types.go")

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var doc string

	ast.Inspect(file, func(node ast.Node) bool {
		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok || typeSpec.Name.Name != "KollectSnapshotSinkSpec" {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return false
		}

		for _, field := range structType.Fields.List {
			if !strings.Contains(field.Tag.Value, `json:"`+reservedHTTPField+`,`) {
				continue
			}

			if field.Doc == nil {
				t.Fatalf("%s: KollectSnapshotSinkSpec.%s has no doc comment", path, reservedHTTPField)
			}

			lines := make([]string, 0, len(field.Doc.List))

			for _, comment := range field.Doc.List {
				text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
				if strings.HasPrefix(text, "+") {
					continue
				}

				lines = append(lines, text)
			}

			doc = strings.Join(lines, "\n")
		}

		return false
	})

	if doc == "" {
		t.Fatalf("%s: could not read the doc comment of KollectSnapshotSinkSpec.%s", path, reservedHTTPField)
	}

	return doc
}

func assertDescribesReservedType(t *testing.T, label, description string) {
	t.Helper()

	if description == "" {
		t.Fatalf("%s: spec.%s has no description", label, reservedHTTPField)
	}

	flat := strings.ToLower(squash(description))

	if strings.Contains(flat, forbiddenHTTPClaim) {
		t.Errorf(
			"%s: spec.%s still claims %q, but admission rejects `type: %s` -- no implementation ships\n"+
				"got: %s",
			label, reservedHTTPField, forbiddenHTTPClaim, reservedHTTPField, squash(description),
		)
	}

	for _, phrase := range requiredHTTPDescriptionPhrases {
		if !strings.Contains(flat, phrase) {
			t.Errorf(
				"%s: spec.%s description must say %q so the published schema matches admission\n"+
					"got: %s",
				label, reservedHTTPField, phrase, squash(description),
			)
		}
	}
}

// TestReservedHTTPSinkTypeIsDocumentedAsRejected pins the one sentence that
// admission, the generated schema, and the public docs must agree on.
func TestReservedHTTPSinkTypeIsDocumentedAsRejected(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)

	t.Run("admission rejects the reserved type", func(t *testing.T) {
		t.Parallel()

		validation := readRepoFile(t, root, "internal", "validation", "family_sink.go")
		if strings.Contains(validation, "SnapshotSinkTypeHTTP,") {
			t.Fatal("internal/validation/family_sink.go admits the reserved `http` type; " +
				"this test documents a rejection that no longer happens")
		}
	})

	t.Run("api doc comment", func(t *testing.T) {
		t.Parallel()

		doc := httpFieldDoc(t, root)
		assertDescribesReservedType(t, "api/v1alpha1/kollectsnapshotsink_types.go", doc)

		// hack/gen-glossary.py renders only the FIRST line of the description
		// into docs/GLOSSARY.md, so line one has to stand on its own.
		first := strings.TrimSpace(strings.SplitN(doc, "\n", 2)[0])
		if !strings.HasPrefix(first, reservedHTTPField+" ") || !strings.HasSuffix(first, ".") {
			t.Errorf(
				"the first doc-comment line is rendered alone into docs/GLOSSARY.md, so it must be a "+
					"complete sentence starting with %q and ending in a period\ngot: %s",
				reservedHTTPField, first,
			)
		}

		assertDescribesReservedType(t, "api doc comment, first line", first)
	})

	// One doc comment, three committed copies, three different generators. All
	// of them have to be regenerated together, so all of them are asserted.
	const (
		crdFile    = "kollect.dev_kollectsnapshotsinks.yaml"
		goldenFile = "kollectsnapshotsink.spec.openapi.yaml"
	)

	generated := []struct {
		label string
		path  string
		// The golden file is a bare spec fragment, not a whole CRD.
		fragment bool
	}{
		{
			label: filepath.Join("config", "crd", "bases", crdFile),
			path:  filepath.Join(root, "config", "crd", "bases", crdFile),
		},
		{
			label: filepath.Join("charts", "kollect", "crds", crdFile),
			path:  filepath.Join(root, "charts", "kollect", "crds", crdFile),
		},
		{
			label:    filepath.Join("test", "schema", "golden", goldenFile),
			path:     GoldenPath(root, goldenFile),
			fragment: true,
		},
	}

	for _, artifact := range generated {
		label := artifact.label

		t.Run(label, func(t *testing.T) {
			t.Parallel()

			var props map[string]apiextensionsv1.JSONSchemaProps

			if artifact.fragment {
				props = goldenSpecProps(t, artifact.path)
			} else {
				props = snapshotSinkSpecProps(t, artifact.path)
			}

			prop, ok := props[reservedHTTPField]
			if !ok {
				t.Fatalf("%s: spec.%s missing", label, reservedHTTPField)
			}

			assertDescribesReservedType(t, label, prop.Description)

			// The description is generated, so it must not diverge from its source.
			if squash(prop.Description) != squash(httpFieldDoc(t, root)) {
				t.Errorf(
					"%s: spec.%s description is stale against the Go doc comment; regenerate\nhave: %s\nwant: %s",
					label, reservedHTTPField, squash(prop.Description), squash(httpFieldDoc(t, root)),
				)
			}
		})
	}

	t.Run("public docs say the same thing", func(t *testing.T) {
		t.Parallel()

		example := squash(readRepoFile(t, root, "docs", "examples", "connection-test.md"))
		want := "Reserved backend names (`azureblob`, `http`) are rejected by admission because no implementation ships"

		if !strings.Contains(example, want) {
			t.Errorf("docs/examples/connection-test.md no longer states %q", want)
		}

		glossary := readRepoFile(t, root, "docs", "GLOSSARY.md")

		var row string

		for _, line := range strings.Split(glossary, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "| `"+reservedHTTPField+"` |") {
				row = line

				break
			}
		}

		if row == "" {
			t.Fatalf("docs/GLOSSARY.md has no `%s` row; run: python3 hack/gen-glossary.py", reservedHTTPField)
		}

		assertDescribesReservedType(t, "docs/GLOSSARY.md", row)
	})
}
