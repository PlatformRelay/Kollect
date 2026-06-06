// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"fmt"
	"net/url"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

type CommitAuthor struct {
	Name  string
	Email string
}

type Config struct {
	Endpoint      string
	TLS           TLSConfig
	SSH           SSHConfig
	CABundle      []byte
	Cluster       string
	Branch        string
	PushPolicy    PushPolicy
	CommitMessage string
	Author        CommitAuthor
	CloneDepth    int
	Prune         bool
	AuthType      AuthType
}

func ConfigFromSpec(spec kollectdevv1alpha1.KollectSinkSpec, caPEM []byte) (Config, error) {
	if spec.Type != TypeName {
		return Config{}, fmt.Errorf("expected git sink, got %q", spec.Type)
	}

	if err := ValidateTLSSpec(spec.TLS); err != nil {
		return Config{}, err
	}

	endpoint := strings.TrimSpace(spec.Endpoint)
	if endpoint == "" {
		return Config{}, fmt.Errorf("git sink requires spec.endpoint")
	}

	if _, err := url.Parse(endpoint); err != nil {
		return Config{}, fmt.Errorf("parse endpoint: %w", err)
	}

	tlsCfg, err := TLSConfigFromSpec(spec.TLS, caPEM)
	if err != nil {
		return Config{}, err
	}

	pem := caPEM
	if len(pem) == 0 && spec.TLS != nil {
		pem = spec.TLS.CABundle
	}

	cfg := Config{
		Endpoint:      endpoint,
		TLS:           tlsCfg,
		CABundle:      pem,
		Cluster:       strings.TrimSpace(spec.Cluster),
		PushPolicy:    PushPolicyCommit,
		CommitMessage: defaultCommitMessage,
		Author: CommitAuthor{
			Name:  defaultAuthorName,
			Email: defaultAuthorEmail,
		},
		CloneDepth: defaultCloneDepth,
		AuthType:   AuthTypeToken,
	}

	if spec.Git != nil {
		if applyErr := applyGitSpec(&cfg, spec.Git); applyErr != nil {
			return Config{}, applyErr
		}
	}

	authType, err := resolveAuthType(cfg.Endpoint, spec.Git)
	if err != nil {
		return Config{}, err
	}

	cfg.AuthType = authType

	return cfg, nil
}

func applyGitSpec(cfg *Config, gitSpec *kollectdevv1alpha1.GitSpec) error {
	if branch := strings.TrimSpace(gitSpec.Branch); branch != "" {
		if err := ValidateGitRef(branch); err != nil {
			return fmt.Errorf("git branch: %w", err)
		}

		cfg.Branch = branch
	}

	switch strings.TrimSpace(gitSpec.PushPolicy) {
	case "", kollectdevv1alpha1.GitPushPolicyCommit:
		cfg.PushPolicy = PushPolicyCommit
	case kollectdevv1alpha1.GitPushPolicyForcePush:
		cfg.PushPolicy = PushPolicyForcePush
	default:
		return fmt.Errorf("unsupported git pushPolicy %q", gitSpec.PushPolicy)
	}

	if msg := strings.TrimSpace(gitSpec.CommitMessage); msg != "" {
		cfg.CommitMessage = msg
	}

	if gitSpec.Author != nil {
		if name := strings.TrimSpace(gitSpec.Author.Name); name != "" {
			cfg.Author.Name = name
		}

		if email := strings.TrimSpace(gitSpec.Author.Email); email != "" {
			cfg.Author.Email = email
		}
	}

	if gitSpec.CloneDepth != nil {
		depth := int(*gitSpec.CloneDepth)
		if depth < 1 {
			return fmt.Errorf("git cloneDepth must be >= 1")
		}

		cfg.CloneDepth = depth
	}

	cfg.Prune = gitSpec.Prune

	return nil
}

func resolveAuthType(endpoint string, gitSpec *kollectdevv1alpha1.GitSpec) (AuthType, error) {
	if gitSpec != nil && gitSpec.Auth != nil {
		switch strings.TrimSpace(gitSpec.Auth.Type) {
		case "", kollectdevv1alpha1.GitAuthTypeToken:
			return AuthTypeToken, nil
		case kollectdevv1alpha1.GitAuthTypeSSH:
			return AuthTypeSSH, nil
		default:
			return "", fmt.Errorf("unsupported git auth type %q", gitSpec.Auth.Type)
		}
	}

	_, _, scheme, err := parseEndpoint(endpoint)
	if err != nil {
		return "", err
	}

	if scheme == schemeSSH {
		return AuthTypeSSH, nil
	}

	return AuthTypeToken, nil
}

func parseEndpoint(endpoint string) (cloneURL, branch, scheme string, err error) {
	cloneURL, branch, err = parseRemote(endpoint)
	if err != nil {
		return "", "", "", err
	}

	u, err := url.Parse(cloneURL)
	if err != nil {
		return "", "", "", err
	}

	return cloneURL, branch, u.Scheme, nil
}

func (c Config) EffectiveBranch(defaultBranch string) string {
	if c.Branch != "" {
		return c.Branch
	}

	return defaultBranch
}

func (c Config) withDefaults() Config {
	if c.PushPolicy == "" {
		c.PushPolicy = PushPolicyCommit
	}

	if c.CommitMessage == "" {
		c.CommitMessage = defaultCommitMessage
	}

	if c.Author.Name == "" {
		c.Author.Name = defaultAuthorName
	}

	if c.Author.Email == "" {
		c.Author.Email = defaultAuthorEmail
	}

	if c.CloneDepth <= 0 {
		c.CloneDepth = defaultCloneDepth
	}

	if c.AuthType == "" {
		c.AuthType = AuthTypeToken
	}

	return c
}
