// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// builtinPrunePointers are the default RFC 6901 exclusions applied when
// prune.defaults is true (ADR-0306 §Built-in defaults). They drop high-churn
// server-side-apply noise that bloats Git diffs without inventory value.
var builtinPrunePointers = []string{
	"/metadata/managedFields",
	"/metadata/resourceVersion",
	"/metadata/generation",
	"/metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration",
	"/metadata/annotations/argocd.argoproj.io~1tracking-id",
}

// BuiltinPrunePointers returns a copy of the default RFC 6901 prune pointers.
func BuiltinPrunePointers() []string {
	out := make([]string, len(builtinPrunePointers))
	copy(out, builtinPrunePointers)

	return out
}

// PruneResource builds the embedded full-object payload for export.mode: Resource.
//
// It deep-copies the informer object (never mutating the shared cache), keeps the
// sections requested by export.include, drops identity duplicated on the Item
// envelope, applies built-in + author path pruning, and finally scrubs sensitive
// keys via the merged scrubber (ADR-0306, ADR-0303, ADR-0104).
func PruneResource(
	obj *unstructured.Unstructured,
	export *kollectdevv1alpha1.ExportSpec,
	scrubber *Scrubber,
) map[string]any {
	if obj == nil {
		return nil
	}

	root := obj.DeepCopy().Object
	root = selectIncludeSections(root, export.IncludeOrDefault())

	if export.DedupeIdentityEnabled() {
		dropEnvelopeIdentity(root)
	}

	prune := export.Prune
	if prune.DefaultsEnabled() {
		for _, ptr := range builtinPrunePointers {
			removeJSONPointer(root, ptr)
		}
	}

	if prune != nil {
		for _, ptr := range prune.JSONPointers {
			removeJSONPointer(root, ptr)
		}

		for _, jp := range prune.JSONPaths {
			removeJSONPath(root, jp)
		}
	}

	if scrubber != nil {
		if scrubbed, ok := scrubber.Scrub(root).(map[string]any); ok {
			root = scrubbed
		}
	}

	return root
}

// selectIncludeSections returns a new object keeping only the requested top-level
// sections. apiVersion and kind are always kept so the blob stays self-describing.
func selectIncludeSections(root map[string]any, include string) map[string]any {
	if include == kollectdevv1alpha1.ExportIncludeAll {
		return root
	}

	out := make(map[string]any, 4)
	for _, k := range []string{"apiVersion", "kind"} {
		if v, ok := root[k]; ok {
			out[k] = v
		}
	}

	keep := func(section string) {
		if v, ok := root[section]; ok {
			out[section] = v
		}
	}

	switch include {
	case kollectdevv1alpha1.ExportIncludeMetadataOnly:
		keep("metadata")
	case kollectdevv1alpha1.ExportIncludeSpecOnly:
		keep("spec")
	case kollectdevv1alpha1.ExportIncludeStatusOnly:
		keep("status")
	default: // SpecAndStatus
		keep("spec")
		keep("status")
	}

	return out
}

// dropEnvelopeIdentity removes identity fields already carried on the Item
// envelope (name, namespace, uid). apiVersion/kind stay for self-description.
func dropEnvelopeIdentity(root map[string]any) {
	meta, ok := root["metadata"].(map[string]any)
	if !ok {
		return
	}

	for _, k := range []string{"name", "namespace", "uid"} {
		delete(meta, k)
	}
}

// removeJSONPointer deletes the node addressed by an RFC 6901 pointer, if present.
func removeJSONPointer(root map[string]any, pointer string) {
	pointer = strings.TrimSpace(pointer)
	if pointer == "" || pointer == "/" {
		return
	}

	if !strings.HasPrefix(pointer, "/") {
		return
	}

	tokens := strings.Split(pointer[1:], "/")
	for i := range tokens {
		tokens[i] = unescapeJSONPointerToken(tokens[i])
	}

	removeByTokens(root, tokens)
}

func unescapeJSONPointerToken(token string) string {
	token = strings.ReplaceAll(token, "~1", "/")
	token = strings.ReplaceAll(token, "~0", "~")

	return token
}

// removeByTokens walks tokens through nested maps/slices and deletes the leaf.
func removeByTokens(node any, tokens []string) {
	if len(tokens) == 0 {
		return
	}

	last := len(tokens) == 1
	token := tokens[0]

	switch typed := node.(type) {
	case map[string]any:
		if last {
			delete(typed, token)

			return
		}

		removeByTokens(typed[token], tokens[1:])
	case []any:
		idx, err := strconv.Atoi(token)
		if err != nil || idx < 0 || idx >= len(typed) {
			return
		}

		if last {
			// Slices are addressed by index; deletion shifts trailing elements.
			// We cannot reassign the parent's slice header here, so the caller's
			// element is set to nil instead of physically removed.
			typed[idx] = nil

			return
		}

		removeByTokens(typed[idx], tokens[1:])
	}
}

// removeJSONPath deletes a value addressed by a dotted/bracketed JSONPath subset
// ("$.a.b", `$.a["b"]`, "$.a[0]"). Filters and wildcards are no-ops (Phase 1).
func removeJSONPath(root map[string]any, path string) {
	tokens, ok := jsonPathTokens(path)
	if !ok || len(tokens) == 0 {
		return
	}

	removeByTokens(root, tokens)
}

// jsonPathTokens parses a supported JSONPath subset into pointer-style tokens.
// It returns ok=false for filter/wildcard expressions we deliberately skip.
func jsonPathTokens(path string) ([]string, bool) {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$")

	var tokens []string

	i := 0
	for i < len(path) {
		switch path[i] {
		case '.':
			i++
		case '[':
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, false
			}

			inner := path[i+1 : i+end]
			i += end + 1

			inner = strings.TrimSpace(inner)
			if inner == "*" || strings.HasPrefix(inner, "?") {
				return nil, false
			}

			inner = strings.Trim(inner, `"'`)
			tokens = append(tokens, inner)
		default:
			end := i
			for end < len(path) && path[end] != '.' && path[end] != '[' {
				end++
			}

			seg := strings.TrimSpace(path[i:end])
			if seg == "*" {
				return nil, false
			}

			if seg != "" {
				tokens = append(tokens, seg)
			}

			i = end
		}
	}

	return tokens, true
}
