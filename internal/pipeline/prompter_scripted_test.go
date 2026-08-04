// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"errors"
	"testing"
)

func TestScriptedPrompter_transcriptOrder(t *testing.T) {
	t.Parallel()

	yes := true
	s := NewScriptedPrompter([]PromptAnswer{
		{Confirm: &yes},
		{Select: "a"},
		{MultiSelect: []string{"x"}},
		{Input: "n"},
	})

	ok, err := s.Confirm("c?", true)
	if err != nil || !ok {
		t.Fatalf("Confirm: ok=%v err=%v", ok, err)
	}
	got, err := s.Select("s?", []string{"a", "b"})
	if err != nil || got != "a" {
		t.Fatalf("Select: got=%q err=%v", got, err)
	}
	ms, err := s.MultiSelect("m?", []string{"x", "y"}, nil)
	if err != nil || len(ms) != 1 || ms[0] != "x" {
		t.Fatalf("MultiSelect: got=%v err=%v", ms, err)
	}
	in, err := s.Input("i?", "def", nil)
	if err != nil || in != "n" {
		t.Fatalf("Input: got=%q err=%v", in, err)
	}

	prompts := s.Prompts()
	want := []string{"confirm:c?", "select:s?", "multiselect:m?", "input:i?"}
	if len(prompts) != len(want) {
		t.Fatalf("prompts=%v want=%v", prompts, want)
	}
	for i := range want {
		if prompts[i] != want[i] {
			t.Errorf("prompts[%d]=%q want %q", i, prompts[i], want[i])
		}
	}
}

func TestScriptedPrompter_exhaustionError(t *testing.T) {
	t.Parallel()

	s := NewScriptedPrompter(nil)
	_, err := s.Input("x", "", nil)
	if err == nil {
		t.Fatal("expected exhaustion error")
	}
}

func TestScriptedPrompter_propagatesErr(t *testing.T) {
	t.Parallel()

	s := NewScriptedPrompter([]PromptAnswer{{Err: ErrCanceled}})
	_, err := s.Confirm("c?", true)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("got %v", err)
	}
}
