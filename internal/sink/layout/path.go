// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package layout

import (
	"fmt"
	"regexp"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

// LayoutPlaceholders are the placeholders allowed in layout.pathTemplate (ADR-0419).
var LayoutPlaceholders = map[string]struct{}{
	"cluster": {}, "namespace": {}, "name": {}, "targetNamespace": {}, "targetName": {},
	"sourceNamespace": {}, "sourceName": {}, "group": {}, "kind": {}, "uid": {},
	"generation": {}, "extension": {},
}

var (
	layoutPlaceholderPattern = regexp.MustCompile(`\{([a-zA-Z]+)\}`)
	unsafeSegmentChars       = regexp.MustCompile(`[/\\:]+`)
	collapseDots             = regexp.MustCompile(`\.\.+`)
)

// renderItemPath renders the per-resource path for an item under a resolved layout.
func renderItemPath(item collect.Item, r ResolvedLayout) (string, error) {
	cluster := r.Cluster
	if strings.TrimSpace(cluster) == "" {
		cluster = "default"
	}

	kind := item.Kind
	if r.Filename.LowercaseKindEnabled() {
		kind = strings.ToLower(kind)
	}

	group := groupSegment(item.Group, r.Filename.GroupInPathOrDefault())

	maxLen := int(r.Filename.MaxSegmentLengthOrDefault())
	repl := map[string]string{
		"cluster":         sanitizeSegment(cluster, maxLen),
		"namespace":       sanitizeSegment(r.InventoryNamespace, maxLen),
		"name":            sanitizeSegment(r.InventoryName, maxLen),
		"targetNamespace": sanitizeSegment(item.TargetNamespace, maxLen),
		"targetName":      sanitizeSegment(item.TargetName, maxLen),
		"sourceNamespace": sanitizeSegment(item.Namespace, maxLen),
		"sourceName":      sanitizeSegment(item.Name, maxLen),
		"group":           sanitizeSegment(group, maxLen),
		"kind":            sanitizeSegment(kind, maxLen),
		"uid":             sanitizeSegment(item.UID, maxLen),
		"generation":      fmt.Sprintf("%d", r.Generation),
		"extension":       r.Extension,
	}

	rendered := layoutPlaceholderPattern.ReplaceAllStringFunc(r.PathTemplate, func(match string) string {
		key := strings.Trim(match, "{}")
		if v, ok := repl[key]; ok {
			return v
		}

		return match
	})

	return cleanRenderedPath(rendered)
}

// groupSegment resolves the {group} value for the configured policy.
func groupSegment(group, policy string) string {
	group = strings.TrimSpace(group)
	switch policy {
	case kollectdevv1alpha1.LayoutGroupInPathNever:
		return ""
	default: // auto (omit when empty via cleanRenderedPath) and always
		return group
	}
}

// cleanRenderedPath drops empty segments (e.g. omitted {group}) and rejects traversal.
func cleanRenderedPath(path string) (string, error) {
	path = strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")

	segments := strings.Split(path, "/")
	kept := make([]string, 0, len(segments))
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if seg == "." || seg == ".." {
			return "", fmt.Errorf("layout path %q contains traversal segment", path)
		}
		kept = append(kept, seg)
	}

	if len(kept) == 0 {
		return "", fmt.Errorf("layout path rendered empty")
	}

	return strings.Join(kept, "/"), nil
}

// sanitizeSegment strips path-unsafe characters and caps the segment length (ADR-0419 §Filename safety).
func sanitizeSegment(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	value = unsafeSegmentChars.ReplaceAllString(value, "-")
	value = collapseDots.ReplaceAllString(value, "-")
	value = strings.Trim(value, ".-")
	if value == "" {
		return ""
	}

	if maxLen > 0 && len(value) > maxLen {
		value = strings.Trim(value[:maxLen], ".-")
	}

	return value
}

// ValidateLayoutPathTemplate checks a layout.pathTemplate's placeholders and shape (ADR-0419).
func ValidateLayoutPathTemplate(template string) error {
	template = strings.TrimSpace(template)
	if template == "" {
		return nil
	}

	if strings.Contains(template, "..") {
		return fmt.Errorf("layout.pathTemplate must not contain '..'")
	}

	for _, match := range layoutPlaceholderPattern.FindAllStringSubmatch(template, -1) {
		if len(match) < 2 {
			continue
		}
		if _, ok := LayoutPlaceholders[match[1]]; !ok {
			return fmt.Errorf("layout.pathTemplate contains unsupported placeholder {%s}", match[1])
		}
	}

	if !strings.Contains(template, "{sourceName}") && !strings.Contains(template, "{name}") &&
		!strings.Contains(template, "{uid}") {
		return fmt.Errorf("layout.pathTemplate must include a per-resource identifier ({sourceName}, {name}, or {uid})")
	}

	return nil
}
