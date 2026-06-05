// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gitlab

import (
	"fmt"
	"net/url"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/git"
)

// Config holds resolved GitLab sink settings (HTTPS git remote).
type Config struct {
	Endpoint     string
	TLS          git.TLSConfig
	CABundle     []byte
	MergeRequest MergeRequestConfig
}

// ConfigFromSpec validates and resolves a KollectSink GitLab spec.
func ConfigFromSpec(spec kollectdevv1alpha1.KollectSinkSpec, caPEM []byte) (Config, error) {
	if spec.Type != TypeName {
		return Config{}, fmt.Errorf("expected gitlab sink, got %q", spec.Type)
	}

	if err := git.ValidateTLSSpec(spec.TLS); err != nil {
		return Config{}, err
	}

	endpoint := strings.TrimSpace(spec.Endpoint)
	if endpoint == "" {
		return Config{}, fmt.Errorf("gitlab sink requires spec.endpoint")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return Config{}, fmt.Errorf("parse endpoint: %w", err)
	}

	if !isHTTPSEndpointScheme(u.Scheme) {
		return Config{}, fmt.Errorf("gitlab endpoint must use https or http, got %q", u.Scheme)
	}

	tlsCfg, err := git.TLSConfigFromSpec(spec.TLS, caPEM)
	if err != nil {
		return Config{}, err
	}

	pem := caPEM
	if len(pem) == 0 && spec.TLS != nil {
		pem = spec.TLS.CABundle
	}

	mrCfg, err := mergeRequestConfigFromSpec(spec)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Endpoint:     endpoint,
		TLS:          tlsCfg,
		CABundle:     pem,
		MergeRequest: mrCfg,
	}, nil
}

func mergeRequestConfigFromSpec(spec kollectdevv1alpha1.KollectSinkSpec) (MergeRequestConfig, error) {
	if spec.GitLab == nil || spec.GitLab.MergeRequest == nil {
		return MergeRequestConfig{}, nil
	}

	mr := spec.GitLab.MergeRequest
	cfg := MergeRequestConfig{
		Mode:          MergeRequestMode(strings.TrimSpace(mr.Mode)),
		TargetBranch:  strings.TrimSpace(mr.TargetBranch),
		BranchPrefix:  strings.TrimSpace(mr.BranchPrefix),
		TitleTemplate: strings.TrimSpace(mr.TitleTemplate),
		AutoMerge:     mr.AutoMerge,
	}

	if err := ValidateMergeRequestConfig(cfg); err != nil {
		return MergeRequestConfig{}, err
	}

	return cfg, nil
}

// GitConfig converts GitLab settings to the shared git export config.
func (c Config) GitConfig() git.Config {
	return git.Config{
		Endpoint:      c.Endpoint,
		TLS:           c.TLS,
		CABundle:      c.CABundle,
		PushPolicy:    git.PushPolicyForcePush,
		CommitMessage: "kollect: export inventory",
		Author: git.CommitAuthor{
			Name:  "kollect",
			Email: "kollect@kollect.dev",
		},
		CloneDepth: 1,
		AuthType:   git.AuthTypeToken,
	}
}
