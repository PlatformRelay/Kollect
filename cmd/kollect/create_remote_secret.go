// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"github.com/konih/kollect/internal/remotesecret"
)

func runCreateRemoteSecret(args []string) error {
	fs := flag.NewFlagSet("create-remote-secret", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		cluster   string
		namespace string
		apiServer string
		token     string
		caFile    string
	)

	fs.StringVar(&cluster, "cluster", "", "spoke cluster name (required)")
	fs.StringVar(&namespace, "namespace", "platform", "hub namespace for the Secret")
	fs.StringVar(&apiServer, "api-server", "", "spoke Kubernetes API server URL")
	fs.StringVar(&token, "token", "", "spoke service account token")
	fs.StringVar(&caFile, "ca-file", "", "PEM-encoded CA for spoke API TLS")

	if err := fs.Parse(args); err != nil {
		return err
	}

	caData := ""
	if caFile != "" {
		pem, err := os.ReadFile(caFile) //nolint:gosec // G304: explicit --ca-file from operator CLI
		if err != nil {
			return fmt.Errorf("read ca file: %w", err)
		}

		caData = base64.StdEncoding.EncodeToString(pem)
	}

	out, err := remotesecret.GenerateYAML(remotesecret.Options{
		ClusterName: cluster,
		Namespace:   namespace,
		APIServer:   apiServer,
		Token:       token,
		CAData:      caData,
	})
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(os.Stdout, out)

	return err
}
