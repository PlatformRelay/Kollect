// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sinktls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/validation"
)

func testCAPEM(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "kollect-test-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func setAllowInsecure(t *testing.T, allow bool) {
	t.Helper()
	validation.SetAllowInsecureSinks(allow)
	t.Cleanup(func() { validation.SetAllowInsecureSinks(false) })
}

func TestFromSpec_nilSpecIsPlaintext(t *testing.T) {
	cfg, err := FromSpec(nil, nil)
	if err != nil {
		t.Fatalf("FromSpec(nil): %v", err)
	}
	if cfg.Enabled() {
		t.Fatal("nil spec must not enable TLS")
	}
	if cfg.ClientConfig() != nil {
		t.Fatal("ClientConfig must be nil when TLS is not enabled")
	}
}

// TestFromSpec_insecureDeniedWithSentinel pins the K-14 gate and its single error
// source: every sink package returns validation.ErrInsecureSinksNotAllowed.
func TestFromSpec_insecureDeniedWithSentinel(t *testing.T) {
	setAllowInsecure(t, false)

	_, err := FromSpec(&kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true}, nil)
	if !errors.Is(err, validation.ErrInsecureSinksNotAllowed) {
		t.Fatalf("FromSpec error = %v, want validation.ErrInsecureSinksNotAllowed", err)
	}
}

func TestFromSpec_insecureAllowedWithOptIn(t *testing.T) {
	setAllowInsecure(t, true)

	cfg, err := FromSpec(&kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true}, nil)
	if err != nil {
		t.Fatalf("FromSpec with opt-in: %v", err)
	}
	client := cfg.ClientConfig()
	if client == nil || !client.InsecureSkipVerify {
		t.Fatalf("ClientConfig = %+v, want InsecureSkipVerify", client)
	}
}

func TestFromSpec_resolvedCAWinsOverInlineBundle(t *testing.T) {
	cfg, err := FromSpec(&kollectdevv1alpha1.TLSSpec{CABundle: []byte("not-pem")}, testCAPEM(t))
	if err != nil {
		t.Fatalf("FromSpec: resolved CA PEM must take precedence over caBundle: %v", err)
	}
	client := cfg.ClientConfig()
	if client == nil || client.RootCAs == nil {
		t.Fatal("RootCAs not populated from resolved CA PEM")
	}
	if client.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want TLS 1.2 floor", client.MinVersion)
	}
}

func TestFromSpec_invalidCABundle(t *testing.T) {
	if _, err := FromSpec(&kollectdevv1alpha1.TLSSpec{CABundle: []byte("not-pem")}, nil); err == nil {
		t.Fatal("expected an error for an unparsable CA bundle")
	}
}

func TestHandshakeConfig_alwaysNonNil(t *testing.T) {
	cfg := Config{}
	hs := cfg.HandshakeConfig()
	if hs == nil {
		t.Fatal("HandshakeConfig must never be nil (system roots, verification on)")
	}
	if hs.InsecureSkipVerify || hs.MinVersion != tls.VersionTLS12 {
		t.Fatalf("HandshakeConfig = %+v, want verifying TLS 1.2 floor", hs)
	}
}
