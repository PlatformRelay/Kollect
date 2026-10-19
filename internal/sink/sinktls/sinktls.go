// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package sinktls resolves a sink's spec.tls into client TLS settings. It is the
// single implementation shared by the git, GitLab, Kafka and NATS sinks, so the
// K-14 insecure-transport gate and CA handling cannot drift between them.
package sinktls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/validation"
)

// Config holds resolved TLS settings for outbound sink connections.
type Config struct {
	InsecureSkipVerify bool
	RootCAs            *x509.CertPool
}

// FromSpec builds Config from the sink spec and the CA PEM the caller resolved
// (sink.CheckCASources rejects a spec that sets both caBundle and caSecretRef, so
// at most one source reaches here; the inline caBundle is the fallback when caPEM
// is empty). The pool holds only that CA. It refuses insecureSkipVerify unless the
// process-wide --allow-insecure-sinks opt-in is set (K-14), so the gate holds even
// where no admission webhook runs.
func FromSpec(tlsSpec *kollectdevv1alpha1.TLSSpec, caPEM []byte) (Config, error) {
	cfg := Config{}
	if tlsSpec == nil {
		return cfg, nil
	}
	if err := validation.CheckInsecureSkipVerify(tlsSpec); err != nil {
		return cfg, err
	}

	cfg.InsecureSkipVerify = tlsSpec.InsecureSkipVerify

	pem := caPEM
	if len(pem) == 0 {
		pem = tlsSpec.CABundle
	}
	if len(pem) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return cfg, errors.New("tls: failed to parse CA bundle PEM")
		}
		cfg.RootCAs = pool
	}

	return cfg, nil
}

// Enabled reports whether the config carries any TLS material. Protocols that
// default to plaintext (Kafka, NATS) switch to TLS only when this is true.
func (c Config) Enabled() bool {
	return c.InsecureSkipVerify || c.RootCAs != nil
}

// ClientConfig returns a *tls.Config when TLS material is configured, else nil
// so a plaintext-by-default protocol keeps its default.
func (c Config) ClientConfig() *tls.Config {
	if !c.Enabled() {
		return nil
	}

	return c.HandshakeConfig()
}

// HandshakeConfig always returns a *tls.Config, for protocols that are TLS by
// scheme (HTTPS git remotes): system roots unless a CA is configured.
func (c Config) HandshakeConfig() *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.InsecureSkipVerify, //nolint:gosec // gated by --allow-insecure-sinks (K-14)
		RootCAs:            c.RootCAs,
	}
}

// ErrCABothSources marks a spec.tls that sets both caBundle and caSecretRef.
var ErrCABothSources = errors.New("spec.tls sets both caBundle and caSecretRef; set exactly one")

// CheckCASources rejects a spec.tls that names two CA sources. It lives here so the
// git and GitLab config check and sink.CheckCASources return the same error.
func CheckCASources(tlsSpec *kollectdevv1alpha1.TLSSpec) error {
	if tlsSpec != nil && len(tlsSpec.CABundle) > 0 && tlsSpec.CASecretRef != nil {
		return kollecterrors.Terminal(ErrCABothSources)
	}

	return nil
}

// ErrNoCACertificate marks CA PEM from spec.tls that yields no certificate.
var ErrNoCACertificate = errors.New("spec.tls: the CA bundle holds no PEM certificate")

// AppendedPool returns the system roots with the certificates in caPEM appended, the
// rule go-git applies to a Git CABundle. Unlike go-git, PEM that adds no certificate
// (garbage, empty, only a key) is a terminal error, and the error never echoes the PEM.
// It is the pool for the S3, GCS, Postgres and MongoDB sinks; FromSpec (Kafka, NATS,
// the Git probe) keeps its CA-only pool.
func AppendedPool(caPEM []byte) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}

	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, kollecterrors.Terminal(ErrNoCACertificate)
	}

	return pool, nil
}
