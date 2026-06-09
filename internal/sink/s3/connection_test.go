// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestTestConnection_missingBucket(t *testing.T) {
	t.Parallel()

	err := TestConnection(t.Context(), kollectdevv1alpha1.KollectSinkSpec{
		Type:     "s3",
		Endpoint: "",
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty endpoint")
	}
}

func TestTestConnection_invalidEndpoint(t *testing.T) {
	t.Parallel()

	err := TestConnection(t.Context(), kollectdevv1alpha1.KollectSinkSpec{
		Type:     "s3",
		Endpoint: "s3://",
	}, nil)
	if err == nil {
		t.Fatal("expected error for bucket-less endpoint")
	}
}

type fakeHeadBucketClient struct {
	err       error
	gotBucket string
}

func (f *fakeHeadBucketClient) HeadBucket(_ context.Context, params *awss3.HeadBucketInput, _ ...func(*awss3.Options)) (*awss3.HeadBucketOutput, error) {
	if params != nil && params.Bucket != nil {
		f.gotBucket = aws.ToString(params.Bucket)
	}
	if f.err != nil {
		return nil, f.err
	}

	return &awss3.HeadBucketOutput{}, nil
}

func TestTestConnectionWithClient_HeadBucketError(t *testing.T) {
	t.Parallel()

	client := &fakeHeadBucketClient{err: errors.New("access denied")}
	cfg := Config{Bucket: "inventory"}
	err := testConnectionWithClient(t.Context(), cfg, client)
	if err == nil || !strings.Contains(err.Error(), "s3 HeadBucket") {
		t.Fatalf("testConnectionWithClient() error = %v, want wrapped HeadBucket error", err)
	}
	if client.gotBucket != "inventory" {
		t.Fatalf("bucket = %q, want inventory", client.gotBucket)
	}
}

func TestTestConnectionWithClient_Success(t *testing.T) {
	t.Parallel()

	client := &fakeHeadBucketClient{}
	cfg := Config{Bucket: "inventory"}
	if err := testConnectionWithClient(t.Context(), cfg, client); err != nil {
		t.Fatalf("testConnectionWithClient() error = %v", err)
	}
}
