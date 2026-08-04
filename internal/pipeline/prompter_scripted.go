// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import "fmt"

// PromptAnswer is one scripted response consumed by ScriptedPrompter in call order.
// Exactly one field should be set per answer.
type PromptAnswer struct {
	Input       string
	Select      string
	MultiSelect []string
	Confirm     *bool
	Err         error
}

// ScriptedPrompter is a deterministic Prompter for hermetic wizard transcript tests.
type ScriptedPrompter struct {
	answers []PromptAnswer
	idx     int
	prompts []string
}

// NewScriptedPrompter returns a Prompter that yields answers in order.
func NewScriptedPrompter(answers []PromptAnswer) *ScriptedPrompter {
	return &ScriptedPrompter{answers: answers}
}

// Prompts returns the recorded prompt labels in call order (prefixed by kind).
func (s *ScriptedPrompter) Prompts() []string {
	out := make([]string, len(s.prompts))
	copy(out, s.prompts)
	return out
}

func (s *ScriptedPrompter) next(kind, message string) (PromptAnswer, error) {
	s.prompts = append(s.prompts, kind+":"+message)
	if s.idx >= len(s.answers) {
		return PromptAnswer{}, fmt.Errorf("scripted prompter: no answer left for %s %q", kind, message)
	}
	a := s.answers[s.idx]
	s.idx++
	if a.Err != nil {
		return PromptAnswer{}, a.Err
	}
	return a, nil
}

// Input implements Prompter.
func (s *ScriptedPrompter) Input(message, defaultValue string, validate ValidateFunc) (string, error) {
	a, err := s.next("input", message)
	if err != nil {
		return "", err
	}
	val := a.Input
	if val == "" {
		val = defaultValue
	}
	if validate != nil {
		if err := validate(val); err != nil {
			return "", err
		}
	}
	return val, nil
}

// Select implements Prompter.
func (s *ScriptedPrompter) Select(message string, _ []string) (string, error) {
	a, err := s.next("select", message)
	if err != nil {
		return "", err
	}
	return a.Select, nil
}

// MultiSelect implements Prompter.
func (s *ScriptedPrompter) MultiSelect(message string, _ []string, defaults []string) ([]string, error) {
	a, err := s.next("multiselect", message)
	if err != nil {
		return nil, err
	}
	if a.MultiSelect == nil {
		return append([]string(nil), defaults...), nil
	}
	return append([]string(nil), a.MultiSelect...), nil
}

// Confirm implements Prompter.
func (s *ScriptedPrompter) Confirm(message string, defaultValue bool) (bool, error) {
	a, err := s.next("confirm", message)
	if err != nil {
		return false, err
	}
	if a.Confirm == nil {
		return defaultValue, nil
	}
	return *a.Confirm, nil
}
