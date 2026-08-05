// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"errors"
	"os"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

func TestMapSurveyErr(t *testing.T) {
	t.Parallel()

	if err := mapSurveyErr(nil); err != nil {
		t.Fatalf("nil: got %v", err)
	}
	if err := mapSurveyErr(terminal.InterruptErr); !errors.Is(err, ErrCanceled) {
		t.Fatalf("InterruptErr: got %v want ErrCanceled", err)
	}
	sentinel := errors.New("boom")
	if err := mapSurveyErr(sentinel); !errors.Is(err, sentinel) {
		t.Fatalf("passthrough: got %v", err)
	}
}

func TestNewSurveyPrompter_wiresStdioAndColor(t *testing.T) {
	t.Parallel()

	s := NewSurveyPrompter(true)
	if s.In != os.Stdin || s.Out != os.Stdout || s.ErrOut != os.Stderr {
		t.Fatalf("stdio: In=%v Out=%v ErrOut=%v", s.In, s.Out, s.ErrOut)
	}
	if !s.Color {
		t.Fatal("Color=true not wired")
	}
	s2 := NewSurveyPrompter(false)
	if s2.Color {
		t.Fatal("Color=false not wired")
	}
}

func TestAskOpts_colorTrueVsFalse(t *testing.T) {
	t.Parallel()

	withColor := &SurveyPrompter{Color: true, In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	noColor := &SurveyPrompter{Color: false, In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	cOpts := withColor.askOpts()
	nOpts := noColor.askOpts()
	if len(cOpts) != 1 {
		t.Fatalf("color=true: got %d ask opts want 1 (stdio only)", len(cOpts))
	}
	if len(nOpts) != 2 {
		t.Fatalf("color=false: got %d ask opts want 2 (stdio + WithIcons)", len(nOpts))
	}

	var applied survey.AskOptions
	for _, o := range nOpts {
		if err := o(&applied); err != nil {
			t.Fatalf("apply no-color AskOpt: %v", err)
		}
	}
	if applied.PromptConfig.Icons.Question.Text != "?" {
		t.Fatalf("WithIcons Question.Text=%q want ?", applied.PromptConfig.Icons.Question.Text)
	}
	if applied.PromptConfig.Icons.Question.Format != "" {
		t.Fatalf("WithIcons Question.Format=%q want empty", applied.PromptConfig.Icons.Question.Format)
	}
}

func TestSurveyPrompter_Input_Select_MultiSelect_Confirm(t *testing.T) {
	t.Parallel()

	var sawPrompt survey.Prompt
	var sawOpts int
	s := &SurveyPrompter{
		Color:  true,
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
		askOne: func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			sawPrompt = p
			sawOpts = len(opts)
			switch dest := response.(type) {
			case *string:
				*dest = "picked"
			case *[]string:
				*dest = []string{"a", "b"}
			case *bool:
				*dest = true
			default:
				t.Fatalf("unexpected response type %T", response)
			}
			return nil
		},
	}

	in, err := s.Input("msg", "def", nil)
	if err != nil || in != "picked" {
		t.Fatalf("Input: got %q err=%v", in, err)
	}
	if _, ok := sawPrompt.(*survey.Input); !ok {
		t.Fatalf("Input prompt type %T", sawPrompt)
	}
	if sawOpts != 1 {
		t.Fatalf("Input opts=%d want 1", sawOpts)
	}

	sel, err := s.Select("pick", []string{"x", "y"})
	if err != nil || sel != "picked" {
		t.Fatalf("Select: got %q err=%v", sel, err)
	}
	if q, ok := sawPrompt.(*survey.Select); !ok || q.PageSize != 15 {
		t.Fatalf("Select prompt %#v", sawPrompt)
	}

	ms, err := s.MultiSelect("multi", []string{"a", "b", "c"}, []string{"a"})
	if err != nil || len(ms) != 2 || ms[0] != "a" || ms[1] != "b" {
		t.Fatalf("MultiSelect: got %v err=%v", ms, err)
	}
	if q, ok := sawPrompt.(*survey.MultiSelect); !ok || q.PageSize != 15 {
		t.Fatalf("MultiSelect prompt %#v", sawPrompt)
	}

	ok, err := s.Confirm("sure?", false)
	if err != nil || !ok {
		t.Fatalf("Confirm: ok=%v err=%v", ok, err)
	}
	if _, is := sawPrompt.(*survey.Confirm); !is {
		t.Fatalf("Confirm prompt %T", sawPrompt)
	}
}

func TestSurveyPrompter_Input_withValidatorAndInterrupt(t *testing.T) {
	t.Parallel()

	called := false
	s := &SurveyPrompter{
		Color:  false,
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
		askOne: func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			if len(opts) != 3 { // stdio + icons + validator
				t.Fatalf("opts=%d want 3", len(opts))
			}
			var ao survey.AskOptions
			for _, o := range opts {
				if err := o(&ao); err != nil {
					t.Fatalf("apply AskOpt: %v", err)
				}
			}
			for _, v := range ao.Validators {
				if err := v("ok"); err != nil {
					t.Fatalf("validator: %v", err)
				}
				called = true
			}
			return terminal.InterruptErr
		},
	}
	_, err := s.Input("n", "", func(answer string) error {
		if answer != "ok" {
			return errors.New("bad")
		}
		return nil
	})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("got %v want ErrCanceled", err)
	}
	if !called {
		t.Fatal("expected validator AskOpt to be invoked")
	}
}

func TestSurveyPrompter_askOneDefaultsToSurvey(t *testing.T) {
	t.Parallel()

	s := NewSurveyPrompter(false)
	if s.askOne != nil {
		t.Fatal("NewSurveyPrompter should leave askOne nil (default)")
	}
	fn := s.resolveAskOne()
	if fn == nil {
		t.Fatal("resolveAskOne returned nil")
	}
}

func TestSurveyPrompter_propagatesAskError(t *testing.T) {
	t.Parallel()

	boom := errors.New("ask failed")
	s := &SurveyPrompter{
		Color: true,
		askOne: func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			return boom
		},
	}
	if _, err := s.Select("x", []string{"a"}); !errors.Is(err, boom) {
		t.Fatalf("Select: %v", err)
	}
	if _, err := s.MultiSelect("x", []string{"a"}, nil); !errors.Is(err, boom) {
		t.Fatalf("MultiSelect: %v", err)
	}
	if _, err := s.Confirm("x", true); !errors.Is(err, boom) {
		t.Fatalf("Confirm: %v", err)
	}
}
