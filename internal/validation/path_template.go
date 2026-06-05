// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/konih/kollect/internal/pathvalidate"
)

var (
	allowedPathTemplatePlaceholders = map[string]struct{}{
		"cluster": {}, "namespace": {}, "name": {}, "generation": {}, "extension": {},
	}
	pathTemplatePlaceholderPattern = regexp.MustCompile(`\{([a-z]+)\}`)
)

func ValidatePathTemplate(template string) error {
	template = strings.TrimSpace(template)
	if template == "" {
		return nil
	}
	if err := pathvalidate.RejectTraversal(template); err != nil {
		return fmt.Errorf("pathTemplate: %w", err)
	}
	if !strings.Contains(template, "{namespace}") || !strings.Contains(template, "{name}") {
		return fmt.Errorf("pathTemplate must include {namespace} and {name}")
	}
	for _, match := range pathTemplatePlaceholderPattern.FindAllStringSubmatch(template, -1) {
		if len(match) < 2 {
			continue
		}
		if _, ok := allowedPathTemplatePlaceholders[match[1]]; !ok {
			return fmt.Errorf("pathTemplate contains unsupported placeholder {%s}", match[1])
		}
	}
	return nil
}
