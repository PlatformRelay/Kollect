// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package kafka

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

	"github.com/segmentio/kafka-go"

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

// TestNewBackend_populatesTransportTLS is the K-13 test lock: spec.tls must reach
// the kafka.Transport so broker traffic is encrypted instead of silently plaintext.
func TestNewBackend_populatesTransportTLS(t *testing.T) {
	t.Parallel()

	b, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type: "kafka",
		TLS:  &kollectdevv1alpha1.TLSSpec{CABundle: testCAPEM(t)},
		Kafka: &kollectdevv1alpha1.KafkaSpec{
			Brokers: []string{"broker:9092"},
			Topic:   "inventory",
		},
	}, nil, nil)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	writer, ok := b.writer.(*kafka.Writer)
	if !ok {
		t.Fatalf("writer is %T, want *kafka.Writer", b.writer)
	}
	transport, ok := writer.Transport.(*kafka.Transport)
	if !ok {
		t.Fatalf("transport is %T, want *kafka.Transport", writer.Transport)
	}
	if transport.TLS == nil {
		t.Fatal("transport.TLS is nil; spec.tls.caBundle was ignored (K-13)")
	}
	if transport.TLS.RootCAs == nil {
		t.Fatal("transport.TLS.RootCAs is nil; CA bundle not applied")
	}
	if transport.TLS.MinVersion == 0 {
		t.Fatal("transport.TLS.MinVersion is zero; expected TLS 1.2 floor")
	}
}

func TestNewBackend_noTLSByDefault(t *testing.T) {
	t.Parallel()

	b, err := NewBackend(kollectdevv1alpha1.KollectSinkSpec{
		Type:  "kafka",
		Kafka: &kollectdevv1alpha1.KafkaSpec{Brokers: []string{"broker:9092"}, Topic: "inventory"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	writer := b.writer.(*kafka.Writer)
	transport := writer.Transport.(*kafka.Transport)
	if transport.TLS != nil {
		t.Fatal("transport.TLS should be nil when spec.tls is unset")
	}
}

func TestTLSConfigFromSpec_invalidCABundle(t *testing.T) {
	t.Parallel()

	_, err := TLSConfigFromSpec(&kollectdevv1alpha1.TLSSpec{CABundle: []byte("not-pem")}, nil)
	if err == nil {
		t.Fatal("expected error for invalid CA bundle")
	}
}

// TestTLSConfigFromSpec_insecureDeniedByDefault pins the K-14 gate: without the
// process-wide opt-in, an insecureSkipVerify request is refused at construction,
// not silently honoured.
func TestTLSConfigFromSpec_insecureDeniedByDefault(t *testing.T) {
	validation.SetAllowInsecureSinks(false)
	t.Cleanup(func() { validation.SetAllowInsecureSinks(false) })

	if _, err := TLSConfigFromSpec(&kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true}, nil); err == nil {
		t.Fatal("expected insecureSkipVerify to be refused without --allow-insecure-sinks")
	}
}

func TestTLSConfigFromSpec_insecureAllowedWithOptIn(t *testing.T) {
	validation.SetAllowInsecureSinks(true)
	t.Cleanup(func() { validation.SetAllowInsecureSinks(false) })

	cfg, err := TLSConfigFromSpec(&kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true}, nil)
	if err != nil {
		t.Fatalf("TLSConfigFromSpec with opt-in: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify true with opt-in")
	}
	if client := cfg.ClientConfig(); client == nil || !client.InsecureSkipVerify {
		t.Fatal("expected ClientConfig to carry InsecureSkipVerify")
	}
}
