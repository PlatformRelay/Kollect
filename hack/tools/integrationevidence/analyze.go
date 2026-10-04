// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Final actions of a go test -json event.
const (
	actionPass = "pass"
	actionFail = "fail"
	actionSkip = "skip"
)

// maxOutputLines caps the output kept per test, package or build. The last lines are kept,
// because that is where a failure message is.
const maxOutputLines = 40

// event is the subset of a go test -json line the analysis reads.
type event struct {
	Action     string
	Package    string
	ImportPath string
	Test       string
	Output     string
}

// report is the verdict on one run. OK decides the exit status; Complete says whether every
// expected test passed, which is what "integration verified" means.
type report struct {
	Mode            runMode
	Expected        int
	Executed        int
	Passed          int
	Failed          []string
	Skipped         []string
	SkippedSubtests []string
	Missing         []string
	// Outside lists integration tests in packages the run did not select: nothing ran them.
	Outside         []string
	UntaggedPassed  int
	UntaggedSkipped []string
	Problems        []string
	Complete        bool
	OK              bool
	// Explanations holds the output to show under the report, keyed by what it explains.
	Explanations map[string][]string
}

// streamState collects results by package and test, never by order: event order across packages
// differs between Go versions. A test name can run more than once (internal and external test
// package), so every result is kept and the worst one counts.
type streamState struct {
	started     map[string]bool
	pkgResult   map[string]string
	pkgOutput   map[string][]string
	runs        map[testID]int
	results     map[testID][]string
	output      map[testID][]string
	buildOutput map[string][]string
	buildFailed []string
	malformed   []int
}

func readStream(in analysisInput) (*streamState, error) {
	st := &streamState{
		started:     map[string]bool{},
		pkgResult:   map[string]string{},
		pkgOutput:   map[string][]string{},
		runs:        map[testID]int{},
		results:     map[testID][]string{},
		output:      map[testID][]string{},
		buildOutput: map[string][]string{},
	}

	scanner := bufio.NewScanner(in.Stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := scanner.Bytes()
		if strings.TrimSpace(string(line)) == "" {
			continue
		}

		var ev event
		if err := json.Unmarshal(line, &ev); err != nil || ev.Action == "" {
			st.malformed = append(st.malformed, lineNo)

			continue
		}

		st.record(ev)
	}

	return st, scanner.Err()
}

// keepLast appends line and keeps only the last maxOutputLines lines.
func keepLast(lines []string, line string) []string {
	lines = append(lines, strings.TrimRight(line, "\n"))
	if len(lines) > maxOutputLines {
		lines = lines[len(lines)-maxOutputLines:]
	}

	return lines
}

func (st *streamState) record(ev event) {
	switch ev.Action {
	case "build-output":
		st.buildOutput[ev.ImportPath] = keepLast(st.buildOutput[ev.ImportPath], ev.Output)

		return
	case "build-fail":
		st.buildFailed = append(st.buildFailed, ev.ImportPath)

		return
	}

	if ev.Test == "" {
		switch ev.Action {
		case "start":
			st.started[ev.Package] = true
		case actionPass, actionFail, actionSkip:
			st.pkgResult[ev.Package] = ev.Action
		case "output":
			st.pkgOutput[ev.Package] = keepLast(st.pkgOutput[ev.Package], ev.Output)
		}

		return
	}

	id := testID{Package: ev.Package, Test: ev.Test}

	switch ev.Action {
	case "run":
		st.runs[id]++
	case actionPass, actionFail, actionSkip:
		st.results[id] = append(st.results[id], ev.Action)
	case "output":
		st.output[id] = keepLast(st.output[id], ev.Output)
	}
}

// worst returns fail over skip over pass, or "" for no result.
func worst(results []string) string {
	w := ""

	for _, r := range results {
		switch {
		case r == actionFail:
			return actionFail
		case r == actionSkip:
			w = actionSkip
		case w == "":
			w = r
		}
	}

	return w
}

// analyze decides whether a run proves its expected integration tests ran and passed.
func analyze(in analysisInput) report {
	r := report{Mode: in.Mode, Explanations: map[string][]string{}}
	for _, n := range in.Expected {
		r.Expected += n
	}

	st, err := readStream(in)
	if err != nil {
		r.Problems = append(r.Problems, fmt.Sprintf("result stream unreadable: %v", err))
	}

	if in.ExitCode != 0 {
		r.Problems = append(r.Problems, fmt.Sprintf("go test exited %d", in.ExitCode))
	}

	if len(in.Expected) == 0 {
		r.Problems = append(r.Problems, "no integration tests found in the selected packages")
	}

	r.Outside = in.Outside

	for _, n := range st.malformed {
		r.Problems = append(r.Problems, fmt.Sprintf("malformed result line %d", n))
	}

	for _, p := range st.buildFailed {
		r.Problems = append(r.Problems, "build failed: "+p)
		r.Explanations["build "+p] = st.buildOutput[p]
	}

	r.Problems = append(r.Problems, st.truncation(in.Packages)...)

	st.classify(in.Expected, &r)
	st.packageFailures(&r)

	r.Complete = len(r.Problems) == 0 && len(r.Failed) == 0 && len(r.Missing) == 0 &&
		len(r.Skipped) == 0 && len(r.SkippedSubtests) == 0 && len(r.Outside) == 0
	if in.Mode == modeExplore {
		r.OK = len(r.Problems) == 0 && len(r.Failed) == 0 && len(r.Missing) == 0
	} else {
		r.OK = r.Complete
	}

	return r
}

// truncation names every started package or test without a final result, and every selected
// package that never reported.
func (st *streamState) truncation(selected []string) []string {
	var problems []string

	for _, p := range sortedKeys(st.started) {
		if st.pkgResult[p] == "" {
			problems = append(problems, "truncated: package "+p+" has no final result")
		}
	}

	for _, p := range selected {
		if !st.started[p] && st.pkgResult[p] == "" {
			problems = append(problems, "package "+p+" reported no result")
		}
	}

	var open, orphaned []string

	for id, runs := range st.runs {
		if len(st.results[id]) < runs {
			open = append(open, id.String())
		}
	}

	// test2json always reports a run before a result; a result without one is not a real stream.
	for id, results := range st.results {
		if len(results) > st.runs[id] {
			orphaned = append(orphaned, id.String())
		}
	}

	sort.Strings(open)
	sort.Strings(orphaned)

	for _, name := range open {
		problems = append(problems, "truncated: test "+name+" has no final result")
	}

	for _, name := range orphaned {
		problems = append(problems, "malformed: test "+name+" reported a result without a run event")
	}

	return problems
}

func (st *streamState) classify(expected map[testID]int, r *report) {
	for id, declared := range expected {
		results := st.results[id]

		// Counts are per declaration, so expected = executed = passed on a complete run even when a
		// name is declared in both the internal and the external test package.
		switch w := worst(results); {
		case w == actionFail:
			r.Executed += len(results)
		case w == actionSkip:
			r.Skipped = append(r.Skipped, id.String())
		case len(results) < declared:
			r.Missing = append(r.Missing, fmt.Sprintf("%s (%d of %d declarations reported)", id, len(results), declared))
		default:
			r.Executed += len(results)
			r.Passed += len(results)
		}
	}

	for id, results := range st.results {
		w := worst(results)
		top, _, isSubtest := strings.Cut(id.Test, "/")

		switch {
		case w == actionFail:
			r.Failed = append(r.Failed, id.String())
			r.Explanations[id.String()] = st.output[id]
		case isSubtest && w == actionSkip && expected[testID{Package: id.Package, Test: top}] > 0:
			r.SkippedSubtests = append(r.SkippedSubtests, id.String())
		case isSubtest || expected[id] > 0:
			// Expected tests are counted above; other subtests are summarised by their parent.
		case w == actionPass:
			r.UntaggedPassed++
		case w == actionSkip:
			r.UntaggedSkipped = append(r.UntaggedSkipped, id.String())
		}
	}

	sort.Strings(r.Failed)
	sort.Strings(r.Skipped)
	sort.Strings(r.SkippedSubtests)
	sort.Strings(r.Missing)
	sort.Strings(r.UntaggedSkipped)
}

// packageFailures names every failed package. Its own output is shown when no failed test in it
// already explains the failure (a TestMain panic, a race outside a test).
func (st *streamState) packageFailures(r *report) {
	for _, p := range sortedKeys(st.started) {
		if st.pkgResult[p] != actionFail {
			continue
		}

		r.Problems = append(r.Problems, "package "+p+" failed")

		if !hasFailedTestIn(r.Failed, p) {
			r.Explanations["package "+p] = st.pkgOutput[p]
		}
	}
}

func hasFailedTestIn(failed []string, pkg string) bool {
	for _, name := range failed {
		if strings.HasPrefix(name, pkg+".") {
			return true
		}
	}

	return false
}

// text renders the report for a CI log: counts first, then each list with its names, then the
// output that explains each failure.
func (r report) text() string {
	var b strings.Builder

	fmt.Fprintf(&b, "integration evidence (%s): expected %d, executed %d, passed %d, failed %d, skipped %d, missing %d\n",
		r.Mode, r.Expected, r.Executed, r.Passed, len(r.Failed), len(r.Skipped), len(r.Missing))
	fmt.Fprintf(&b, "untagged tests in the same packages: passed %d, skipped %d\n",
		r.UntaggedPassed, len(r.UntaggedSkipped))

	section := func(title string, names []string) {
		if len(names) == 0 {
			return
		}

		fmt.Fprintf(&b, "%s:\n", title)

		for _, n := range names {
			fmt.Fprintf(&b, "  %s\n", n)
		}
	}

	section("problems", r.Problems)
	section("failed", r.Failed)
	section("skipped (expected integration tests)", r.Skipped)
	section("skipped subtests of expected tests", r.SkippedSubtests)
	section("missing (expected, no result)", r.Missing)
	section("untagged skips (not required)", r.UntaggedSkipped)
	section("integration tests outside the selected packages (never run)", r.Outside)

	for _, key := range sortedKeys(r.Explanations) {
		out := r.Explanations[key]
		if len(out) == 0 {
			continue
		}

		fmt.Fprintf(&b, "output of %s (last %d lines at most):\n", key, maxOutputLines)

		for _, line := range out {
			fmt.Fprintf(&b, "  | %s\n", line)
		}
	}

	switch {
	case r.Complete:
		b.WriteString("RESULT: PASS - every expected integration test ran and passed\n")
	case r.OK:
		b.WriteString("RESULT: INCOMPLETE - NOT a completed integration verification (explore mode)\n")
	default:
		b.WriteString("RESULT: FAIL\n")
	}

	return b.String()
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
