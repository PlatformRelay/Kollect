// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// perf-report aggregates benchmark artifacts and the metrics catalog for coordinator agents.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/konih/kollect/internal/metrics"
)

type report struct {
	GeneratedAt   string                 `json:"generatedAt"`
	ScaleTier     string                 `json:"scaleTier"`
	RepoRoot      string                 `json:"repoRoot"`
	Benchmarks    []benchResult          `json:"benchmarks,omitempty"`
	BenchArtifact string                 `json:"benchArtifact,omitempty"`
	Metrics       []metrics.CatalogEntry `json:"metrics"`
	ScaleTargets  scaleTargets           `json:"scaleTargets"`
	Notes         []string               `json:"notes,omitempty"`
}

type benchResult struct {
	Name        string  `json:"name"`
	NsPerOp     float64 `json:"nsPerOp,omitempty"`
	BytesPerOp  int64   `json:"bytesPerOp,omitempty"`
	AllocsPerOp int64   `json:"allocsPerOp,omitempty"`
}

type scaleTargets struct {
	SpokeObjects int    `json:"spokeObjectsBaseline"`
	HubClusters  int    `json:"hubClustersBaseline"`
	Reference    string `json:"reference"`
}

var benchLine = regexp.MustCompile(`^(Benchmark\w+)(-\d+)?\s+\d+\s+(\d+(?:\.\d+)?)\s+ns/op(?:\s+(\d+)\s+B/op)?(?:\s+(\d+)\s+allocs/op)?`)

func main() {
	format := flag.String("format", "json", "output format: json or markdown")
	output := flag.String("output", "", "write to file instead of stdout")
	repoRoot := flag.String("root", "", "repository root (default: auto-detect)")
	flag.Parse()

	root := *repoRoot
	if root == "" {
		root = detectRepoRoot()
	}

	rep := buildReport(root)

	switch strings.ToLower(*format) {
	case "markdown", "md":
		data := renderMarkdown(rep)
		writeOutput(*output, data)
	case "json":
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
			os.Exit(1)
		}
		writeOutput(*output, data)
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q\n", *format)
		os.Exit(2)
	}
}

func detectRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}

		dir = parent
	}
}

func buildReport(root string) report {
	artifact, benches := parseLatestBench(filepath.Join(root, "artifacts", "bench"))

	notes := []string{
		"Regenerate after benchmark or load-test changes: task perf-report -- --format=markdown",
		"Never commit agent-context/PERF-SNAPSHOT.md or artifacts/bench/",
	}

	if artifact == "" {
		notes = append(notes, "No bench artifacts yet — run: task bench")
	}

	return report{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		ScaleTier:     detectScaleTier(),
		RepoRoot:      root,
		Benchmarks:    benches,
		BenchArtifact: artifact,
		Metrics:       metrics.Catalog,
		ScaleTargets: scaleTargets{
			SpokeObjects: 10_000,
			HubClusters:  100,
			Reference:    "docs/adr/0026-performance-scalability.md",
		},
		Notes: notes,
	}
}

func detectScaleTier() string {
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		return "ci"
	}

	if os.Getenv("KOLECT_LOAD_TEST") == "1" {
		return "load"
	}

	return "dev"
}

func parseLatestBench(dir string) (string, []benchResult) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}

		files = append(files, filepath.Join(dir, e.Name()))
	}

	if len(files) == 0 {
		return "", nil
	}

	sort.Strings(files)
	latest := files[len(files)-1]

	data, err := os.ReadFile(latest)
	if err != nil {
		return latest, nil
	}

	seen := make(map[string]benchResult)
	for _, line := range strings.Split(string(data), "\n") {
		m := benchLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}

		name := m[1]
		var ns float64
		fmt.Sscanf(m[3], "%f", &ns)

		br := benchResult{Name: name, NsPerOp: ns}
		if m[4] != "" {
			fmt.Sscanf(m[4], "%d", &br.BytesPerOp)
		}

		if m[5] != "" {
			fmt.Sscanf(m[5], "%d", &br.AllocsPerOp)
		}

		seen[name] = br
	}

	out := make([]benchResult, 0, len(seen))
	for _, br := range seen {
		out = append(out, br)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return latest, out
}

func renderMarkdown(rep report) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "# PERF-SNAPSHOT (local only — do not commit)\n\n")
	fmt.Fprintf(&b, "Generated: %s  \n", rep.GeneratedAt)
	fmt.Fprintf(&b, "Scale tier: `%s`  \n", rep.ScaleTier)
	fmt.Fprintf(&b, "Repo: `%s`\n\n", rep.RepoRoot)

	fmt.Fprintf(&b, "## Scale targets\n\n")
	fmt.Fprintf(&b, "| Target | Value | Reference |\n")
	fmt.Fprintf(&b, "| --- | --- | --- |\n")
	fmt.Fprintf(&b, "| Spoke watched objects (baseline) | %d+ | %s |\n", rep.ScaleTargets.SpokeObjects, rep.ScaleTargets.Reference)
	fmt.Fprintf(&b, "| Hub clusters (baseline) | %d+ | ADR-0022 |\n\n", rep.ScaleTargets.HubClusters)

	fmt.Fprintf(&b, "## Benchmarks\n\n")
	if rep.BenchArtifact == "" {
		fmt.Fprintf(&b, "_No artifacts yet — run `task bench`._\n\n")
	} else {
		fmt.Fprintf(&b, "Source: `%s`\n\n", rep.BenchArtifact)
		fmt.Fprintf(&b, "| Benchmark | ns/op | B/op | allocs/op |\n")
		fmt.Fprintf(&b, "| --- | --- | --- | --- |\n")
		for _, br := range rep.Benchmarks {
			fmt.Fprintf(&b, "| %s | %.0f | %d | %d |\n", br.Name, br.NsPerOp, br.BytesPerOp, br.AllocsPerOp)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "## Metrics catalog\n\n")
	fmt.Fprintf(&b, "| Metric | Type | PromQL hint | Agent interpretation |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- |\n")
	for _, m := range rep.Metrics {
		labels := strings.Join(m.Labels, ", ")
		if labels != "" {
			labels = " (" + labels + ")"
		}

		fmt.Fprintf(&b, "| `%s`%s | %s | `%s` | %s |\n",
			m.Name, labels, m.Type, m.PromQLHint, m.AgentHint)
	}

	fmt.Fprintf(&b, "\n## Notes\n\n")
	for _, note := range rep.Notes {
		fmt.Fprintf(&b, "- %s\n", note)
	}

	return []byte(b.String())
}

func writeOutput(path string, data []byte) {
	if path == "" {
		os.Stdout.Write(data)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			fmt.Println()
		}

		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir output dir: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write output: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
}
