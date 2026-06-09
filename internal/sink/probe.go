// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/bigquery"
	"github.com/konih/kollect/internal/sink/gcs"
	"github.com/konih/kollect/internal/sink/git"
	"github.com/konih/kollect/internal/sink/gitlab"
	kafkasink "github.com/konih/kollect/internal/sink/kafka"
	"github.com/konih/kollect/internal/sink/mongodb"
	natssink "github.com/konih/kollect/internal/sink/nats"
	"github.com/konih/kollect/internal/sink/postgres"
	s3sink "github.com/konih/kollect/internal/sink/s3"
)

type connectionTester func(context.Context, kollectdevv1alpha1.KollectSinkSpec, BuildContext) (string, error)

var connectionTesters = map[string]connectionTester{
	git.TypeName:       testGitConnection,
	gitlab.TypeName:    testGitLabConnection,
	postgres.TypeName:  testPostgresConnection,
	bigquery.TypeName:  testBigQueryConnection,
	mongodb.TypeName:   testMongoConnection,
	kafkasink.TypeName: testKafkaConnection,
	natssink.TypeName:  testNatsConnection,
	"s3":               testS3Connection,
	gcs.TypeName:       testGCSConnection,
}

// RunConnectionTest probes sink connectivity using the same backends as export.
func RunConnectionTest(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	tester, ok := connectionTesters[spec.Type]
	if !ok {
		return "", fmt.Errorf("connection test not supported for sink type %q", spec.Type)
	}

	return tester(ctx, spec, buildCtx)
}

func testGitConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	cfg, err := git.ConfigFromSpec(spec, buildCtx.CAPEM)
	if err != nil {
		return "", err
	}

	auth := GitAuthFromSecretData(buildCtx.SecretData, gitAuthTypeFromSpec(spec))
	if err := git.TestConnection(ctx, cfg, auth); err != nil {
		return "", err
	}

	return "TLS and git remote reachability verified", nil
}

func testGitLabConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	cfg, err := gitlab.ConfigFromSpec(spec, buildCtx.CAPEM)
	if err != nil {
		return "", err
	}

	auth := GitAuthFromSecretData(buildCtx.SecretData, "")
	if err := gitlab.TestConnection(ctx, cfg, auth); err != nil {
		return "", err
	}

	return "TLS and GitLab remote reachability verified", nil
}

func testPostgresConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	if err := postgres.TestConnection(ctx, spec, buildCtx.DatabaseSecretData); err != nil {
		return "", err
	}

	return "PostgreSQL ping succeeded", nil
}

func testBigQueryConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	if err := bigquery.TestConnection(ctx, spec, buildCtx.DatabaseSecretData); err != nil {
		return "", err
	}

	return "BigQuery dataset metadata and dry-run query succeeded", nil
}

func testMongoConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	if err := mongodb.TestConnection(ctx, spec, buildCtx.DatabaseSecretData); err != nil {
		return "", err
	}

	return "MongoDB ping succeeded", nil
}

func testKafkaConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	if err := kafkasink.TestConnection(ctx, spec, buildCtx.SecretData); err != nil {
		return "", err
	}

	return "Kafka broker metadata request succeeded", nil
}

func testNatsConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	if err := natssink.TestConnection(ctx, spec, buildCtx.SecretData, buildCtx.CAPEM); err != nil {
		return "", err
	}

	return "NATS JetStream account info request succeeded", nil
}

func testS3Connection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	if err := s3sink.TestConnection(ctx, spec, buildCtx.SecretData); err != nil {
		return "", err
	}

	return "S3 bucket HeadBucket succeeded", nil
}

func testGCSConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	buildCtx BuildContext,
) (string, error) {
	if err := gcs.TestConnection(ctx, spec, buildCtx.SecretData); err != nil {
		return "", err
	}

	return "GCS bucket HeadBucket succeeded", nil
}
