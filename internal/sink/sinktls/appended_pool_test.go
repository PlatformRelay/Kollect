// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sinktls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"strings"
	"testing"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// P1: a leaf signed by the configured CA verifies against the appended pool.
func TestAppendedPool_trustsConfiguredCA(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	caY := sinktlstest.NewCA(t, "Y")

	pool, err := AppendedPool(caX.PEM)
	if err != nil {
		t.Fatalf("AppendedPool: %v", err)
	}

	verify := func(ca *sinktlstest.CA) error {
		_, err := ca.Server(t, "localhost").Leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: "localhost"})
		return err
	}

	if err := verify(caX); err != nil {
		t.Fatalf("leaf signed by the configured CA X: %v", err)
	}

	if err := verify(caY); err == nil {
		t.Fatal("leaf signed by CA Y verified; the pool trusts a CA nobody configured")
	}
}

// P2: the pool keeps the system roots. The parent re-executes this binary with root
// P as the only system root; the child checks that a P-signed leaf still verifies.
func TestAppendedPool_keepsSystemRoots(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")
	rootP := sinktlstest.NewCA(t, "P")
	leafP := rootP.Server(t, "localhost")

	sinktlstest.RunWithSystemRoots(t, "TestAppendedPoolChild_keepsSystemRoots", rootP.PEM,
		"SINKTLS_TEST_CA_PEM="+string(caX.PEM),
		"SINKTLS_TEST_LEAF_PEM="+string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafP.Leaf.Raw})),
	)
}

func TestAppendedPoolChild_keepsSystemRoots(t *testing.T) {
	if !sinktlstest.InChild() {
		t.Skip("child of TestAppendedPool_keepsSystemRoots only")
	}

	block, _ := pem.Decode([]byte(os.Getenv("SINKTLS_TEST_LEAF_PEM")))
	if block == nil {
		t.Fatal("no leaf PEM from the parent")
	}

	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}

	pool, err := AppendedPool([]byte(os.Getenv("SINKTLS_TEST_CA_PEM")))
	if err != nil {
		t.Fatalf("AppendedPool: %v", err)
	}

	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: "localhost"}); err != nil {
		t.Fatalf("leaf signed by system root P: %v (the CA replaced the system roots)", err)
	}
}

// P3: PEM that yields no certificate is a terminal error naming spec.tls that does
// not echo the input.
func TestAppendedPool_rejectsPEMWithoutCertificate(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"garbage":          "not a certificate",
		"empty":            "",
		"whitespace":       " \n\t\n",
		"private key only": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool, err := AppendedPool([]byte(input))
			if err == nil {
				t.Fatalf("AppendedPool accepted %s as a CA (pool %v)", name, pool != nil)
			}

			if !errors.Is(err, ErrNoCACertificate) {
				t.Errorf("error %q is not ErrNoCACertificate", err)
			}

			if !kollecterrors.IsTerminal(err) {
				t.Errorf("error %q is not terminal", err)
			}

			if !strings.Contains(err.Error(), "spec.tls") {
				t.Errorf("error %q does not name spec.tls", err)
			}

			if trimmed := strings.TrimSpace(input); trimmed != "" && strings.Contains(err.Error(), trimmed) {
				t.Errorf("error echoes the input: %q", err)
			}
		})
	}
}
