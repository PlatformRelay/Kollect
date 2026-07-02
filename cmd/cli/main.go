// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Command kollect-pipeline collects Kubernetes inventory from a kubeconfig without
// installing the kollect operator (ADR-0801): a short-lived CLI for CI/CD pipelines,
// as opposed to the long-running in-cluster operator built by cmd/main.go.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kollect-pipeline",
		Short: "Collect Kubernetes inventory from CI/CD pipelines without installing the operator",
	}
}

func main() {
	root := newRootCmd()

	collectCmd, exitCode := newCollectCmd()
	root.AddCommand(collectCmd)

	if err := root.Execute(); err != nil {
		os.Exit(ExitFatalError)
	}

	os.Exit(*exitCode)
}
