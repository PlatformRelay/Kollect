// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/secretkv"
)

// Static URL faults (K-25): a *url.ParseError echoes the raw server string,
// so neither message may %w-wrap it. Credentials belong in the token /
// username+password secret keys, never in the URL.
var (
	ErrInvalidServerURL  = errors.New("nats sink: invalid server URL")
	ErrURLCredentialsSet = errors.New("nats sink: server URL must not embed credentials; use token or username/password")
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

	if err := validateServerURL(url); err != nil {
		return Config{}, err
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

// validateServerURL fails closed on a malformed server entry and on any entry
// carrying userinfo (K-25): the credential never reaches an error string
// because the URL is rejected before connect and the messages are static.
// Comma-separated server lists are validated per entry.
func validateServerURL(servers string) error {
	for _, server := range strings.Split(servers, ",") {
		server = strings.TrimSpace(server)
		if server == "" {
			continue
		}

		// Bare host:port entries are interpreted by the NATS client as
		// nats://host:port, so validate them under the same scheme.
		toParse := server
		if !strings.Contains(toParse, "://") {
			toParse = "nats://" + toParse
		}

		u, err := url.Parse(toParse)
		if err != nil {
			return ErrInvalidServerURL
		}

		if u.User != nil {
			return ErrURLCredentialsSet
		}
	}

	return nil
}
