// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"fmt"
	"strings"

	"k8s.io/client-go/util/jsonpath"
)

// HasJSONPathFilter reports whether path uses a JSONPath filter expression.
func HasJSONPathFilter(path string) bool {
	return strings.Contains(path, "[?(")
}

// ValidateAttributePath compile-checks CEL expressions or parses JSONPath syntax.
func ValidateAttributePath(extractor *Extractor, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("empty path")
	}

	if !strings.HasPrefix(path, celPrefix) {
		if strings.HasPrefix(path, "object.") || strings.HasPrefix(path, "object[") {
			return fmt.Errorf("CEL expressions must use %q prefix", celPrefix)
		}
	}

	if strings.HasPrefix(path, celPrefix) {
		expr := strings.TrimPrefix(path, celPrefix)
		ast, issues := extractor.celEnv.Compile(strings.TrimSpace(expr))
		if issues != nil && issues.Err() != nil {
			return fmt.Errorf("compile CEL: %w", issues.Err())
		}

		if _, err := extractor.celEnv.Program(ast); err != nil {
			return fmt.Errorf("build CEL program: %w", err)
		}

		return nil
	}

	jp := jsonpath.New("validate")
	if err := jp.Parse(normalizeJSONPath(path)); err != nil {
		return fmt.Errorf("parse JSONPath: %w", err)
	}

	return nil
}
