// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Command kollect-pipeline collects Kubernetes inventory from a kubeconfig without
// installing the kollect operator (ADR-0801): a short-lived CLI for CI/CD pipelines,
// as opposed to the long-running in-cluster operator built by cmd/main.go.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/platformrelay/kollect/internal/version"
)

func newRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "kollect-pipeline",
		Short:   "Collect Kubernetes inventory from CI/CD pipelines without installing the operator",
		Version: version.String(),
	}
}

// buildRoot wires every subcommand onto a fresh root. The returned *int is the
// exit code written by the subcommand that actually ran (collect, init, or validate).
func buildRoot() (*cobra.Command, *int) {
	root := newRootCmd()
	exitCode := new(int)

	collectCmd, collectExit := newCollectCmd()
	root.AddCommand(collectCmd)

	initCmd, initExit := newInitCmd()
	root.AddCommand(initCmd)

	validateCmd, validateExit := newValidateCmd()
	root.AddCommand(validateCmd)

	// After Execute, prefer the exit code from whichever command ran. Every
	// subcommand writes its own *int; we expose a shared pointer that main
	// reads, updated via PersistentPostRun on each leaf.
	root.PersistentPostRun = func(cmd *cobra.Command, _ []string) {
		switch cmd.Name() {
		case "collect":
			*exitCode = *collectExit
		case "init":
			*exitCode = *initExit
		case "validate":
			*exitCode = *validateExit
		}
	}

	return root, exitCode
}

func main() {
	root, exitCode := buildRoot()

	if err := root.Execute(); err != nil {
		os.Exit(ExitFatalError)
	}

	os.Exit(*exitCode)
}
