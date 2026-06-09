// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestTLSConfigFromSpec_insecureSkip(t *testing.T) {
	t.Parallel()

	cfg, err := TLSConfigFromSpec(&kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true}, nil)
	if err != nil {
		t.Fatalf("TLSConfigFromSpec: %v", err)
	}

	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify true")
	}

	tlsCfg, err := cfg.ClientConfig()
	if err != nil {
		t.Fatalf("ClientConfig: %v", err)
	}
	if tlsCfg == nil || !tlsCfg.InsecureSkipVerify {
		t.Error("client config should inherit insecure skip verify")
	}
}

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

func TestTLSConfigFromSpec_cabundle(t *testing.T) {
	t.Parallel()

	cfg, err := TLSConfigFromSpec(&kollectdevv1alpha1.TLSSpec{CABundle: testCAPEM(t)}, nil)
	if err != nil {
		t.Fatalf("TLSConfigFromSpec: %v", err)
	}

	if !cfg.Enabled() {
		t.Fatal("expected TLS enabled with CA bundle")
	}

	tlsCfg, err := cfg.ClientConfig()
	if err != nil {
		t.Fatalf("ClientConfig: %v", err)
	}
	if tlsCfg == nil || tlsCfg.RootCAs == nil {
		t.Fatal("expected RootCAs from CA bundle")
	}
}

func TestTLSConfigFromSpec_invalidCABundle(t *testing.T) {
	t.Parallel()

	_, err := TLSConfigFromSpec(&kollectdevv1alpha1.TLSSpec{CABundle: []byte("not-pem")}, nil)
	if err == nil {
		t.Fatal("expected error for invalid CA bundle")
	}
}

func TestTLSConfigFromSpec_offByDefault(t *testing.T) {
	t.Parallel()

	cfg, err := TLSConfigFromSpec(nil, nil)
	if err != nil {
		t.Fatalf("TLSConfigFromSpec: %v", err)
	}
	if cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify false by default")
	}
	if cfg.Enabled() {
		t.Error("expected TLS disabled when no CA and no insecure skip")
	}
}
