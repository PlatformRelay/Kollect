// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/platformrelay/kollect/internal/pipeline"
	"github.com/platformrelay/kollect/internal/sink"
)

// setLoggerOnce guards ctrl.SetLogger: it mutates shared global state and is not safe to
// call from multiple goroutines. A real CLI invocation only ever calls it once anyway (one
// process, one collect command); Once makes that explicit and race-free under `go test -race`
// with parallel subtests that each drive the full RunE path.
var setLoggerOnce sync.Once

// newCollectCmd builds the `collect` subcommand. The returned *int is written once RunE
// finishes a full (non-flag-validation) run; main reads it after cmd.Execute() returns to
// decide the process exit code (ExitSuccess/ExitPartialFailure/ExitFatalError) without
// calling os.Exit from inside testable code.
func newCollectCmd() (*cobra.Command, *int) {
	flags := &collectFlags{}
	exitCode := new(int)

	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect Kubernetes inventory from a kubeconfig without installing the operator",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code, err := runCollect(cmd, flags)
			*exitCode = code

			return err
		},
	}

	bindCollectFlags(cmd, flags)

	return cmd, exitCode
}

// runCollect validates flags, then delegates to runCollectPipeline. Flag/config validation
// failures are returned as errors (cobra prints them and main maps them to ExitFatalError);
// once a run actually starts, its outcome is reported via the returned exit code instead,
// since a partial multi-context failure isn't a Go-level error.
func runCollect(cmd *cobra.Command, flags *collectFlags) (int, error) {
	if flags.config == "" {
		return ExitFatalError, fmt.Errorf("--config is required")
	}

	level, ok := logLevels[flags.logLevel]
	if !ok {
		return ExitFatalError, fmt.Errorf("invalid --log-level %q: must be one of debug|info|warn|error", flags.logLevel)
	}

	format, err := pipeline.ParseStdoutFormat(flags.format)
	if err != nil {
		return ExitFatalError, err
	}

	stdoutMode := flags.output == pipeline.StdoutSentinel
	if stdoutMode && flags.dryRun {
		return ExitFatalError, fmt.Errorf("--output - (stdout) and --dry-run are mutually exclusive")
	}

	if cmd.Flags().Changed("format") && !stdoutMode {
		return ExitFatalError, fmt.Errorf("--format applies only with --output - (stdout export)")
	}

	setLoggerOnce.Do(func() {
		ctrl.SetLogger(zap.New(zap.Level(level)))
	})

	return runCollectPipeline(cmd, flags, format)
}

func runCollectPipeline(cmd *cobra.Command, flags *collectFlags, format pipeline.StdoutFormat) (int, error) {
	loaded, err := pipeline.LoadConfig(flags.config)
	if err != nil {
		return ExitFatalError, err
	}

	kubeconfigPath := effectiveKubeconfigPath(flags.kubeconfig)

	contexts, warnings, err := resolveContexts(kubeconfigPath, flags.context)
	if err != nil {
		return ExitFatalError, err
	}

	for _, w := range warnings {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
	}

	sinkSpec, err := pipeline.ResolveSink(loaded, flags.output)
	if err != nil {
		return ExitFatalError, err
	}

	if len(contexts) > 1 && sinkSpec.Cluster != "" {
		return ExitFatalError, fmt.Errorf(
			"spec.cluster (%q) conflicts with multiple --context values; leave spec.cluster unset for multi-context runs",
			sinkSpec.Cluster)
	}

	secretData, err := pipeline.ResolveSinkSecretData(sinkSpec, loaded.Secrets)
	if err != nil {
		return ExitFatalError, err
	}

	loaded.Targets = pipeline.ApplyNamespaceOverride(loaded.Targets, flags.namespace)

	results := pipeline.RunAllContexts(cmd.Context(), contexts, kubeconfigPath, loaded,
		sinkSpec, secretData, sink.NewRegistry(), nil, flags.dryRun)

	// Stdout export: data-only records go to stdout in context+target order; logs, warnings,
	// and the per-context errors below all go to stderr, so a consumer can pipe stdout to a
	// parser untouched. A write/marshal failure here is a fatal output error (exit 2).
	if pipeline.IsStdoutSink(sinkSpec) {
		var records []pipeline.StdoutRecord
		for _, r := range results {
			records = append(records, r.Records...)
		}

		if err := pipeline.WriteStdoutRecords(cmd.OutOrStdout(), format, records); err != nil {
			return ExitFatalError, err
		}
	}

	for _, r := range results {
		if r.Fatal != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "context %s: %v\n", r.Context, r.Fatal)
		}

		for _, e := range r.Errs {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "context %s: %v\n", r.Context, e)
		}
	}

	return mapContextResultsToExit(results), nil
}

func effectiveKubeconfigPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	if env := os.Getenv("KUBECONFIG"); env != "" {
		return env
	}

	return clientcmd.RecommendedHomeFile
}
