// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import "errors"

// ErrCanceled is returned when the operator declines a confirming step or interrupts
// the terminal prompt (Ctrl-C) during init.
var ErrCanceled = errors.New("wizard canceled")

// ValidateFunc optionally rejects an Input answer.
type ValidateFunc func(answer string) error

// Prompter is the thin injectable prompt surface for the init wizard (ADR-0802 /
// PIPE-SPIKE-01). The wizard depends on this interface, not on survey directly.
type Prompter interface {
	Input(message, defaultValue string, validate ValidateFunc) (string, error)
	Select(message string, options []string) (string, error)
	MultiSelect(message string, options []string, defaults []string) ([]string, error)
	Confirm(message string, defaultValue bool) (bool, error)
}
