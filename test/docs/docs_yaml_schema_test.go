// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package docs_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

// kollectAPIGroup is the only API group this repo owns; a document in it must
// resolve to a committed CRD schema.
const kollectAPIGroup = "kollect.dev"

// Corpus counts. A gate that silently stops finding YAML reports a vacuous
// green, which is how this defect class reached its third recurrence.
//
// Two of the three are exact equalities that fail in both directions; the third
// is a lower bound, and says so. See each constant for why.
const (
	// minDiscoveredYAMLBlocks is a LOWER BOUND on every YAML block found under
	// docs/ -- fenced blocks in Markdown plus standalone .yaml/.yml files. 70 exist
	// today. It stays a bound rather than an equality because it also counts blocks
	// that legitimately opt out, so it moves for reasons that are not defects.
	// It is a "70 -> ~0" tripwire only: it does not detect a single page losing its
	// examples, and it cannot detect a block the extractor never discovered at all,
	// because an undiscovered block is not counted either.
	minDiscoveredYAMLBlocks = 65

	// validatedKollectDocs counts complete kollect.dev objects that actually went
	// through both checks. It is an EQUALITY against the exact corpus, not a bound
	// below it, and it fails in BOTH directions.
	//
	// Fewer than this means an example was deleted or redirected away with
	// `superseded`/`proposed` on an ADR/RFC page -- the anti-vacuity case. More
	// means an example was added; that must move this constant too, so a reviewer
	// sees the corpus change stated in the diff rather than absorbed by slack.
	validatedKollectDocs = 30

	// validatedFragments counts kollect.dev fragments that actually went through
	// both checks. Also an EQUALITY, failing in both directions, and for a sharper
	// reason: headroom here is not slack, it is the exploit. With this set to 8
	// against a corpus of 10, two fragments could be reverted to the bare-string
	// sink-ref form this lane exists to eliminate and relabelled
	// `kollect-doc: ignore` while the gate stayed green. Nothing verifies that an
	// `ignore` reason is honest, so this count is the only thing between a
	// plausible one-liner and re-admitting the defect.
	validatedFragments = 10
)

// nonKollectDocGroups is a CLOSED allowlist of foreign API groups that
// legitimately appear in docs/. A document in any other group fails the gate, so
// a typo in the group ("kolect.dev") cannot pass itself off as "not ours".
var nonKollectDocGroups = map[string]struct{}{
	"":                          {}, // core/v1: Secret, ConfigMap, Namespace, ...
	"apps":                      {},
	"rbac.authorization.k8s.io": {},
	"kustomize.config.k8s.io":   {},
	"monitoring.coreos.com":     {},
}

// TestDocsYAMLValidatesAgainstCRDSchemas is the schema gate for docs/.
//
// It applies the same two checks test/samples/crd_schema_test.go applies to
// config/samples/ -- strict typed decode plus validation against the committed
// openAPIV3Schema in config/crd/bases/ -- to every YAML example published on the
// documentation site.
//
// Both checks are load-bearing and neither subsumes the other:
//
//   - Strict decode catches fields the API does not have. A structural CRD schema
//     PRUNES unknown fields rather than rejecting them, so `kubectl apply` accepts
//     a document carrying a removed field and silently drops it. Schema validation
//     alone cannot see that.
//   - Schema validation catches value shapes the Go types accept but the CRD does
//     not -- InventorySinkRefList.UnmarshalJSON still decodes the legacy bare-string
//     sink-ref form that the CRD rejects. Strict decoding alone cannot see that.
//
// # Coverage tiers
//
// Tier 1 -- complete documents (apiVersion + kind present). Validated in full. No
// opt-out: a `kollect-doc: ignore` directive on a complete kollect.dev object is
// itself a failure, so the escape hatch cannot be used to silence a real example.
//
// Tier 2 -- fragments (a partial snippet with no apiVersion/kind -- legitimate and
// common in docs). Every fragment must carry a `# kollect-doc:` directive as the
// first line of its block; an undirected fragment FAILS. A `fragment <Kind>` or
// `fragment <Kind>.<path>` directive is synthesised into a whole object and run
// through both checks with Required errors filtered out, which is exactly what
// "this snippet is abbreviated" means.
//
// Directives are per-DOCUMENT and never per-file: a marker on the first document
// of a multi-document fence does not reach the others, and there is no way to
// silence a page.
//
// # What this gate does NOT cover
//
//   - Prose. A `sinkRefs` in a table row, a mermaid edge label, or a shell comment
//     is invisible here. hack/test/docs_removed_api_fields_test.sh covers that.
//
//   - Non-YAML fenced blocks (```json, ```sh, ```bash) -- their
//     bodies are never parsed. They are still walked to their closer, because a
//     fence the scanner declines to collect but fails to step over leaves it
//     mid-body and loses every later block on the page.
//
//   - MARKUP VARIANTS THE FENCE SCANNER DOES NOT RECOGNISE. This is the gate's
//     most dangerous gap, because it fails SILENTLY: an undiscovered block is not
//     validated, not counted, and cannot move minDiscoveredYAMLBlocks, so no floor
//     detects it and no message is printed. It is also the gap this repo has
//     reopened three times.
//
//     What IS discovered is pinned case by case in fence_scanner_test.go -- read
//     that table, not this paragraph, for the authoritative list. In summary:
//     backtick or tilde fences of any length >= 3, with the language taken from
//     the first token of the info string; attribute lists resolve to their FIRST
//     class, so "{.yaml .annotate}" is YAML and "{.annotate .yaml}" deliberately
//     is not, matching what superfences highlights. Fences indented in list items,
//     content tabs and admonitions are discovered, as are blockquoted fences --
//     at any indentation after the marker and at any nesting depth, so the
//     blockquoted-list-item shape ">    ```yaml" counts. Because md_in_html is
//     enabled (mkdocs.yml:135), a fence inside a <div markdown="1"> is discovered
//     too -- and so is one inside raw HTML that carries no markdown attribute at
//     all, because superfences is a PREPROCESSOR: it lexes the fence before the
//     HTML block ever matters, and this scanner is line-based for the same reason.
//
//     Known remaining gaps: YAML pulled in by a pymdownx.snippets include (the
//     text is not in the page at all), and a superfences custom_fences block whose
//     name is not "yaml".
//
//     Two live escapes have come from this class already: a bare-string
//     databaseSinkRefs fragment behind a "{.yaml .annotate}" fence, and a complete
//     object behind a title attribute containing backticks. If you are fixing a
//     third, the code to change is extractFencedYAML and fenceOpenPattern below --
//     infoStringLanguage only classifies an info string once the walker has
//     already found the fence, so a variant the walker never matches will never
//     reach it. Add the case to fence_scanner_test.go first and watch it fail.
//
//   - Blocks a human declared out of scope with `kollect-doc: ignore <reason>`
//     (Helm values, CI workflows, Prometheus config). The reason is mandatory and
//     reader-visible, but nothing verifies that the reason is honest.
//
//   - Examples under docs/adr/ and docs/rfc/ marked `kollect-doc: superseded
//     <reason>` or `kollect-doc: proposed <reason>`. An ADR records a decision as
//     taken and an RFC records one never taken; retconning either would destroy the
//     record. The directive text renders in the page, so the reader is told.
//
//   - Allowlisted foreign-group documents (Secret, Role, ...), skipped entirely --
//     the allowlist keys on group alone, so a malformed Secret in docs/ is never
//     validated.
//
//   - This project's validating webhooks: an example can satisfy the CRD schema
//     and still be rejected on apply.
//
//   - Fragment paths are trusted. `fragment KollectProfile.spec` on a snippet that
//     is really a KollectTarget spec validates against the wrong schema.
//
//   - Semantics. An example whose effective policy is wider or narrower than the
//     surrounding prose claims is schema-valid and passes. A KollectScope that
//     populates one family allowlist and leaves the other two empty reads as
//     "export is capped" but leaves two families unrestricted; no gate here, and
//     none in hack/test/docs_removed_api_fields_test.sh, compares an example
//     against what its page says it does.
func TestDocsYAMLValidatesAgainstCRDSchemas(t *testing.T) {
	t.Parallel()

	validators := loadCRDValidators(t)
	strictDecoder := strictKollectDecoder(t)
	root := filepath.Join("..", "..", "docs")

	counts := &discoveryCounts{}

	blocks := collectDocsYAMLBlocks(t, root)
	counts.blocks = len(blocks)

	for _, block := range blocks {
		validateDocsBlock(t, block, validators, strictDecoder, counts)
	}

	counts.assertFloors(t)
}

// discoveryCounts tracks what the walk actually reached.
type discoveryCounts struct {
	blocks    int
	fullDocs  int
	fragments int
}

// assertFloors fails the gate when discovery collapsed.
func (c *discoveryCounts) assertFloors(t *testing.T) {
	t.Helper()

	if c.blocks < minDiscoveredYAMLBlocks {
		t.Errorf("only %d YAML blocks discovered under docs/, expected at least %d --"+
			" the extractor is missing examples and the green is vacuous", c.blocks, minDiscoveredYAMLBlocks)
	}

	if c.fullDocs != validatedKollectDocs {
		t.Errorf("%d complete kollect.dev documents validated, expected exactly %d."+
			" Fewer means a complete example was deleted, or redirected away with"+
			" `superseded`/`proposed`; more means one was added. Either way, move"+
			" validatedKollectDocs in the same commit so the change is stated",
			c.fullDocs, validatedKollectDocs)
	}

	if c.fragments != validatedFragments {
		t.Errorf("%d kollect.dev fragments validated, expected exactly %d."+
			" Fewer means a fragment was deleted, or redirected away with"+
			" `ignore`/`superseded`/`proposed`; more means one was added. Either way,"+
			" move validatedFragments in the same commit so the change is stated",
			c.fragments, validatedFragments)
	}
}

// validateDocsBlock runs every document of one block through the tier it belongs to.
func validateDocsBlock(
	t *testing.T,
	block yamlBlock,
	validators map[schema.GroupVersionKind]apiextensionsvalidation.SchemaValidator,
	strictDecoder runtime.Decoder,
	counts *discoveryCounts,
) {
	t.Helper()

	for i, doc := range splitYAMLDocuments(t, block.name(), block.body) {
		docName := fmt.Sprintf("%s#%d", block.name(), i)

		directive, err := parseDirective(block, doc)
		if err != nil {
			t.Errorf("%s: %v", docName, err)

			continue
		}

		gvk, class, classErr := classifyDocument(doc)

		switch class {
		case documentEmpty:
			continue
		case documentIdentified:
			validateIdentifiedDoc(t, docName, gvk, doc, directive, validators, strictDecoder, counts)
		case documentUnidentified:
			validateFragmentDoc(t, docName, doc, classErr, directive, validators, strictDecoder, counts)
		}
	}
}

// validateIdentifiedDoc handles a complete Kubernetes object (tier 1).
func validateIdentifiedDoc(
	t *testing.T,
	docName string,
	gvk schema.GroupVersionKind,
	doc []byte,
	directive blockDirective,
	validators map[schema.GroupVersionKind]apiextensionsvalidation.SchemaValidator,
	strictDecoder runtime.Decoder,
	counts *discoveryCounts,
) {
	t.Helper()

	if gvk.Group != kollectAPIGroup {
		if _, allowed := nonKollectDocGroups[gvk.Group]; !allowed {
			t.Errorf("%s: unknown API group %q (kind %s) -- neither %q nor an allowlisted"+
				" foreign group; a typo in the group must not skip the gate",
				docName, gvk.Group, gvk.Kind, kollectAPIGroup)
		}

		return
	}

	if directive.kind == directiveIgnore {
		t.Errorf("%s: a complete kollect.dev %s carries `kollect-doc: ignore` --"+
			" complete objects cannot opt out of the gate", docName, gvk.Kind)

		return
	}

	if directive.kind == directiveNotShipped {
		return
	}

	counts.fullDocs++
	validateDocument(t, docName, gvk, doc, validators, strictDecoder, false)
}

// validateFragmentDoc handles an abbreviated snippet (tier 2).
func validateFragmentDoc(
	t *testing.T,
	docName string,
	doc []byte,
	classErr error,
	directive blockDirective,
	validators map[schema.GroupVersionKind]apiextensionsvalidation.SchemaValidator,
	strictDecoder runtime.Decoder,
	counts *discoveryCounts,
) {
	t.Helper()

	switch directive.kind {
	case directiveIgnore, directiveNotShipped:
		return
	case directiveNone:
		t.Errorf("%s: undirected YAML fragment (%v). Every fragment must declare its contract on"+
			" the first line of its own block: `# kollect-doc: fragment <Kind>[.<path>]`,"+
			" `# kollect-doc: ignore <reason>`, or (docs/adr, docs/rfc only)"+
			" `# kollect-doc: superseded|proposed <reason>`", docName, classErr)
	case directiveFragment:
		synthetic, synthGVK, synthErr := synthesiseFragment(directive, doc)
		if synthErr != nil {
			t.Errorf("%s: %v", docName, synthErr)

			return
		}

		counts.fragments++
		validateDocument(t, docName, synthGVK, synthetic, validators, strictDecoder, true)
	}
}

// validateDocument runs the strict typed decode and the CRD schema check on one document.
// When partial is set, Required errors are dropped: an abbreviated snippet is
// allowed to omit fields, but never to misspell or mis-shape the ones it shows.
func validateDocument(
	t *testing.T,
	docName string,
	gvk schema.GroupVersionKind,
	doc []byte,
	validators map[schema.GroupVersionKind]apiextensionsvalidation.SchemaValidator,
	strictDecoder runtime.Decoder,
	partial bool,
) {
	t.Helper()

	if _, _, err := strictDecoder.Decode(doc, nil, nil); err != nil {
		t.Errorf("%s: strict decode into %s failed -- the field does not exist on the Go API type,"+
			" and the CRD would PRUNE it silently rather than reject it: %v", docName, gvk.Kind, err)
	}

	validator, ok := validators[gvk]
	if !ok {
		t.Errorf("%s: no CRD schema in config/crd/bases/ for %s -- a documented kollect.dev"+
			" example must match a committed CRD", docName, gvk)

		return
	}

	var unstructured map[string]any
	if err := yaml.Unmarshal(doc, &unstructured); err != nil {
		t.Errorf("%s: parse for schema validation: %v", docName, err)

		return
	}

	errs := apiextensionsvalidation.ValidateCustomResource(nil, unstructured, validator)
	if partial {
		errs = dropRequiredErrors(errs)
	}

	if len(errs) > 0 {
		t.Errorf("%s: rejected by the %s CRD schema (the API server would reject `kubectl apply` too):\n%s",
			docName, gvk.Kind, formatFieldErrors(errs))
	}
}

// dropRequiredErrors removes "field is required" errors, which an abbreviated snippet may legitimately trigger.
func dropRequiredErrors(errs field.ErrorList) field.ErrorList {
	kept := make(field.ErrorList, 0, len(errs))

	for _, e := range errs {
		if e.Type == field.ErrorTypeRequired {
			continue
		}

		kept = append(kept, e)
	}

	return kept
}

// formatFieldErrors renders a field.ErrorList one error per indented line.
func formatFieldErrors(errs field.ErrorList) string {
	lines := make([]string, 0, len(errs))
	for _, e := range errs {
		lines = append(lines, "  - "+e.Error())
	}

	return strings.Join(lines, "\n")
}

// directiveClass is the contract a YAML block declares for itself.
type directiveClass int

const (
	// directiveNone means the block carried no `kollect-doc:` line.
	directiveNone directiveClass = iota
	// directiveFragment means the block is an abbreviated part of a named kind.
	directiveFragment
	// directiveIgnore means the block is not a kollect API object at all.
	directiveIgnore
	// directiveNotShipped means the block deliberately shows API surface that is
	// not the shipped one -- superseded by a later decision, or proposed and never
	// implemented. ADR/RFC pages only.
	directiveNotShipped
)

// notShippedVerbs are the ADR/RFC-only directives that exempt a block from the
// shipped API, mapped to the wording their reason must carry.
var notShippedVerbs = map[string]string{
	"superseded": "e.g. `superseded by ADR-0414`",
	"proposed":   "e.g. `proposed never implemented, ADR-0604 is Parked`",
}

// blockDirective is a parsed `# kollect-doc:` line.
type blockDirective struct {
	kind directiveClass
	// crdKind is the Kubernetes kind a fragment belongs to.
	crdKind string
	// path is the dotted location of the fragment inside that kind ("" = object root).
	path string
}

// directivePattern matches the directive on the first line of a block.
var directivePattern = regexp.MustCompile(`^#\s*kollect-doc:\s*(\S+)\s*(.*)$`)

// parseDirective reads the `# kollect-doc:` line, which must be the first
// non-blank line of THIS DOCUMENT -- not of the fence it shares with others.
// A multi-document fence therefore needs one directive per exempted document,
// so a single marker can never silence its fence-mates, and there is no way to
// declare a contract for a whole file.
func parseDirective(block yamlBlock, doc []byte) (blockDirective, error) {
	first := ""

	for _, line := range strings.Split(string(doc), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		first = strings.TrimSpace(line)

		break
	}

	match := directivePattern.FindStringSubmatch(first)
	if match == nil {
		return blockDirective{kind: directiveNone}, nil
	}

	verb, rest := match[1], strings.TrimSpace(match[2])

	switch verb {
	case "fragment":
		crdKind, path, _ := strings.Cut(rest, ".")
		if crdKind == "" {
			return blockDirective{}, errors.New(
				"`kollect-doc: fragment` needs a kind, e.g. `fragment KollectInventory.spec`")
		}

		return blockDirective{kind: directiveFragment, crdKind: crdKind, path: path}, nil
	case "ignore":
		if rest == "" {
			return blockDirective{}, errors.New("`kollect-doc: ignore` needs a reason on the same line," +
				" e.g. `ignore Helm values, not a kollect API object`")
		}

		return blockDirective{kind: directiveIgnore}, nil
	default:
		example, isNotShipped := notShippedVerbs[verb]
		if !isNotShipped {
			return blockDirective{}, fmt.Errorf("unknown `kollect-doc` directive %q --"+
				" expected fragment, ignore, superseded, or proposed", verb)
		}

		if !block.notShippedAllowed() {
			return blockDirective{}, fmt.Errorf("`kollect-doc: %s` is only allowed under"+
				" docs/adr/ and docs/rfc/, where a page records a decision rather than the"+
				" shipped API, not in %s", verb, block.path)
		}

		if rest == "" {
			return blockDirective{}, fmt.Errorf(
				"`kollect-doc: %s` needs a reason on the same line, %s", verb, example)
		}

		return blockDirective{kind: directiveNotShipped}, nil
	}
}

// synthesiseFragment wraps a fragment into a whole object so it can be schema-validated.
func synthesiseFragment(directive blockDirective, doc []byte) ([]byte, schema.GroupVersionKind, error) {
	var parsed map[string]any
	if err := yaml.Unmarshal(doc, &parsed); err != nil {
		return nil, schema.GroupVersionKind{}, fmt.Errorf("parse fragment: %w", err)
	}

	gvk := schema.GroupVersionKind{
		Group:   kollectAPIGroup,
		Version: kollectdevv1alpha1.GroupVersion.Version,
		Kind:    directive.crdKind,
	}

	object := map[string]any{
		"apiVersion": gvk.GroupVersion().String(),
		"kind":       gvk.Kind,
	}

	if directive.path == "" {
		for key, value := range parsed {
			if key == "apiVersion" || key == "kind" {
				continue
			}

			object[key] = value
		}
	} else {
		cursor := object

		segments := strings.Split(directive.path, ".")
		for _, segment := range segments[:len(segments)-1] {
			next := map[string]any{}
			cursor[segment] = next
			cursor = next
		}

		cursor[segments[len(segments)-1]] = parsed
	}

	encoded, err := yaml.Marshal(object)
	if err != nil {
		return nil, gvk, fmt.Errorf("re-encode fragment: %w", err)
	}

	return encoded, gvk, nil
}

// yamlBlock is one YAML example discovered under docs/.
type yamlBlock struct {
	// path is the repo-relative source file.
	path string
	// line is the first line of the YAML body in that file (1-based).
	line int
	// body is the YAML text with any fence indentation stripped.
	body string
}

// name identifies the block by source location.
func (b yamlBlock) name() string {
	return fmt.Sprintf("%s:%d", b.path, b.line)
}

// notShippedAllowed reports whether this block may exempt itself from the shipped API.
func (b yamlBlock) notShippedAllowed() bool {
	slashed := filepath.ToSlash(b.path)

	return strings.HasPrefix(slashed, "docs/adr/") || strings.HasPrefix(slashed, "docs/rfc/")
}

// skippedDocsDirs are build artefacts that live under docs/ but are not docs.
// They are gitignored; a `task lint:markdown` run installs docs/node_modules,
// whose vendored YAML would otherwise flood the gate. Dot-directories are skipped
// on the same grounds.
var skippedDocsDirs = map[string]struct{}{
	"node_modules": {},
	"site":         {},
}

// fenceOpenPattern matches ANY fenced code block opener -- backtick or tilde,
// three or more -- capturing indentation, the fence marker, and the info string.
//
// It deliberately does not anchor on the bare "```yaml" spelling. mkdocs.yml enables
// pymdownx.superfences (:139) and attr_list (:134), so "{.yaml .annotate}" and
// 'yaml title="inv.yaml"' are idiomatic on this site and render as YAML. An
// anchored pattern skipped them silently -- not validated, not counted, no
// warning -- and markdownlint has MD040 disabled and no rule on info strings, so
// nothing else caught them either. That made the gate teach its own bypass: a
// fragment failing with "undirected YAML fragment" went quiet the moment its
// author added a fence attribute.
//
// The blockquote group takes ANY run of spaces or tabs after each ">", not one.
// A single space matched "> ```yaml" and nothing else: ">  ```yaml" and the
// natural blockquoted-list-item shape ">    ```yaml" both failed the anchored
// match outright and went undiscovered -- silently, which is this gate's worst
// failure mode. Both render as language-yaml on the pinned toolchain.
// blockquotePrefixPattern below has to allow exactly the same runs: an opener this
// pattern admits but that one cannot strip never closes.
var fenceOpenPattern = regexp.MustCompile("^([ \\t]*(?:>[ \\t]*)*)(`{3,}|~{3,})(.*)$")

// blockquotePrefixPattern matches the indentation and blockquote markers a line
// carries before its content.
//
// It MUST stay in step with fenceOpenPattern's first group. It backs both the
// closer (isClosingFence) and stripFencePrefix's fallback, so an opener the wide
// pattern admits and this one cannot strip is a fence that opens and never closes:
// it runs to EOF and takes every later block on the page with it. That is worse
// than the silent miss it replaced, and ">  > ```yaml" produced exactly it while
// this pattern still allowed one space per ">".
var blockquotePrefixPattern = regexp.MustCompile(`^[ \t]*(?:>[ \t]*)*`)

// stripFencePrefix removes a fence line's leading indentation and blockquote
// markers, so a quoted fence yields the same body as an unquoted one.
//
// When the prefix carries no blockquote marker this is plain TrimPrefix, which
// keeps the long-standing behaviour for indented fences byte-identical -- in THIS
// function. The claim does not extend to the closer path; see isClosingFence.
// Only a quoted fence takes the fallback, which is needed because a blank line
// inside a blockquote is written ">" with no trailing space and so does not carry
// the opener's full prefix.
func stripFencePrefix(line, prefix string) string {
	if !strings.Contains(prefix, ">") {
		return strings.TrimPrefix(line, prefix)
	}

	if strings.HasPrefix(line, prefix) {
		return line[len(prefix):]
	}

	return line[len(blockquotePrefixPattern.FindString(line)):]
}

// infoStringLanguage extracts the language token from a fence info string.
//
// Handles the three forms this site can render: a bare language ("yaml"), a
// language carrying Material attributes ('yaml title="inv.yaml"', "yaml{.annotate}"),
// and a pure attribute list ("{.yaml .annotate}", "{ .yaml #id }").
func infoStringLanguage(info string) string {
	info = strings.TrimSpace(info)
	if info == "" {
		return ""
	}

	if strings.HasPrefix(info, "{") {
		for _, field := range strings.Fields(strings.Trim(info, "{}")) {
			if strings.HasPrefix(field, ".") {
				return strings.ToLower(strings.TrimPrefix(field, "."))
			}
		}

		return ""
	}

	language := strings.Fields(info)[0]
	if cut := strings.IndexAny(language, "{,;"); cut >= 0 {
		language = language[:cut]
	}

	return strings.ToLower(language)
}

// isClosingFence reports whether a line closes a fence opened with marker.
// CommonMark requires the same character, at least as many of them, and nothing
// else on the line. The closer has to widen with the opener: admitting a tilde
// fence while still terminating only on backticks would let one "~~~yaml" opener
// swallow the rest of the page into a single block.
func isClosingFence(line, marker string) bool {
	// A closer inside a blockquote carries the same "> " markers as its opener.
	//
	// KNOWN DIVERGENCE, deliberately left in place. The strip is unconditional --
	// this function does not know whether the opener was quoted -- so a body line
	// "> ```" terminates an UNQUOTED fence here, where superfences carries on.
	// Byte-identity with the pre-blockquote behaviour therefore holds only
	// EMPIRICALLY: the sole corpus match is under docs/node_modules, which the walk
	// excludes. Conditioning the strip on the opener's prefix would swap a verified
	// empirical claim for an unverified one about the renderer, so TestIsClosingFence
	// pins the divergence as a fact instead of hiding it.
	line = line[len(blockquotePrefixPattern.FindString(line)):]

	trimmed := strings.TrimSpace(line)
	if len(trimmed) < len(marker) {
		return false
	}

	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != marker[0] {
			return false
		}
	}

	return true
}

// collectDocsYAMLBlocks gathers every YAML example under docs/: fenced blocks in
// Markdown pages and whole .yaml/.yml files.
func collectDocsYAMLBlocks(t *testing.T, root string) []yamlBlock {
	t.Helper()

	var blocks []yamlBlock

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			if _, skipped := skippedDocsDirs[entry.Name()]; skipped || strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}

			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && !isYAMLFile(path) {
			return nil
		}

		rel := filepath.ToSlash(filepath.Join("docs", mustRel(t, root, path)))

		//nolint:gosec // G304: path comes from walking the repo's docs tree.
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("%s: read: %v", rel, readErr)

			return nil
		}

		if ext == ".md" {
			blocks = append(blocks, extractFencedYAML(rel, string(data))...)

			return nil
		}

		blocks = append(blocks, yamlBlock{path: rel, line: 1, body: string(data)})

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return blocks
}

// mustRel returns the path relative to root.
func mustRel(t *testing.T, root, path string) string {
	t.Helper()

	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("relativise %s: %v", path, err)
	}

	return rel
}

// extractFencedYAML pulls every ```yaml block out of a Markdown page, including
// fences indented inside list items -- those are examples too, and an extractor
// that skipped them would go quietly green on five of this repo's pages.
func extractFencedYAML(rel, content string) []yamlBlock {
	lines := strings.Split(content, "\n")

	var blocks []yamlBlock

	for i := 0; i < len(lines); i++ {
		match := fenceOpenPattern.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}

		prefix, marker, info := match[1], match[2], match[3]

		// There is deliberately no "a backtick fence info string may not contain a
		// backtick" guard here. CommonMark has that rule; pymdownx.superfences, which
		// is what actually renders this site, does not -- it lexes
		// ```yaml title="`inv.yaml`" as YAML like any other block. Enforcing the
		// stricter rule skipped a renderable block, and skipping it by `continue`
		// left the walker mid-body so every later block on the page was lost too.
		// Whatever the reason for not collecting a fence, ALWAYS advance past its
		// closer.
		var body []string

		j := i + 1
		for ; j < len(lines); j++ {
			if isClosingFence(lines[j], marker) {
				break
			}

			body = append(body, stripFencePrefix(lines[j], prefix))
		}

		if language := infoStringLanguage(info); language == "yaml" || language == "yml" {
			blocks = append(blocks, yamlBlock{path: rel, line: i + 2, body: strings.Join(body, "\n")})
		}

		// Skip past the whole fence, YAML or not. A fence body is never rescanned for
		// further openers, so a YAML fence quoted inside a Markdown example cannot be
		// mistaken for a real block.
		i = j
	}

	return blocks
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

			validators[schema.GroupVersionKind{
				Group:   internal.Spec.Group,
				Version: version.Name,
				Kind:    internal.Spec.Names.Kind,
			}] = validator
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
func splitYAMLDocuments(t *testing.T, name string, body string) [][]byte {
	t.Helper()

	reader := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader([]byte(body))))

	var docs [][]byte

	for {
		doc, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Errorf("%s: split yaml documents: %v", name, err)

			break
		}

		docs = append(docs, doc)
	}

	return docs
}

// documentClass says whether a YAML document is empty, a complete object, or a fragment.
type documentClass int

const (
	// documentEmpty is a comment-only or blank document -- nothing to validate.
	documentEmpty documentClass = iota
	// documentIdentified carries a parseable apiVersion and kind.
	documentIdentified
	// documentUnidentified has content but no usable apiVersion/kind: a fragment.
	documentUnidentified
)

// classifyDocument reads apiVersion/kind from one YAML document.
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
