// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/git"
	kafkasink "github.com/konih/kollect/internal/sink/kafka"
	"github.com/konih/kollect/internal/sink/postgres"
	s3sink "github.com/konih/kollect/internal/sink/s3"
)

// RunConnectionTest probes sink connectivity using the same backends as export.
func RunConnectionTest(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	switch spec.Type {
	case "git":
		cfg, err := git.ConfigFromSpec(spec, buildCtx.CAPEM)
		if err != nil {
			return "", err
		}

		if err := git.TestConnection(ctx, cfg); err != nil {
			return "", err
		}

		return "TLS and git remote reachability verified", nil
	case "postgres":
		if err := postgres.TestConnection(ctx, spec, buildCtx.DatabaseSecretData); err != nil {
			return "", err
		}

		return "PostgreSQL ping succeeded", nil
	case "kafka":
		if err := kafkasink.TestConnection(ctx, spec, buildCtx.SecretData); err != nil {
			return "", err
		}

		return "Kafka broker metadata request succeeded", nil
	case "s3":
		if err := s3sink.TestConnection(ctx, spec, buildCtx.SecretData); err != nil {
			return "", err
		}

		return "S3 bucket HeadBucket succeeded", nil
	default:
		return "", fmt.Errorf("connection test not supported for sink type %q", spec.Type)
	}
}
