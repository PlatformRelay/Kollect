// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const (
	pkgA = "example.com/m/a"
	pkgB = "example.com/m/b"
)

// stream builds a go test -json stream; each step is action, package, test[, output].
func stream(t *testing.T, steps ...[]string) string {
	t.Helper()

	var buf bytes.Buffer

	for _, step := range steps {
		ev := map[string]string{"Action": step[0], "Package": step[1]}
		if len(step) > 2 && step[2] != "" {
			ev["Test"] = step[2]
		}

		if len(step) > 3 {
			ev["Output"] = step[3]
		}

		line, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}

		buf.Write(line)
		buf.WriteByte('\n')
	}

	return buf.String()
}

// passed is the run/pass pair for one test.
func passed(pkg, test string) [][]string {
	return [][]string{{"run", pkg, test}, {"pass", pkg, test}}
}

func steps(parts ...[][]string) [][]string {
	var out [][]string
	for _, p := range parts {
		out = append(out, p...)
	}

	return out
}

var twoExpected = map[testID]int{{pkgA, "TestOne"}: 1, {pkgB, "TestTwo"}: 1}

// allPass is a complete, passing run of both expected tests.
func allPass(t *testing.T) string {
	return stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		passed(pkgB, "TestTwo"),
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)
}

func analyzeRun(t *testing.T, mode runMode, exit int, s string) report {
	t.Helper()

	return analyze(analysisInput{
		Expected: twoExpected,
		Packages: []string{pkgA, pkgB},
		Stream:   strings.NewReader(s),
		ExitCode: exit,
		Mode:     mode,
	})
}

func requireFailure(t *testing.T, r report, mention string) {
	t.Helper()

	if r.OK {
		t.Fatalf("run reported OK; want failure mentioning %q\n%s", mention, r.text())
	}

	if !strings.Contains(r.text(), mention) {
		t.Fatalf("report does not mention %q:\n%s", mention, r.text())
	}
}

func TestAnalyze_allExpectedPass(t *testing.T) {
	r := analyzeRun(t, modeRequired, 0, allPass(t))
	if !r.OK || !r.Complete {
		t.Fatalf("complete passing run not accepted:\n%s", r.text())
	}

	if r.Expected != 2 || r.Executed != 2 || r.Passed != 2 {
		t.Fatalf("counts expected/executed/passed = %d/%d/%d, want 2/2/2", r.Expected, r.Executed, r.Passed)
	}
}

func TestAnalyze_expectedTestSkippedFailsRequired(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		[][]string{{"run", pkgB, "TestTwo"}, {"skip", pkgB, "TestTwo"}},
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)
	requireFailure(t, analyzeRun(t, modeRequired, 0, s), pkgB+".TestTwo")
}

func TestAnalyze_allExpectedSkippedFailsRequired(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		[][]string{{"run", pkgA, "TestOne"}, {"skip", pkgA, "TestOne"}},
		[][]string{{"run", pkgB, "TestTwo"}, {"skip", pkgB, "TestTwo"}},
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)

	r := analyzeRun(t, modeRequired, 0, s)
	requireFailure(t, r, "skipped")

	if r.Passed != 0 || len(r.Skipped) != 2 {
		t.Fatalf("passed=%d skipped=%d, want 0 and 2", r.Passed, len(r.Skipped))
	}
}

func TestAnalyze_missingExpectedTestAmongPasses(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)

	r := analyzeRun(t, modeRequired, 0, s)
	requireFailure(t, r, pkgB+".TestTwo")

	if len(r.Missing) != 1 {
		t.Fatalf("missing = %q, want exactly TestTwo", r.Missing)
	}
}

func TestAnalyze_failedSubtestFails(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		[][]string{
			{"run", pkgB, "TestTwo"}, {"run", pkgB, "TestTwo/sub"},
			{"output", pkgB, "TestTwo/sub", "    boom: the assertion\n"},
			{"fail", pkgB, "TestTwo/sub"}, {"fail", pkgB, "TestTwo"},
		},
		[][]string{{"pass", pkgA}, {"fail", pkgB}},
	)...)

	r := analyzeRun(t, modeRequired, 1, s)
	requireFailure(t, r, pkgB+".TestTwo/sub")

	if !strings.Contains(r.text(), "boom: the assertion") {
		t.Fatalf("report does not show the failing output:\n%s", r.text())
	}
}

func TestAnalyze_zeroSelectedFails(t *testing.T) {
	s := stream(t, []string{"start", pkgA}, []string{"start", pkgB},
		[]string{"output", pkgA, "", "testing: warning: no tests to run\n"},
		[]string{"pass", pkgA}, []string{"pass", pkgB})

	r := analyzeRun(t, modeRequired, 0, s)
	requireFailure(t, r, "missing")

	if r.Executed != 0 || len(r.Missing) != 2 {
		t.Fatalf("executed=%d missing=%d, want 0 and 2", r.Executed, len(r.Missing))
	}
}

func TestAnalyze_noExpectedTestsFails(t *testing.T) {
	r := analyze(analysisInput{
		Expected: map[testID]int{},
		Packages: []string{pkgA},
		Stream:   strings.NewReader(stream(t, []string{"start", pkgA}, []string{"pass", pkgA})),
		Mode:     modeRequired,
	})
	requireFailure(t, r, "no integration tests")
}

func TestAnalyze_nonZeroExitIsNeverMasked(t *testing.T) {
	for _, mode := range []runMode{modeRequired, modeExplore} {
		requireFailure(t, analyzeRun(t, mode, 2, allPass(t)), "exited 2")
	}
}

func TestAnalyze_malformedLineFails(t *testing.T) {
	s := allPass(t) + "panic: this is not json\n"
	requireFailure(t, analyzeRun(t, modeRequired, 0, s), "malformed")
}

func TestAnalyze_truncatedTestFails(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		[][]string{{"run", pkgB, "TestTwo"}},
		[][]string{{"pass", pkgA}},
	)...)
	requireFailure(t, analyzeRun(t, modeRequired, 0, s), "truncated")
}

func TestAnalyze_truncatedPackageFails(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		passed(pkgB, "TestTwo"),
		[][]string{{"pass", pkgA}},
	)...)
	requireFailure(t, analyzeRun(t, modeRequired, 0, s), "truncated")
}

func TestAnalyze_unreportedPackageFails(t *testing.T) {
	r := analyze(analysisInput{
		Expected: twoExpected,
		Packages: []string{pkgA, pkgB, "example.com/m/c"},
		Stream:   strings.NewReader(allPass(t)),
		Mode:     modeRequired,
	})
	requireFailure(t, r, "example.com/m/c")
}

func TestAnalyze_buildFailureFails(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}},
		passed(pkgA, "TestOne"),
		[][]string{{"pass", pkgA}, {"start", pkgB}, {"fail", pkgB}},
	)...)
	s = `{"ImportPath":"example.com/m/b [example.com/m/b.test]","Action":"build-fail"}` + "\n" + s

	r := analyzeRun(t, modeRequired, 1, s)
	requireFailure(t, r, "build failed")
	requireFailure(t, r, pkgB+".TestTwo")
}

func TestAnalyze_untaggedSkipIsReportedNotFailed(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		passed(pkgB, "TestTwo"),
		[][]string{{"run", pkgB, "TestEnvtest"}, {"skip", pkgB, "TestEnvtest"}},
		passed(pkgB, "TestUnit"),
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)

	r := analyzeRun(t, modeRequired, 0, s)
	if !r.OK {
		t.Fatalf("an untagged skip failed the run:\n%s", r.text())
	}

	if r.UntaggedPassed != 1 || len(r.UntaggedSkipped) != 1 {
		t.Fatalf("untagged passed=%d skipped=%q, want 1 and [TestEnvtest]", r.UntaggedPassed, r.UntaggedSkipped)
	}

	if !strings.Contains(r.text(), pkgB+".TestEnvtest") {
		t.Fatalf("report does not list the untagged skip:\n%s", r.text())
	}
}

func TestAnalyze_untaggedFailureFails(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		passed(pkgB, "TestTwo"),
		[][]string{{"run", pkgB, "TestUnit"}, {"fail", pkgB, "TestUnit"}},
		[][]string{{"pass", pkgA}, {"fail", pkgB}},
	)...)
	requireFailure(t, analyzeRun(t, modeRequired, 1, s), pkgB+".TestUnit")
}

func TestAnalyze_exploreSkipIsIncompleteButNotFailed(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		[][]string{{"run", pkgA, "TestOne"}, {"skip", pkgA, "TestOne"}},
		passed(pkgB, "TestTwo"),
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)

	r := analyzeRun(t, modeExplore, 0, s)
	if !r.OK {
		t.Fatalf("explore mode failed on a skip:\n%s", r.text())
	}

	if r.Complete {
		t.Fatal("explore run with a skipped expected test claims to be complete")
	}

	if !strings.Contains(r.text(), "NOT a completed integration verification") {
		t.Fatalf("explore report does not say the run is incomplete:\n%s", r.text())
	}
}

func TestAnalyze_exploreFailureFails(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		[][]string{{"run", pkgB, "TestTwo"}, {"fail", pkgB, "TestTwo"}},
		[][]string{{"pass", pkgA}, {"fail", pkgB}},
	)...)
	requireFailure(t, analyzeRun(t, modeExplore, 1, s), pkgB+".TestTwo")
}

func TestAnalyze_exploreMissingFails(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)
	requireFailure(t, analyzeRun(t, modeExplore, 0, s), pkgB+".TestTwo")
}

func TestAnalyze_orderAcrossPackagesDoesNotMatter(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgB}},
		passed(pkgB, "TestTwo"),
		[][]string{{"start", pkgA}, {"pass", pkgB}},
		passed(pkgA, "TestOne"),
		[][]string{{"pass", pkgA}},
	)...)
	if r := analyzeRun(t, modeRequired, 0, s); !r.OK {
		t.Fatalf("interleaved packages rejected:\n%s", r.text())
	}
}

func TestAnalyze_truncatedUntaggedTestFails(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		passed(pkgB, "TestTwo"),
		[][]string{{"run", pkgB, "TestUnit"}},
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)
	requireFailure(t, analyzeRun(t, modeRequired, 0, s), "truncated: test "+pkgB+".TestUnit")
}

func TestAnalyze_duplicateNameWorstResultCounts(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		[][]string{{"run", pkgA, "TestOne"}, {"skip", pkgA, "TestOne"}},
		passed(pkgA, "TestOne"),
		passed(pkgB, "TestTwo"),
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)

	r := analyze(analysisInput{
		Expected: map[testID]int{{pkgA, "TestOne"}: 2, {pkgB, "TestTwo"}: 1},
		Packages: []string{pkgA, pkgB}, Stream: strings.NewReader(s), Mode: modeRequired,
	})
	requireFailure(t, r, pkgA+".TestOne")
}

func TestAnalyze_duplicateNameMustRunEveryDeclaration(t *testing.T) {
	s := allPass(t)

	r := analyze(analysisInput{
		Expected: map[testID]int{{pkgA, "TestOne"}: 2, {pkgB, "TestTwo"}: 1},
		Packages: []string{pkgA, pkgB}, Stream: strings.NewReader(s), Mode: modeRequired,
	})
	requireFailure(t, r, pkgA+".TestOne")

	both := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"), passed(pkgA, "TestOne"), passed(pkgB, "TestTwo"),
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)

	r = analyze(analysisInput{
		Expected: map[testID]int{{pkgA, "TestOne"}: 2, {pkgB, "TestTwo"}: 1},
		Packages: []string{pkgA, pkgB}, Stream: strings.NewReader(both), Mode: modeRequired,
	})
	if !r.OK {
		t.Fatalf("both declarations passed but the run failed:\n%s", r.text())
	}
}

func TestAnalyze_skippedSubtestFailsRequired(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		[][]string{
			{"run", pkgB, "TestTwo"}, {"run", pkgB, "TestTwo/case"},
			{"skip", pkgB, "TestTwo/case"}, {"pass", pkgB, "TestTwo"},
		},
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)
	requireFailure(t, analyzeRun(t, modeRequired, 0, s), pkgB+".TestTwo/case")

	r := analyzeRun(t, modeExplore, 0, s)
	if !r.OK || r.Complete {
		t.Fatalf("explore: want OK and incomplete, got OK=%v complete=%v", r.OK, r.Complete)
	}
}

func TestAnalyze_showsTheEndOfLongFailureOutput(t *testing.T) {
	events := [][]string{{"start", pkgA}, {"start", pkgB}}
	events = append(events, passed(pkgA, "TestOne")...)
	events = append(events, []string{"run", pkgB, "TestTwo"})

	for i := 0; i < 60; i++ {
		events = append(events, []string{"output", pkgB, "TestTwo", "    container log line\n"})
	}

	events = append(events,
		[]string{"output", pkgB, "TestTwo", "    x_test.go:9: THE REAL FAILURE REASON\n"},
		[]string{"fail", pkgB, "TestTwo"}, []string{"pass", pkgA}, []string{"fail", pkgB})

	r := analyzeRun(t, modeRequired, 1, stream(t, events...))
	requireFailure(t, r, "THE REAL FAILURE REASON")
}

func TestAnalyze_showsCompilerOutput(t *testing.T) {
	const build = `{"ImportPath":"example.com/m/b [example.com/m/b.test]",`
	s := build + `"Action":"build-output","Output":"b/b_test.go:7:2: undefined: thing\n"}` + "\n" +
		build + `"Action":"build-fail"}` + "\n" +
		stream(t, steps(
			[][]string{{"start", pkgA}},
			passed(pkgA, "TestOne"),
			[][]string{{"pass", pkgA}, {"start", pkgB}, {"fail", pkgB}},
		)...)
	requireFailure(t, analyzeRun(t, modeRequired, 1, s), "undefined: thing")
}

func TestAnalyze_namesFailedPackageWithItsOutput(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		passed(pkgB, "TestTwo"),
		[][]string{{"output", pkgB, "", "panic: setup in TestMain\n"}, {"pass", pkgA}, {"fail", pkgB}},
	)...)

	r := analyzeRun(t, modeRequired, 1, s)
	requireFailure(t, r, "package "+pkgB+" failed")
	requireFailure(t, r, "panic: setup in TestMain")
}

func TestAnalyze_testsOutsideTheSelectionFail(t *testing.T) {
	r := analyze(analysisInput{
		Expected: twoExpected,
		Outside:  []string{"example.com/m/c.TestForgotten"},
		Packages: []string{pkgA, pkgB}, Stream: strings.NewReader(allPass(t)), Mode: modeRequired,
	})
	requireFailure(t, r, "example.com/m/c.TestForgotten")
}

func TestWorst_isOrderIndependent(t *testing.T) {
	cases := map[string][]string{
		actionFail: {actionPass, actionFail},
		actionSkip: {actionPass, actionSkip},
		"":         nil,
	}

	for want, results := range cases {
		if got := worst(results); got != want {
			t.Errorf("worst(%q) = %q, want %q", results, got, want)
		}

		reversed := append([]string{}, results...)
		for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
			reversed[i], reversed[j] = reversed[j], reversed[i]
		}

		if got := worst(reversed); got != want {
			t.Errorf("worst(%q) = %q, want %q", reversed, got, want)
		}
	}
}

func TestAnalyze_duplicateNameOneFailureIsNamed(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		[][]string{{"run", pkgA, "TestOne"}, {"fail", pkgA, "TestOne"}},
		passed(pkgB, "TestTwo"),
		[][]string{{"fail", pkgA}, {"pass", pkgB}},
	)...)

	r := analyze(analysisInput{
		Expected: map[testID]int{{pkgA, "TestOne"}: 2, {pkgB, "TestTwo"}: 1},
		Packages: []string{pkgA, pkgB}, Stream: strings.NewReader(s), ExitCode: 1, Mode: modeRequired,
	})

	if len(r.Failed) != 1 || r.Failed[0] != pkgA+".TestOne" {
		t.Fatalf("failed = %q, want [%s.TestOne]\n%s", r.Failed, pkgA, r.text())
	}
}

func TestAnalyze_outsideTestsMakeExploreIncompleteNotFailed(t *testing.T) {
	r := analyze(analysisInput{
		Expected: twoExpected,
		Outside:  []string{"example.com/m/c.TestElsewhere"},
		Packages: []string{pkgA, pkgB}, Stream: strings.NewReader(allPass(t)), Mode: modeExplore,
	})

	if !r.OK || r.Complete {
		t.Fatalf("explore with tests outside the selection: want OK and incomplete, got OK=%v complete=%v\n%s",
			r.OK, r.Complete, r.text())
	}

	if !strings.Contains(r.text(), "example.com/m/c.TestElsewhere") {
		t.Fatalf("report does not list the test outside the selection:\n%s", r.text())
	}
}

func TestAnalyze_countsEveryDeclaration(t *testing.T) {
	both := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"), passed(pkgA, "TestOne"), passed(pkgB, "TestTwo"),
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)

	r := analyze(analysisInput{
		Expected: map[testID]int{{pkgA, "TestOne"}: 2, {pkgB, "TestTwo"}: 1},
		Packages: []string{pkgA, pkgB}, Stream: strings.NewReader(both), Mode: modeRequired,
	})

	if r.Expected != 3 || r.Executed != 3 || r.Passed != 3 {
		t.Fatalf("expected/executed/passed = %d/%d/%d, want 3/3/3", r.Expected, r.Executed, r.Passed)
	}
}

func TestAnalyze_resultWithoutRunIsMalformed(t *testing.T) {
	s := stream(t, steps(
		[][]string{{"start", pkgA}, {"start", pkgB}},
		passed(pkgA, "TestOne"),
		[][]string{{"pass", pkgB, "TestTwo"}},
		[][]string{{"pass", pkgA}, {"pass", pkgB}},
	)...)
	requireFailure(t, analyzeRun(t, modeRequired, 0, s), "without a run event")
}
