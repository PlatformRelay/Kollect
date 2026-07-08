// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
)

// collectFlags holds the parsed flags for the `collect` subcommand (ADR-0801).
type collectFlags struct {
	kubeconfig string
	config     string
	output     string
	dryRun     bool
	logLevel   string
	context    []string
	namespace  string
}

// logLevels maps the --log-level flag value to a zap level; also used to validate the flag.
var logLevels = map[string]zapcore.Level{
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
}

func bindCollectFlags(cmd *cobra.Command, f *collectFlags) {
	cmd.Flags().StringVar(&f.kubeconfig, "kubeconfig", "",
		"path to kubeconfig file (default: $KUBECONFIG, then ~/.kube/config)")
	cmd.Flags().StringVar(&f.config, "config", "",
		"directory of KollectProfile + KollectTarget + Sink YAML files (required)")
	cmd.Flags().StringVar(&f.output, "output", "",
		"local filesystem output directory (implies type:local sink when no Sink YAML is found)")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false,
		"collect and print what would be written; do not write files or push git")
	cmd.Flags().StringVar(&f.logLevel, "log-level", "info", "debug|info|warn|error")
	cmd.Flags().StringSliceVar(&f.context, "context", nil,
		"kubecontext name(s) and/or glob pattern(s) to collect from; repeatable and comma-separated")
	cmd.Flags().StringVar(&f.namespace, "namespace", "",
		"restrict collection to a single namespace (overrides target selectors)")
}
