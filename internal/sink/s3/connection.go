// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// TestConnection verifies bucket reachability via HeadBucket.
func TestConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	creds map[string][]byte,
) error {
	cfg, err := ConfigFromSpec(spec, creds)
	if err != nil {
		return err
	}

	client, err := newClient(cfg)
	if err != nil {
		return fmt.Errorf("s3 client: %w", err)
	}

	_, err = client.HeadBucket(ctx, &awss3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return fmt.Errorf("s3 HeadBucket %q: %w", cfg.Bucket, err)
	}

	return nil
}
