// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Command integrationevidence runs the integration-tagged tests of the given packages and exits
// zero only when the run proves they ran: every integration test found in the source passed (or,
// in explore mode, at worst skipped), the go test process exited zero, and the result stream was
// complete. See openspec/specs/integration-test-execution/spec.md.
//
//	go run ./hack/tools/integrationevidence [-mode=required|explore] [-json-out=file] <packages>
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("integrationevidence", flag.ContinueOnError)
	flags.SetOutput(stderr)

	mode := flags.String("mode", string(modeRequired),
		"required: every integration test must pass; explore: skips allowed, reported as incomplete")
	jsonOut := flags.String("json-out", "", "also write the raw go test -json stream to this file")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	m := runMode(*mode)
	if m != modeRequired && m != modeExplore {
		complain(stderr, "unknown -mode %q\n", *mode)

		return 2
	}

	patterns := flags.Args()
	if len(patterns) == 0 {
		complain(stderr, "no packages given\n")

		return 2
	}

	wd, err := os.Getwd()
	if err != nil {
		complain(stderr, "%v\n", err)

		return 2
	}

	pkgs, err := listPackages(wd, patterns)
	if err != nil {
		complain(stderr, "%v\n", err)

		return 1
	}

	expected, err := discoverExpected(pkgs)
	if err != nil {
		complain(stderr, "%v\n", err)

		return 1
	}

	// Every integration test in the module must be in the selection, or nothing runs it.
	all, err := listPackages(wd, []string{"./..."})
	if err != nil {
		complain(stderr, "%v\n", err)

		return 1
	}

	allExpected, err := discoverExpected(all)
	if err != nil {
		complain(stderr, "%v\n", err)

		return 1
	}

	selected := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		selected = append(selected, p.ImportPath)
	}

	var results bytes.Buffer

	cmd := testCommand(m, patterns, os.Environ())
	cmd.Stdout = io.MultiWriter(&results, &outputEcho{w: stderr})
	cmd.Stderr = stderr

	code, err := exitCode(cmd.Run())
	if err != nil {
		complain(stderr, "go test did not run: %v\n", err)

		return 2
	}

	if *jsonOut != "" {
		if err := os.WriteFile(*jsonOut, results.Bytes(), 0o600); err != nil {
			complain(stderr, "write %s: %v\n", *jsonOut, err)

			return 1
		}
	}

	r := analyze(analysisInput{
		Expected: expected,
		Outside:  outsideSelection(allExpected, selected),
		Packages: selected,
		Stream:   &results,
		ExitCode: code,
		Mode:     m,
	})
	if _, err := io.WriteString(stdout, r.text()); err != nil {
		return 2
	}

	if !r.OK {
		return 1
	}

	return 0
}

// complain writes one diagnostic line to stderr. A failed write leaves nothing better to do.
func complain(stderr io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(stderr, "integrationevidence: "+format, args...)
}
