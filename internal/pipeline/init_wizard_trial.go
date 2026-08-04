// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"fmt"
	"path/filepath"
	"strings"
)

// InitTrialLocalOutputDir is the default relative directory suggested for a
// local-directory trial after init (side-effectful collect into files).
const InitTrialLocalOutputDir = "./inventory"

// FormatInitTrialStdoutCmd returns the exact stdout-preview collect command for dir.
// configDir is shell-quoted so paths with spaces stay copy-pasteable.
func FormatInitTrialStdoutCmd(configDir string) string {
	return fmt.Sprintf("kollect-pipeline collect --config %q --output -", configDir)
}

// FormatInitTrialLocalCmd returns the exact local-directory trial collect command for dir.
// configDir is shell-quoted so paths with spaces stay copy-pasteable.
func FormatInitTrialLocalCmd(configDir string) string {
	return fmt.Sprintf(
		"kollect-pipeline collect --config %q --output %s",
		configDir, InitTrialLocalOutputDir,
	)
}

// FormatInitTrialKubectlApplyCmd returns a copyable kubectl apply for dir.
// Init never runs this — it is future operator guidance only.
// configDir is shell-quoted so paths with spaces stay copy-pasteable.
func FormatInitTrialKubectlApplyCmd(configDir string) string {
	return fmt.Sprintf("kubectl apply -f %q", configDir)
}

// FormatInitTrialScreen builds the completion-screen text: exact trial commands plus a
// clearly-marked copyable kubectl apply as future guidance only (ADR-0802 §7).
func FormatInitTrialScreen(configDir string) string {
	var b strings.Builder
	b.WriteString("\nTrial the result\n")
	b.WriteString("----------------\n")
	b.WriteString("Stdout preview (data on stdout, logs on stderr; no files written):\n")
	fmt.Fprintf(&b, "  %s\n\n", FormatInitTrialStdoutCmd(configDir))
	b.WriteString("Local directory trial:\n")
	fmt.Fprintf(&b, "  %s\n\n", FormatInitTrialLocalCmd(configDir))
	b.WriteString("Future guidance only (separate action; requires installed CRDs + authorization).\n")
	b.WriteString("init never applies manifests:\n")
	fmt.Fprintf(&b, "  %s\n", FormatInitTrialKubectlApplyCmd(configDir))
	return b.String()
}

// ValidateInitConfig loads dir through the normal pipeline loader and rejects configs that
// fail profileRef resolution or produce warnings. Used by init after writing YAML so a
// broken generation never looks like a successful completion.
func ValidateInitConfig(dir string) error {
	abs := dir
	if cleaned, err := filepath.Abs(dir); err == nil {
		abs = cleaned
	}
	result, err := LoadConfig(dir)
	if err != nil {
		return fmt.Errorf("generated config in %s failed loader validation: %w", abs, err)
	}
	if len(result.Profiles) == 0 {
		return fmt.Errorf("generated config in %s has no KollectProfile", abs)
	}
	if len(result.Targets) == 0 {
		return fmt.Errorf("generated config in %s has no KollectTarget", abs)
	}
	if len(result.Warnings) > 0 {
		return fmt.Errorf("generated config in %s produced loader warnings: %s",
			abs, strings.Join(result.Warnings, "; "))
	}
	return nil
}
