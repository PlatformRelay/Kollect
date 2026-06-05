// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"fmt"
	"strings"

	"k8s.io/client-go/util/jsonpath"
)

// ValidateAttributePath compile-checks CEL expressions or parses JSONPath syntax.
func ValidateAttributePath(extractor *Extractor, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("empty path")
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
