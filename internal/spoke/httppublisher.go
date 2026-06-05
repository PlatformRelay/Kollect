// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// HTTPPublisher posts spoke reports to a hub ingest endpoint with Kubernetes bearer auth.
type HTTPPublisher struct {
	URL     string
	Cluster string
	Client  *http.Client
}

// NewHTTPPublisher builds a publisher for hub HTTP ingest (ADR-0503).
func NewHTTPPublisher(url, cluster string) *HTTPPublisher {
	return &HTTPPublisher{
		URL:     url,
		Cluster: cluster,
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Publish sends payload to the hub ingest URL.
func (p *HTTPPublisher) Publish(ctx context.Context, _ string, payload []byte) error {
	if p == nil || p.URL == "" {
		return fmt.Errorf("spoke http publish: URL is required")
	}

	token, err := serviceAccountToken()
	if err != nil {
		return fmt.Errorf("spoke http publish token: %w", err)
	}

	// Hub ingest URL comes from operator/KollectHub config (not arbitrary user input).
	//nolint:gosec // G704: intentional outbound POST to configured hub endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("spoke http publish request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(kollectdevv1alpha1.HeaderClusterID, p.Cluster)
	req.Header.Set("Content-Type", "application/json")

	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}

	//nolint:gosec // G704: same configured hub endpoint as request above
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("spoke http publish: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if len(body) > 0 {
			return fmt.Errorf("spoke http publish: status %d: %s", resp.StatusCode, string(body))
		}

		return fmt.Errorf("spoke http publish: status %d", resp.StatusCode)
	}

	return nil
}
