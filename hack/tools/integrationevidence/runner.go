// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
)

// testCommand builds the go test invocation. Required mode appends KOLLECT_REQUIRE_DOCKER=true
// last, so it overrides any value inherited from env: an unusable Docker must fail tests.
func testCommand(mode runMode, patterns, env []string) *exec.Cmd {
	args := append([]string{"test", "-json", "-count=1", "-tags=" + integrationTag}, patterns...)

	cmd := exec.Command("go", args...) //nolint:gosec // fixed binary; patterns come from the Taskfile
	cmd.Env = append([]string{}, env...)

	if mode == modeRequired {
		cmd.Env = append(cmd.Env, requireDockerEnv+"=true")
	}

	return cmd
}

// exitCode turns the result of running a command into its exit status. An error that is not an
// exit status (the command never started) is returned as an error, never as a status.
func exitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}

	return 0, err
}

// outputEcho turns the go test -json stream into the progress plain go test prints, as it
// arrives, so a CI log shows progress even when the job is cancelled before the report: package
// summaries, build output, and any line that is not an event. Per-test output is left to the
// report, which shows it for failed tests.
type outputEcho struct {
	w       io.Writer
	pending []byte
}

func (e *outputEcho) Write(p []byte) (int, error) {
	e.pending = append(e.pending, p...)

	for {
		i := bytes.IndexByte(e.pending, '\n')
		if i < 0 {
			return len(p), nil
		}

		line := e.pending[:i+1]
		e.pending = e.pending[i+1:]

		var ev event
		if err := json.Unmarshal(line, &ev); err == nil && ev.Action != "" {
			if ev.Test != "" {
				continue
			}

			line = []byte(ev.Output)
		}

		if _, err := e.w.Write(line); err != nil {
			return len(p), err
		}
	}
}
