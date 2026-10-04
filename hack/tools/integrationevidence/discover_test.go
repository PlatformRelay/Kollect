// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// discoverFixture lists and discovers the testdata module the way the tool does for the repo.
func discoverFixture(t *testing.T, patterns ...string) []string {
	t.Helper()

	dir, err := filepath.Abs(filepath.Join("testdata", "discover"))
	if err != nil {
		t.Fatal(err)
	}

	pkgs, err := listPackages(dir, patterns)
	if err != nil {
		t.Fatalf("listPackages: %v", err)
	}

	expected, err := discoverExpected(pkgs)
	if err != nil {
		t.Fatalf("discoverExpected: %v", err)
	}

	names := make([]string, 0, len(expected))
	for id := range expected {
		names = append(names, id.String())
	}

	sort.Strings(names)

	return names
}

func TestDiscoverExpected_tagFileTopLevelTestsOnly(t *testing.T) {
	got := discoverFixture(t, "./...")

	// a: TestExport, TestPrune and the external-package TestExternal and bare Test are integration
	// tests. TestMain, Testable, the benchmark, the helper, the method and the untagged TestUnit
	// are not. c builds only without the tag; d builds with or without it.
	want := []string{
		"example.com/discover/a.Test",
		"example.com/discover/a.TestExport",
		"example.com/discover/a.TestExternal",
		"example.com/discover/a.TestPrune",
		"example.com/discover/e.TestDup",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected tests:\n got %q\nwant %q", got, want)
	}
}

func TestDiscoverExpected_packageWithoutTaggedTestsIsEmpty(t *testing.T) {
	if got := discoverFixture(t, "./b", "./c"); len(got) != 0 {
		t.Fatalf("expected no integration tests in b and c, got %q", got)
	}
}

func TestListPackages_reportsEverySelectedPackage(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "discover"))
	if err != nil {
		t.Fatal(err)
	}

	pkgs, err := listPackages(dir, []string{"./..."})
	if err != nil {
		t.Fatalf("listPackages: %v", err)
	}

	paths := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		paths = append(paths, p.ImportPath)
	}

	want := []string{
		"example.com/discover/a", "example.com/discover/b",
		"example.com/discover/c", "example.com/discover/d", "example.com/discover/e",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("packages:\n got %q\nwant %q", paths, want)
	}
}

func TestListPackages_badPatternIsAnError(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "discover"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := listPackages(dir, []string{"./doesnotexist/..."}); err == nil {
		t.Fatal("listPackages accepted a pattern that matches no package")
	}
}

func TestListPackages_patternMatchingNothingIsAnError(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "discover"))
	if err != nil {
		t.Fatal(err)
	}

	// go list only warns, and exits zero, for a pattern that matches no package. Next to a
	// pattern that does match, nothing else would notice that a selected directory is empty.
	if _, err := listPackages(dir, []string{"./a", "./empty/..."}); err == nil {
		t.Fatal("listPackages accepted a pattern that matched no packages")
	}
}

func TestDiscoverExpected_countsEveryDeclaration(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "discover"))
	if err != nil {
		t.Fatal(err)
	}

	pkgs, err := listPackages(dir, []string{"./e", "./a"})
	if err != nil {
		t.Fatal(err)
	}

	expected, err := discoverExpected(pkgs)
	if err != nil {
		t.Fatal(err)
	}

	if got := expected[testID{"example.com/discover/e", "TestDup"}]; got != 2 {
		t.Fatalf("TestDup declarations = %d, want 2 (internal and external test package)", got)
	}

	if got := expected[testID{"example.com/discover/a", "TestExport"}]; got != 1 {
		t.Fatalf("TestExport declarations = %d, want 1", got)
	}
}

func TestOutsideSelection(t *testing.T) {
	all := map[testID]int{
		{"m/a", "TestIn"}: 1, {"m/c", "TestOut"}: 1, {"m/d", "TestAlsoOut"}: 2,
	}

	got := outsideSelection(all, []string{"m/a", "m/b"})
	want := []string{"m/c.TestOut", "m/d.TestAlsoOut"}
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("outsideSelection = %q, want %q", got, want)
	}
}
