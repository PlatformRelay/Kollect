// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gcs

import (
	"context"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/s3"
)

func TestConnection(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	creds map[string][]byte,
	caPEM []byte,
) error {
	clone := spec
	clone.Type = "s3"

	return s3.TestConnection(ctx, clone, creds, caPEM)
}
