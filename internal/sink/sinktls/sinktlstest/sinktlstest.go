// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package sinktlstest issues throwaway certificate authorities and server certificates
// for sink TLS tests, and re-executes a test binary with a stand-in system root pool.
// Keys live in memory only; nothing is written to the repository.
package sinktlstest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// childEnv marks a process started by RunWithSystemRoots.
const childEnv = "KOLLECT_SINKTLSTEST_CHILD"

// certBlockType is the PEM block type of a certificate.
const certBlockType = "CERTIFICATE"

// CA is a test certificate authority.
type CA struct {
	Cert *x509.Certificate
	PEM  []byte
	key  *ecdsa.PrivateKey
}

// NewCA returns a self-signed ECDSA P-256 CA valid for one hour either side of now.
func NewCA(t testing.TB, name string) *CA {
	t.Helper()

	key := newKey(t)
	tmpl := &x509.Certificate{
		SerialNumber:          serial(t),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA %s: %v", name, err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA %s: %v", name, err)
	}

	return &CA{Cert: cert, PEM: pem.EncodeToMemory(&pem.Block{Type: certBlockType, Bytes: der}), key: key}
}

// Server issues a server certificate signed by ca for hosts (DNS names or IP
// addresses); with no hosts it covers localhost and 127.0.0.1. The returned
// certificate has Leaf set.
func (ca *CA) Server(t testing.TB, hosts ...string) tls.Certificate {
	t.Helper()

	if len(hosts) == 0 {
		hosts = []string{"localhost", "127.0.0.1"}
	}

	key := newKey(t)
	tmpl := &x509.Certificate{
		SerialNumber: serial(t),
		Subject:      pkix.Name{CommonName: hosts[0]},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatalf("create server certificate: %v", err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse server certificate: %v", err)
	}

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}

// InChild reports whether this process was started by RunWithSystemRoots.
func InChild() bool {
	return os.Getenv(childEnv) == "1"
}

// RunWithSystemRoots re-executes the current test binary for the single top-level
// test childTest, with rootPEM as the only system root (SSL_CERT_FILE, and
// SSL_CERT_DIR pointing at an empty directory, since an empty value keeps Go's
// default directories). env adds KEY=VALUE pairs. It fails t unless the child
// exits 0 and reports both the RUN and the PASS line for childTest.
func RunWithSystemRoots(t testing.TB, childTest string, rootPEM []byte, env ...string) {
	t.Helper()

	dir := t.TempDir()
	rootFile := filepath.Join(dir, "root.pem")
	if err := os.WriteFile(rootFile, rootPEM, 0o600); err != nil {
		t.Fatalf("write root: %v", err)
	}

	emptyDir := filepath.Join(dir, "certs")
	if err := os.Mkdir(emptyDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	//nolint:gosec // G204: re-executes this test binary with a fixed test name
	cmd := exec.Command(os.Args[0], "-test.v", "-test.count=1", "-test.run", "^"+childTest+"$")
	cmd.Env = append(os.Environ(), childEnv+"=1", "SSL_CERT_FILE="+rootFile, "SSL_CERT_DIR="+emptyDir)
	cmd.Env = append(cmd.Env, env...)

	out, err := cmd.CombinedOutput()
	text := string(out)

	if err != nil || !strings.Contains(text, "=== RUN   "+childTest) || !strings.Contains(text, "--- PASS: "+childTest) {
		t.Fatalf("child %s: exit %v\n%s", childTest, err, text)
	}
}

func newKey(t testing.TB) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	return key
}

func serial(t testing.TB) *big.Int {
	t.Helper()

	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}

	return n
}
