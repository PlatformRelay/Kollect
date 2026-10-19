//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

const tlsVerificationText = "TLS certificate verification failed"

func caTrustSpec() kollectdevv1alpha1.KollectSinkSpec {
	return kollectdevv1alpha1.KollectSinkSpec{
		Type:    "postgres",
		Cluster: "test-cluster",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "ca_trust_items",
			Schema:      "public",
		},
	}
}

// exportAndProbe builds a backend for dsn with caPEM, exports one item, and runs the
// connection probe. It returns the build-or-export error and the probe error.
func exportAndProbe(t *testing.T, dsn string, caPEM []byte) (exportErr, probeErr error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	secret := map[string][]byte{"dsn": []byte(dsn)}
	probeErr = TestConnection(ctx, caTrustSpec(), secret, caPEM)

	backend, err := NewBackend(ctx, caTrustSpec(), secret, caPEM)
	if err != nil {
		return err, probeErr
	}
	defer backend.Close()

	payload, err := json.Marshal([]collect.Item{{
		TargetNamespace: "apps", TargetName: "web", Namespace: "apps", Name: "demo",
		Version: "v1", Kind: "Deployment", UID: "uid-ca",
	}})
	if err != nil {
		t.Fatal(err)
	}

	return backend.Export(ctx, payload, "inventory/apps/demo.json"), probeErr
}

func requireSuccess(t *testing.T, what string, exportErr, probeErr error) {
	t.Helper()

	if exportErr != nil {
		t.Errorf("%s: export: %v", what, exportErr)
	}

	if probeErr != nil {
		t.Errorf("%s: probe: %v", what, probeErr)
	}
}

func requireVerificationFailure(t *testing.T, what string, exportErr, probeErr error) {
	t.Helper()

	for name, err := range map[string]error{"export": exportErr, "probe": probeErr} {
		if err == nil {
			t.Errorf("%s: %s succeeded; want a certificate verification error", what, name)
			continue
		}

		if !strings.Contains(err.Error(), tlsVerificationText) {
			t.Errorf("%s: %s error %q does not say %q", what, name, err, tlsVerificationText)
		}
	}
}

// PI1: a server signed only by private CA X is trusted with CA X and verify-full.
func TestPostgresCATrust_PI1_verifyFullPrivateCA(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	srv := sinktlstest.StartTLSPostgres(t, caX.Server(t))

	exportErr, probeErr := exportAndProbe(t, srv.DSN("sslmode=verify-full"), caX.PEM)
	requireSuccess(t, "verify-full, server signed by X", exportErr, probeErr)
}

// PI2: a server signed by Y is refused with CA X under require and verify-ca.
func TestPostgresCATrust_PI2_wrongCARefused(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	caY := sinktlstest.NewCA(t, "Y")
	srv := sinktlstest.StartTLSPostgres(t, caY.Server(t))

	for _, mode := range []string{"require", "verify-ca"} {
		exportErr, probeErr := exportAndProbe(t, srv.DSN("sslmode="+mode), caX.PEM)
		requireVerificationFailure(t, mode+", server signed by Y", exportErr, probeErr)
	}
}

// PI5: a certificate signed by X for another hostname passes the chain-only modes
// and fails verify-full on the hostname.
func TestPostgresCATrust_PI5_hostnameRulePerMode(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	srv := sinktlstest.StartTLSPostgres(t, caX.Server(t, "other.invalid"))

	for _, mode := range []string{"require", "verify-ca"} {
		exportErr, probeErr := exportAndProbe(t, srv.DSN("sslmode="+mode), caX.PEM)
		requireSuccess(t, mode+", SAN other.invalid", exportErr, probeErr)
	}

	exportErr, probeErr := exportAndProbe(t, srv.DSN("sslmode=verify-full"), caX.PEM)
	requireVerificationFailure(t, "verify-full, SAN other.invalid", exportErr, probeErr)
}

// PI6: with a CA and no sslmode the real connection verifies the hostname: the
// mismatched server fails, the matching one succeeds.
func TestPostgresCATrust_PI6_unsetSSLModeVerifiesHostname(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	mismatched := sinktlstest.StartTLSPostgres(t, caX.Server(t, "other.invalid"))
	matching := sinktlstest.StartTLSPostgres(t, caX.Server(t))

	exportErr, probeErr := exportAndProbe(t, mismatched.DSN(""), caX.PEM)
	requireVerificationFailure(t, "no sslmode, SAN other.invalid", exportErr, probeErr)

	exportErr, probeErr = exportAndProbe(t, mismatched.DSN("sslmode=verify-ca"), caX.PEM)
	requireSuccess(t, "explicit verify-ca, SAN other.invalid", exportErr, probeErr)

	exportErr, probeErr = exportAndProbe(t, matching.DSN(""), caX.PEM)
	requireSuccess(t, "no sslmode, matching SAN", exportErr, probeErr)
}

// PI3: the CA is appended to the system roots. The test re-runs itself as a child
// process with root P as the only system root, against a server signed by P, with CA
// X configured. The child branch is part of this test, not a test of its own, so no
// integration test skips in the parent (task test-integration requires none).
func TestPostgresCATrust_PI3_keepsSystemRoots(t *testing.T) {
	if sinktlstest.InChild() {
		exportErr, probeErr := exportAndProbe(t, os.Getenv("PG_CA_TRUST_DSN"), []byte(os.Getenv("PG_CA_TRUST_CA")))
		requireSuccess(t, "verify-full, server signed by system root P, CA X", exportErr, probeErr)

		return
	}

	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	rootP := sinktlstest.NewCA(t, "P")
	srv := sinktlstest.StartTLSPostgres(t, rootP.Server(t))

	sinktlstest.RunWithSystemRoots(t, t.Name(), rootP.PEM,
		"PG_CA_TRUST_DSN="+srv.DSN("sslmode=verify-full"),
		"PG_CA_TRUST_CA="+string(caX.PEM),
	)
}
