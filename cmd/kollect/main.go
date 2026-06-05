// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "create-remote-secret":
		if err := runCreateRemoteSecret(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "create-remote-secret: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func printUsage(w *os.File) {
	_, _ = fmt.Fprintf(w, `kollect — developer utilities for multi-cluster inventory

Usage:
  kollect create-remote-secret [flags]

Flags for create-remote-secret:
  --cluster       Spoke cluster name (required)
  --namespace     Hub namespace for the Secret (default: platform)
  --api-server    Spoke Kubernetes API URL (default: placeholder)
  --token         Spoke SA token (default: placeholder)
  --ca-file       PEM CA file; base64-encoded into kubeconfig fragment

Example:
  go run ./cmd/kollect create-remote-secret --cluster spoke-a | kubectl apply -f -

See docs/adr/0503-hub-cluster-auth-istio-pattern.md and docs/DEVELOPMENT.md.
`)
}
