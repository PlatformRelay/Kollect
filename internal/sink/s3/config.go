// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"strings"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
)

// ErrCAPlainHTTP marks an s3 or gcs sink that configures a CA for an http endpoint,
// where the CA would never be checked.
var ErrCAPlainHTTP = errors.New("spec.tls: a CA is never checked over plain http; use an https endpoint")

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
	// RootCAs is the system roots plus the spec.tls CA; nil when no CA is configured.
	RootCAs *x509.CertPool
}

func ConfigFromSpec(
	spec kollectdevv1alpha1.KollectSinkSpec,
	creds map[string][]byte,
	caPEM []byte,
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

	if len(caPEM) > 0 {
		pool, err := sinktls.AppendedPool(caPEM)
		if err != nil {
			return Config{}, err
		}
		if err := requireHTTPS(cfg.Endpoint); err != nil {
			return Config{}, err
		}
		cfg.RootCAs = pool
	}

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

// requireHTTPS rejects an endpoint whose scheme is not https while a CA is configured;
// an empty endpoint leaves the choice to the SDK, which newClient checks after building.
// The error names the scheme only, never the endpoint.
func requireHTTPS(endpoint string) error {
	if endpoint == "" {
		return nil
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return kollecterrors.Terminal(fmt.Errorf("%w: the endpoint does not parse", ErrCAPlainHTTP))
	}
	if u.Scheme != "https" {
		return kollecterrors.Terminal(fmt.Errorf("%w (endpoint scheme %q)", ErrCAPlainHTTP, u.Scheme))
	}

	return nil
}
