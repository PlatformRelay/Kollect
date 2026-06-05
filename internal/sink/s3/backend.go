// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/export"
	"github.com/konih/kollect/internal/sink/cap"
	"github.com/konih/kollect/internal/sink/objectstore"
	parquetenc "github.com/konih/kollect/internal/sink/parquet"
)

type Backend struct {
	cfg    Config
	client *awss3.Client
}

func NewBackend(spec kollectdevv1alpha1.KollectSinkSpec, creds map[string][]byte) (*Backend, error) {
	cfg, err := ConfigFromSpec(spec, creds)
	if err != nil {
		return nil, err
	}

	client, err := newClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Backend{cfg: cfg, client: client}, nil
}

func (b *Backend) Type() string {
	return "s3"
}

func (b *Backend) Capabilities() cap.Capabilities {
	return cap.SnapshotStore()
}

func (b *Backend) Export(ctx context.Context, payload []byte, objectPath string) error {
	objectPath = strings.TrimSpace(objectPath)
	if objectPath == "" {
		objectPath = "inventory/latest.json"
	}

	body := payload
	contentType := "application/json"

	if b.cfg.Format == objectstore.FormatParquet {
		items, err := export.ItemsFromPayload(payload)
		if err != nil {
			return fmt.Errorf("s3 parquet export: decode payload: %w", err)
		}

		invNS, invName := objectstore.InventoryFromObjectPath(objectPath)
		cluster := b.cfg.Cluster
		if cluster == "" {
			cluster = "default"
		}

		encoded, err := parquetenc.EncodeItems(items, parquetenc.EncodeOptions{
			Cluster:            cluster,
			InventoryNamespace: invNS,
			InventoryName:      invName,
			HotAttributes:      b.cfg.HotAttributes,
			ExportedAt:         time.Now().UTC(),
		})
		if err != nil {
			return fmt.Errorf("s3 parquet export: encode: %w", err)
		}

		body = encoded
		contentType = parquetenc.ContentType()
	} else if len(payload) == 0 {
		return fmt.Errorf("s3 export: empty payload")
	}

	key := objectPath
	if b.cfg.Prefix != "" {
		key = path.Join(b.cfg.Prefix, objectPath)
	}

	_, err := b.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(b.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3 PutObject: %w", err)
	}

	return nil
}

func newClient(cfg Config) (*awss3.Client, error) {
	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}

	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = cfg.ForcePathStyle
		}
	}), nil
}
