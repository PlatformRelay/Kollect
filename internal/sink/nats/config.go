// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"fmt"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/sink/secretkv"
)

const TypeName = "nats"

const typeName = TypeName

const defaultStreamName = "kollect_events"

type Config struct {
	URL      string
	Subject  string
	Stream   string
	Cluster  string
	Token    string
	Username string
	Password string
}

func ConfigFromSpec(
	spec kollectdevv1alpha1.KollectSinkSpec,
	secretData map[string][]byte,
) (Config, error) {
	if spec.Type != typeName {
		return Config{}, fmt.Errorf("expected nats sink, got %q", spec.Type)
	}

	if spec.Nats == nil {
		return Config{}, fmt.Errorf("nats sink requires spec.nats")
	}

	n := spec.Nats
	url := strings.TrimSpace(n.URL)
	if url == "" {
		url = strings.TrimSpace(spec.Endpoint)
	}

	if url == "" {
		return Config{}, fmt.Errorf("nats sink requires spec.nats.url or spec.endpoint")
	}

	subject := strings.TrimSpace(n.Subject)
	if subject == "" {
		return Config{}, fmt.Errorf("nats sink requires spec.nats.subject")
	}

	stream := strings.TrimSpace(n.Stream)
	if stream == "" {
		stream = defaultStreamName
	}

	cfg := Config{
		URL:     url,
		Subject: subject,
		Stream:  sanitizeStreamName(stream),
		Cluster: strings.TrimSpace(spec.Cluster),
	}

	secretkv.AssignIfPresent(secretData, "token", &cfg.Token)
	secretkv.AssignIfPresent(secretData, "username", &cfg.Username)
	secretkv.AssignIfPresent(secretData, "password", &cfg.Password)

	return cfg, nil
}

func sanitizeStreamName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}
