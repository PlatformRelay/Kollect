// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/platformrelay/kollect/internal/pipeline"
)

// initIsTerminal is overridden in tests to force the non-TTY path.
var initIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

type initFlags struct {
	kubeconfig string
	context    string
	outputDir  string
}

func newInitCmd() (*cobra.Command, *int) {
	flags := &initFlags{}
	exitCode := new(int)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactively generate KollectProfile + KollectTarget YAML from cluster discovery",
		Long: `Walk through kubecontext confirmation, API discovery, namespace scope,
resource filters, and consented attribute sampling, then write deterministic
KollectProfile and KollectTarget YAML.

Requires an interactive terminal. For non-interactive setups, copy a starter from
config/samples/pipeline/ instead.

Attribute sampling reads one representative object only after you consent and
see its GVK/namespace/name. Secret and other sensitive kinds require a distinct
confirmation; sampled secret values are never printed.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			code, err := runInit(cmd, flags)
			*exitCode = code
			return err
		},
	}

	cmd.Flags().StringVar(&flags.kubeconfig, "kubeconfig", "",
		"path to kubeconfig file (default: $KUBECONFIG, then ~/.kube/config)")
	cmd.Flags().StringVar(&flags.context, "context", "",
		"kubecontext to use (default: current-context)")
	cmd.Flags().StringVar(&flags.outputDir, "output-dir", "./collect-config",
		"directory to write profile.yaml and target.yaml")

	return cmd, exitCode
}

func runInit(cmd *cobra.Command, flags *initFlags) (int, error) {
	if !initIsTerminal() {
		return ExitFatalError, pipeline.ErrNonInteractive
	}

	kubeconfig := effectiveKubeconfigPath(flags.kubeconfig)
	color := os.Getenv("NO_COLOR") == ""

	disc, err := pipeline.NewKubeInitDiscoverer(kubeconfig, flags.context)
	if err != nil {
		return ExitFatalError, err
	}
	sampler, err := pipeline.NewKubeInitSampler(kubeconfig, flags.context)
	if err != nil {
		return ExitFatalError, err
	}

	_, err = pipeline.RunInit(pipeline.InitOptions{
		Kubeconfig: kubeconfig,
		Context:    flags.context,
		OutputDir:  flags.outputDir,
		Prompter:   pipeline.NewSurveyPrompter(color),
		Discoverer: disc,
		Sampler:    sampler,
		Stdout:     cmd.OutOrStdout(),
		Stderr:     cmd.ErrOrStderr(),
		IsTerminal: initIsTerminal,
		Color:      color,
	})
	if err != nil {
		if pipeline.IsInitCanceled(err) {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "canceled:", err)
			return ExitFatalError, err
		}
		return ExitFatalError, err
	}
	return ExitSuccess, nil
}
