// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestTestCommand_requiredModeDemandsDocker(t *testing.T) {
	cmd := testCommand(modeRequired, []string{"./a/...", "./b"}, []string{"PATH=/bin", requireDockerEnv + "=false"})

	want := []string{"go", "test", "-json", "-count=1", "-tags=" + integrationTag, "./a/...", "./b"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args:\n got %q\nwant %q", cmd.Args, want)
	}

	// The last assignment wins in exec, so a caller's false cannot weaken required mode.
	if last := cmd.Env[len(cmd.Env)-1]; last != requireDockerEnv+"=true" {
		t.Fatalf("required mode env ends with %q, want %s=true", last, requireDockerEnv)
	}
}

func TestTestCommand_exploreModeLeavesEnvironment(t *testing.T) {
	base := []string{"PATH=/bin"}

	cmd := testCommand(modeExplore, []string{"./a"}, base)
	if slices.ContainsFunc(cmd.Env, func(kv string) bool { return kv == requireDockerEnv+"=true" }) {
		t.Fatalf("explore mode set %s: %q", requireDockerEnv, cmd.Env)
	}
}

func TestExitCode_keepsTheProcessStatus(t *testing.T) {
	code, err := exitCode(exec.Command("sh", "-c", "exit 3").Run())
	if err != nil || code != 3 {
		t.Fatalf("exitCode = %d, %v; want 3, nil", code, err)
	}

	code, err = exitCode(nil)
	if err != nil || code != 0 {
		t.Fatalf("exitCode(nil) = %d, %v; want 0, nil", code, err)
	}

	if _, err := exitCode(exec.Command("/nonexistent-binary").Run()); err == nil {
		t.Fatal("a process that never started was treated as an exit status")
	}
}

func TestOutputEcho_writesTestOutputAsItArrives(t *testing.T) {
	var out strings.Builder

	echo := &outputEcho{w: &out}

	lines := `{"Action":"run","Package":"p","Test":"TestA"}` + "\n" +
		`{"Action":"output","Package":"p","Test":"TestA","Output":"=== RUN   TestA\n"}` + "\n" +
		`{"Action":"output","Package":"p","Output":"ok  \tp\t0.1s\n"}` + "\n" +
		"# raw line that is not json\n" +
		`{"Action":"build-output","ImportPath":"p","Output":"p.go:1: undefined: x\n"}` + "\n"

	// Split the stream mid-line to prove partial writes are buffered, not dropped.
	if _, err := echo.Write([]byte(lines[:30])); err != nil {
		t.Fatal(err)
	}

	if _, err := echo.Write([]byte(lines[30:])); err != nil {
		t.Fatal(err)
	}

	// Per-test output stays out of the live log (the report shows it for failures); package
	// summaries, build output and anything that is not an event go through, like plain go test.
	want := "ok  \tp\t0.1s\n# raw line that is not json\np.go:1: undefined: x\n"
	if out.String() != want {
		t.Fatalf("echoed:\n%q\nwant:\n%q", out.String(), want)
	}
}
