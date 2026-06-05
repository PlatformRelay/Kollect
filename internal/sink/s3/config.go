// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"fmt"
	"net/url"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

type Config struct {
	Bucket          string
	Prefix          string
	Region          string
	Endpoint        string
	ForcePathStyle  bool
	AccessKeyID     string
	SecretAccessKey string
	Cluster         string
	Format          string
	HotAttributes   []string
}

func ConfigFromSpec(
	spec kollectdevv1alpha1.KollectSinkSpec,
	creds map[string][]byte,
) (Config, error) {
	if spec.Type != "s3" {
		return Config{}, fmt.Errorf("expected s3 sink, got %q", spec.Type)
	}

	endpoint := strings.TrimSpace(spec.Endpoint)
	if endpoint == "" {
		return Config{}, fmt.Errorf("s3 sink requires spec.endpoint")
	}

	cfg := Config{Region: "us-east-1", ForcePathStyle: true}
	parseEndpoint(endpoint, &cfg)

	if v, ok := creds["accessKeyID"]; ok {
		cfg.AccessKeyID = string(v)
	}

	if v, ok := creds["AWS_ACCESS_KEY_ID"]; ok && cfg.AccessKeyID == "" {
		cfg.AccessKeyID = string(v)
	}

	if v, ok := creds["secretAccessKey"]; ok {
		cfg.SecretAccessKey = string(v)
	}

	if v, ok := creds["AWS_SECRET_ACCESS_KEY"]; ok && cfg.SecretAccessKey == "" {
		cfg.SecretAccessKey = string(v)
	}

	if cfg.Bucket == "" {
		return Config{}, fmt.Errorf("s3 endpoint must include a bucket")
	}

	cfg.Cluster = strings.TrimSpace(spec.Cluster)
	if spec.ObjectStore != nil {
		cfg.Format = strings.ToLower(strings.TrimSpace(spec.ObjectStore.Format))
		cfg.HotAttributes = append([]string(nil), spec.ObjectStore.HotAttributes...)
	}

	return cfg, nil
}

func parseEndpoint(endpoint string, cfg *Config) {
	if strings.HasPrefix(endpoint, "s3://") {
		rest := strings.TrimPrefix(endpoint, "s3://")
		parts := strings.SplitN(rest, "/", 2)
		cfg.Bucket = parts[0]
		if len(parts) == 2 {
			cfg.Prefix = strings.TrimSuffix(parts[1], "/")
		}

		return
	}

	u, err := url.Parse(endpoint)
	if err == nil && u.Scheme != "" && u.Host != "" {
		cfg.Endpoint = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		path := strings.Trim(u.Path, "/")
		if path != "" {
			segments := strings.SplitN(path, "/", 2)
			cfg.Bucket = segments[0]
			if len(segments) == 2 {
				cfg.Prefix = strings.TrimSuffix(segments[1], "/")
			}
		}

		return
	}

	parts := strings.SplitN(endpoint, "/", 2)
	cfg.Bucket = parts[0]
	if len(parts) == 2 {
		cfg.Prefix = strings.TrimSuffix(parts[1], "/")
	}
}
