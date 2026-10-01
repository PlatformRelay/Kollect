// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/platformrelay/kollect/internal/sink/objectstore"
)

// DeleteExport removes the inventory's exported objects: each candidate path
// exactly as the export wrote it, plus its deterministic .part-NNNN-of-NNNN
// siblings and (for parquet hive layout) every generation inside the
// inventory-owned partition directory (K-28, captain call C-2a). It returns the
// deleted keys relative to the sink prefix.
//
// Listing by prefix can also surface sibling inventories whose names merely
// share the prefix ("app" vs "app-v2"), so every key is checked against the
// matcher before deletion. Missing objects are not errors: cleanup is retried.
func (b *Backend) DeleteExport(ctx context.Context, paths []string) ([]string, error) {
	keyRoot := ""
	if b.cfg.Prefix != "" {
		keyRoot = strings.TrimSuffix(b.cfg.Prefix, "/") + "/"
	}

	var deleted []string

	for _, objectPath := range paths {
		objectPath = strings.TrimSpace(objectPath)
		if objectPath == "" {
			continue
		}

		for _, m := range objectstore.CleanupMatchers(objectPath) {
			got, err := b.deleteMatching(ctx, keyRoot, m)
			if err != nil {
				return deleted, err
			}
			deleted = append(deleted, got...)
		}
	}

	return deleted, nil
}

func (b *Backend) deleteMatching(ctx context.Context, keyRoot string, m objectstore.KeyMatcher) ([]string, error) {
	var deleted []string

	listPrefix := m.Prefix
	if b.cfg.Prefix != "" {
		trailing := strings.HasSuffix(listPrefix, "/")
		listPrefix = path.Join(b.cfg.Prefix, strings.TrimSuffix(listPrefix, "/"))
		if trailing {
			listPrefix += "/"
		}
	}

	var token *string

	for {
		out, err := b.client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
			Bucket:            aws.String(b.cfg.Bucket),
			Prefix:            aws.String(listPrefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("s3 cleanup list %q: %w", listPrefix, err)
		}

		for _, obj := range out.Contents {
			key := aws.ToString(obj.Key)
			rel := strings.TrimPrefix(key, keyRoot)
			if rel == key && keyRoot != "" {
				continue
			}
			if !m.Matches(rel) {
				continue
			}

			if _, err := b.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
				Bucket: aws.String(b.cfg.Bucket),
				Key:    aws.String(key),
			}); err != nil {
				return deleted, fmt.Errorf("s3 cleanup delete %q: %w", key, err)
			}

			deleted = append(deleted, rel)
		}

		if out.IsTruncated == nil || !*out.IsTruncated {
			return deleted, nil
		}

		token = out.NextContinuationToken
	}
}
