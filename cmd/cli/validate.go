// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/platformrelay/kollect/internal/pipeline"
	"github.com/platformrelay/kollect/internal/validation"
)

type validateFlags struct {
	config string
}

// newValidateCmd builds the `validate` subcommand (PIPE-VALIDATE-01). pipeline.LoadConfig
// only does structural YAML decoding (cmd/cli/collect.go's runCollectPipeline calls nothing
// beyond it either), so a config directory that the operator's admission webhook would reject
// was previously accepted silently by both "collect" and "init" and could only be caught by a
// real cluster run. "validate" runs the same internal/validation checks the webhook applies,
// so a config directory can be linted in CI without a cluster.
func newValidateCmd() (*cobra.Command, *int) {
	flags := &validateFlags{}
	exitCode := new(int)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Check a config directory against the operator's admission rules, without a cluster",
		Long: `Loads every KollectProfile, KollectTarget, and KollectSnapshotSink in --config and
runs the same internal/validation checks the operator's admission webhook applies on
create/update.

Checks that need live cluster state are not evaluated here and still require a real run:
KollectScope-bound namespace/GVK enforcement, and any cross-object existence check the
webhook performs against the API server (e.g. a Target's profileRef resolving to a Profile
that exists in-cluster — --config directory profileRefs are already checked structurally by
"collect" and "init" via LoadConfig).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			code, err := runValidate(cmd, flags)
			*exitCode = code

			return err
		},
	}

	cmd.Flags().StringVar(&flags.config, "config", "",
		"directory of KollectProfile + KollectTarget + Sink YAML files (required)")

	return cmd, exitCode
}

func runValidate(cmd *cobra.Command, flags *validateFlags) (int, error) {
	if flags.config == "" {
		return ExitFatalError, fmt.Errorf("--config is required")
	}

	loaded, err := pipeline.LoadConfig(flags.config)
	if err != nil {
		return ExitFatalError, err
	}

	for _, w := range loaded.Warnings {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
	}

	invalid, warnings := validateLoaded(loaded)

	for _, w := range warnings {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
	}

	if len(invalid) > 0 {
		for _, e := range invalid {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "invalid:", e)
		}

		return ExitFatalError, fmt.Errorf("%d config object(s) failed validation", len(invalid))
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: valid (%d profile(s), %d target(s), %d sink(s))\n",
		flags.config, len(loaded.Profiles), len(loaded.Targets), len(loaded.Sinks))

	return ExitSuccess, nil
}

// validateLoaded mirrors internal/webhook/v1alpha1's per-kind validators (kollectprofile_webhook.go,
// kollecttarget_webhook.go, family_sink_webhook.go) minus their cluster-dependent KollectScope
// checks, which need a live client and cannot run offline.
func validateLoaded(loaded pipeline.LoadResult) (invalid []error, warnings []string) {
	for i := range loaded.Profiles {
		profile := &loaded.Profiles[i]

		if errs := validation.ValidateProfile(profile); len(errs) > 0 {
			invalid = append(invalid, validation.ProfileInvalid(profile.Name, errs))
			continue
		}

		warnings = append(warnings, validation.ProfileWarnings(profile)...)
	}

	for i := range loaded.Targets {
		target := &loaded.Targets[i]

		if errs := validation.ValidateTargetSpec(&target.Spec); len(errs) > 0 {
			invalid = append(invalid, validation.TargetInvalid(target.Name, errs))
		}
	}

	for i := range loaded.Sinks {
		snk := &loaded.Sinks[i]

		if errs := validation.ValidateSnapshotSinkSpec(&snk.Spec); len(errs) > 0 {
			invalid = append(invalid, validation.SnapshotSinkInvalid(snk.Name, errs))
			continue
		}

		normalized := snk.Spec.ToKollectSinkSpec()
		warnings = append(warnings, validation.ValidateGitSinkWarnings(&normalized)...)
		warnings = append(warnings, validation.ValidateSinkConfigWarnings(&normalized)...)
	}

	return invalid, warnings
}
