// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package samples_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/konih/kollect/internal/pipeline"
	"github.com/konih/kollect/internal/validation"
)

// TestPipelineSampleDirsLoad asserts every use-case directory under
// config/samples/pipeline/ is a self-contained, copy-paste-ready kollect-pipeline
// config directory (ADR-0801 P-007). Each subdirectory must:
//
//   - load cleanly through the real pipeline.LoadConfig (the same loader the CLI uses),
//     with no "unknown kind" warnings (which would mean a typo'd apiVersion/kind);
//   - carry at least one KollectProfile and one KollectTarget, every profileRef resolved;
//   - have every KollectProfile pass validation.ValidateProfile (this validates the
//     attribute path syntax — JSONPath / cel: / helm:release. — against the real extractor);
//   - resolve to exactly one sink via pipeline.ResolveSink, either the synthetic local
//     sink from --output (subdirs with zero sink manifests) or the single loaded
//     KollectSnapshotSink (subdirs that ship one, which must pass ValidateSnapshotSinkSpec).
//
// This makes the samples honest: they are validated against shipped code, not just
// decoded as YAML.
func TestPipelineSampleDirsLoad(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "config", "samples", "pipeline")

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read pipeline samples dir %q: %v", root, err)
	}

	subdirs := make([]string, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			subdirs = append(subdirs, ent.Name())
		}
	}

	const wantMinSubdirs = 4
	if len(subdirs) < wantMinSubdirs {
		t.Fatalf("expected at least %d use-case subdirs under %q, found %d: %v",
			wantMinSubdirs, root, len(subdirs), subdirs)
	}

	for _, name := range subdirs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(root, name)

			loaded, err := pipeline.LoadConfig(dir)
			if err != nil {
				t.Fatalf("LoadConfig(%q): %v", dir, err)
			}

			if len(loaded.Warnings) > 0 {
				t.Fatalf("%s: LoadConfig produced warnings (unknown/typo'd kinds?): %v", dir, loaded.Warnings)
			}

			if len(loaded.Profiles) == 0 {
				t.Fatalf("%s: no KollectProfile found", dir)
			}

			if len(loaded.Targets) == 0 {
				t.Fatalf("%s: no KollectTarget found", dir)
			}

			for i := range loaded.Profiles {
				p := loaded.Profiles[i]
				if errs := validation.ValidateProfile(&p); len(errs) > 0 {
					t.Fatalf("%s: profile %q failed validation: %v", dir, p.Name, errs)
				}
			}

			// Subdirs that ship a sink manifest resolve that sink directly; subdirs
			// without one are the --output shorthand pattern (synthetic local sink).
			switch len(loaded.Sinks) {
			case 0:
				sinkSpec, err := pipeline.ResolveSink(loaded, "/tmp/inventory")
				if err != nil {
					t.Fatalf("%s: ResolveSink with --output: %v", dir, err)
				}
				if sinkSpec.Type != pipeline.LocalSinkType {
					t.Fatalf("%s: expected synthetic %q sink from --output, got %q", dir, pipeline.LocalSinkType, sinkSpec.Type)
				}
			case 1:
				spec := loaded.Sinks[0].Spec
				if errs := validation.ValidateSnapshotSinkSpec(&spec); len(errs) > 0 {
					t.Fatalf("%s: sink %q failed validation: %v", dir, loaded.Sinks[0].Name, errs)
				}
				sinkSpec, err := pipeline.ResolveSink(loaded, "")
				if err != nil {
					t.Fatalf("%s: ResolveSink with in-dir sink: %v", dir, err)
				}
				if sinkSpec.Type != spec.Type {
					t.Fatalf("%s: ResolveSink type %q != sink spec type %q", dir, sinkSpec.Type, spec.Type)
				}
			default:
				t.Fatalf("%s: %d sink manifests found; a pipeline config dir supports at most one", dir, len(loaded.Sinks))
			}
		})
	}
}

// TestPipelineGitSinkSampleSecretResolvesFromEnv locks the shipped git-sink sample's
// CI-credential flow end to end: its committed Secret manifest carries only a
// ${env:KOLLECT_GIT_TOKEN} placeholder, and the real resolver substitutes the value
// from the environment (the pipeline "secretRef.env" binding — GitLab masked
// variables and friends). Not parallel: t.Setenv forbids it.
func TestPipelineGitSinkSampleSecretResolvesFromEnv(t *testing.T) {
	t.Setenv("KOLLECT_GIT_TOKEN", "masked-ci-token")

	dir := filepath.Join("..", "..", "config", "samples", "pipeline", "git-sink")

	loaded, err := pipeline.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig(%q): %v", dir, err)
	}

	sinkSpec, err := pipeline.ResolveSink(loaded, "")
	if err != nil {
		t.Fatalf("ResolveSink: %v", err)
	}
	if sinkSpec.SecretRef == nil {
		t.Fatalf("%s: expected the git-sink sample to reference its committed Secret manifest", dir)
	}

	data, err := pipeline.ResolveSinkSecretData(sinkSpec, loaded.Secrets)
	if err != nil {
		t.Fatalf("ResolveSinkSecretData: %v", err)
	}
	if got := string(data["token"]); got != "masked-ci-token" {
		t.Errorf("data[token] = %q, want the env-substituted masked-ci-token", got)
	}
}
