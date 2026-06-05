// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
)

// TLSSettings configures optional TLS for queue transport backends (ADR-0503 wire hardening).
type TLSSettings struct {
	InsecureSkipVerify bool
	CAFile             string
	ClientCertFile     string
	ClientKeyFile      string
}

// TLSSettingsFromEnv reads KOLLECT_TRANSPORT_TLS_* variables shared by Redis/NATS/Kafka clients.
func TLSSettingsFromEnv() TLSSettings {
	return TLSSettings{
		InsecureSkipVerify: envBool("KOLLECT_TRANSPORT_TLS_INSECURE_SKIP_VERIFY"),
		CAFile:             os.Getenv("KOLLECT_TRANSPORT_TLS_CA_FILE"),
		ClientCertFile:     os.Getenv("KOLLECT_TRANSPORT_TLS_CLIENT_CERT_FILE"),
		ClientKeyFile:      os.Getenv("KOLLECT_TRANSPORT_TLS_CLIENT_KEY_FILE"),
	}
}

// Enabled reports whether any TLS option is configured.
func (s TLSSettings) Enabled() bool {
	return s.InsecureSkipVerify || s.CAFile != "" || s.ClientCertFile != "" || s.ClientKeyFile != ""
}

// ClientConfig builds a crypto/tls.Config for outbound queue connections.
func (s TLSSettings) ClientConfig() (*tls.Config, error) {
	if !s.Enabled() {
		return nil, nil
	}

	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: s.InsecureSkipVerify, //nolint:gosec // dev-only via explicit env flag
	}

	if s.CAFile != "" {
		pem, err := os.ReadFile(s.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read transport tls ca file: %w", err)
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parse transport tls ca file %q", s.CAFile)
		}

		cfg.RootCAs = pool
	}

	if s.ClientCertFile != "" || s.ClientKeyFile != "" {
		if s.ClientCertFile == "" || s.ClientKeyFile == "" {
			return nil, fmt.Errorf("transport tls: client cert and key files must both be set")
		}

		cert, err := tls.LoadX509KeyPair(s.ClientCertFile, s.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load transport tls client cert: %w", err)
		}

		cfg.Certificates = []tls.Certificate{cert}
	}

	return cfg, nil
}

func envBool(key string) bool {
	v := os.Getenv(key)
	if v == "" {
		return false
	}

	ok, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}

	return ok
}
