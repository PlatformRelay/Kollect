// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package envtestassets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/platformrelay/kollect/internal/envtestassets"
)

// realWorldListing is what bin/k8s actually holds on an Apple-silicon workstation whose checkout
// has downloaded assets for more than one platform: two linux-amd64 versions sorting ahead of the
// darwin-arm64 one, plus a stray non-asset directory left behind by a `--bin-dir bin/k8s`
// invocation. Returning the first entry here is what produced
// "fork/exec bin/k8s/1.35.0-linux-amd64/etcd: exec format error".
var realWorldListing = []string{
	"1.35.0-linux-amd64",
	"1.36.0-linux-amd64",
	"1.36.2-darwin-arm64",
	"k8s",
}

// makeBase builds a throwaway bin/k8s-shaped directory containing dirs and files.
func makeBase(t *testing.T, dirs, files []string) string {
	t.Helper()

	base := t.TempDir()

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(base, dir), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	for _, name := range files {
		if err := os.WriteFile(filepath.Join(base, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	return base
}

func TestDir(t *testing.T) {
	tests := []struct {
		name   string
		dirs   []string
		files  []string
		goos   string
		goarch string
		want   string
	}{
		{
			name:   "apple silicon skips the linux assets that sort first",
			dirs:   realWorldListing,
			goos:   "darwin",
			goarch: "arm64",
			want:   "1.36.2-darwin-arm64",
		},
		{
			name:   "linux amd64 still resolves the linux assets CI depends on",
			dirs:   realWorldListing,
			goos:   "linux",
			goarch: "amd64",
			want:   "1.36.0-linux-amd64",
		},
		{
			name:   "the highest matching version wins, compared numerically not lexically",
			dirs:   []string{"1.36.2-linux-arm64", "1.34.0-linux-arm64", "1.36.10-linux-arm64"},
			goos:   "linux",
			goarch: "arm64",
			want:   "1.36.10-linux-arm64",
		},
		{
			name:   "a longer version outranks the prefix it extends",
			dirs:   []string{"1.36-linux-arm64", "1.36.1-linux-arm64"},
			goos:   "linux",
			goarch: "arm64",
			want:   "1.36.1-linux-arm64",
		},
		{
			name:   "unparseable version fields fall back to a deterministic lexical order",
			dirs:   []string{"1.36.2-linux-arm64", "1.36.beta-linux-arm64"},
			goos:   "linux",
			goarch: "arm64",
			want:   "1.36.beta-linux-arm64",
		},
		{
			name:   "a matching name that is a file is not an asset directory",
			dirs:   []string{"k8s"},
			files:  []string{"1.36.2-darwin-arm64"},
			goos:   "darwin",
			goarch: "arm64",
			want:   "",
		},
		{
			name:   "no host match leaves the choice to controller-runtime",
			dirs:   realWorldListing,
			goos:   "windows",
			goarch: "amd64",
			want:   "",
		},
		{
			name:   "an empty base directory matches nothing",
			goos:   "darwin",
			goarch: "arm64",
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := makeBase(t, tc.dirs, tc.files)

			want := ""
			if tc.want != "" {
				want = filepath.Join(base, tc.want)
			}

			if got := envtestassets.Dir([]string{base}, tc.goos, tc.goarch); got != want {
				t.Fatalf("Dir = %q, want %q", got, want)
			}
		})
	}
}

func TestDirTriesEveryBasePathInOrder(t *testing.T) {
	base := makeBase(t, realWorldListing, nil)
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	got := envtestassets.Dir([]string{missing, base}, "darwin", "arm64")
	if want := filepath.Join(base, "1.36.2-darwin-arm64"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}

	if got := envtestassets.Dir(nil, "darwin", "arm64"); got != "" {
		t.Fatalf("Dir with no base paths = %q, want empty", got)
	}
}

func TestDirReturnsAnAbsolutePath(t *testing.T) {
	base := makeBase(t, realWorldListing, nil)

	rel, err := filepath.Rel(mustGetwd(t), base)
	if err != nil {
		t.Skipf("no relative path from cwd to %s: %v", base, err)
	}

	got := envtestassets.Dir([]string{rel}, "darwin", "arm64")
	if !filepath.IsAbs(got) {
		t.Fatalf("Dir = %q, want an absolute path", got)
	}

	if want := filepath.Join(base, "1.36.2-darwin-arm64"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
}

func TestResolvePrefersKubebuilderAssets(t *testing.T) {
	base := makeBase(t, realWorldListing, nil)
	assets := filepath.Join(base, "1.35.0-linux-amd64")

	t.Setenv(envtestassets.EnvVar, assets)

	if got := envtestassets.Resolve([]string{base}, "darwin", "arm64"); got != assets {
		t.Fatalf("Resolve = %q, want %q", got, assets)
	}
}

func TestResolveFallsBackToTheHostMatchingDir(t *testing.T) {
	base := makeBase(t, realWorldListing, nil)

	t.Setenv(envtestassets.EnvVar, "")

	got := envtestassets.Resolve([]string{base}, "darwin", "arm64")
	if want := filepath.Join(base, "1.36.2-darwin-arm64"); got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	return wd
}
