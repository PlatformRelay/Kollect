// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"errors"
	"io"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

// SurveyPrompter is a survey/v2-backed Prompter for interactive terminals.
type SurveyPrompter struct {
	In     terminal.FileReader
	Out    terminal.FileWriter
	ErrOut io.Writer
	Color  bool
}

// NewSurveyPrompter builds a Prompter bound to the process stdio.
func NewSurveyPrompter(color bool) *SurveyPrompter {
	return &SurveyPrompter{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
		Color:  color,
	}
}

func (s *SurveyPrompter) askOpts() []survey.AskOpt {
	opts := []survey.AskOpt{
		survey.WithStdio(s.In, s.Out, s.ErrOut),
	}
	if !s.Color {
		opts = append(opts, survey.WithIcons(func(icons *survey.IconSet) {
			icons.Question.Text = "?"
			icons.Question.Format = ""
			icons.Help.Format = ""
			icons.Error.Format = ""
			icons.SelectFocus.Format = ""
			icons.MarkedOption.Format = ""
			icons.UnmarkedOption.Format = ""
		}))
	}
	return opts
}

func mapSurveyErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, terminal.InterruptErr) {
		return ErrCanceled
	}
	return err
}

// Input implements Prompter.
func (s *SurveyPrompter) Input(message, defaultValue string, validate ValidateFunc) (string, error) {
	var answer string
	q := &survey.Input{Message: message, Default: defaultValue}
	opts := s.askOpts()
	if validate != nil {
		opts = append(opts, survey.WithValidator(func(ans any) error {
			str, _ := ans.(string)
			return validate(str)
		}))
	}
	err := survey.AskOne(q, &answer, opts...)
	return answer, mapSurveyErr(err)
}

// Select implements Prompter.
func (s *SurveyPrompter) Select(message string, options []string) (string, error) {
	var answer string
	q := &survey.Select{Message: message, Options: options, PageSize: 15}
	err := survey.AskOne(q, &answer, s.askOpts()...)
	return answer, mapSurveyErr(err)
}

// MultiSelect implements Prompter.
func (s *SurveyPrompter) MultiSelect(message string, options []string, defaults []string) ([]string, error) {
	var answer []string
	q := &survey.MultiSelect{Message: message, Options: options, Default: defaults, PageSize: 15}
	err := survey.AskOne(q, &answer, s.askOpts()...)
	return answer, mapSurveyErr(err)
}

// Confirm implements Prompter.
func (s *SurveyPrompter) Confirm(message string, defaultValue bool) (bool, error) {
	var answer bool
	q := &survey.Confirm{Message: message, Default: defaultValue}
	err := survey.AskOne(q, &answer, s.askOpts()...)
	return answer, mapSurveyErr(err)
}
