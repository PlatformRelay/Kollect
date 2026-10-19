//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

const caTrustObject = "inventory/default/inv.json"

// tlsFakeS3 is a path-style S3 endpoint for bucket "b" that answers HeadBucket and
// PutObject and records every request as "METHOD path". With redirectTo set, it
// answers every request with a 307 to that base URL instead.
type tlsFakeS3 struct {
	mu         sync.Mutex
	requests   []string
	redirectTo string
}

func (f *tlsFakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	f.mu.Unlock()

	switch {
	case f.redirectTo != "":
		http.Redirect(w, r, f.redirectTo+r.URL.Path, http.StatusTemporaryRedirect) //nolint:gosec // G710: the test fake redirects to its own plaintext test server
	case r.Method == http.MethodHead && r.URL.Path == "/b":
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut:
		w.Header().Set("ETag", `"etag"`)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// recorded returns the requests seen so far, or only those with method when given.
func (f *tlsFakeS3) recorded(method ...string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []string
	for _, r := range f.requests {
		if len(method) == 0 || strings.HasPrefix(r, method[0]+" ") {
			out = append(out, r)
		}
	}

	return out
}

// startTLS serves f over HTTPS with a certificate for 127.0.0.1 signed by ca and
// returns the sink endpoint for bucket "b".
func startTLS(t *testing.T, ca *sinktlstest.CA, f *tlsFakeS3) string {
	t.Helper()

	srv := httptest.NewUnstartedServer(f)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{ca.Server(t)}, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	return srv.URL + "/b"
}

func exportAndProbe(t *testing.T, endpoint string, caPEM []byte) (exportErr, probeErr error) {
	t.Helper()

	b, err := NewBackend(s3Spec(endpoint), caTrustCreds, caPEM)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	return b.Export(t.Context(), []byte(`{"items":[]}`), caTrustObject), TestConnection(t.Context(), s3Spec(endpoint), caTrustCreds, caPEM)
}

// I1: a server signed only by private CA X is trusted for export and probe.
func TestCATrustIntegration_privateCATrusted(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")
	f := &tlsFakeS3{}

	exportErr, probeErr := exportAndProbe(t, startTLS(t, caX, f), caX.PEM)
	if exportErr != nil || probeErr != nil {
		t.Fatalf("export = %v, probe = %v; want both to succeed", exportErr, probeErr)
	}
	if got := f.recorded(http.MethodPut); len(got) != 1 || got[0] != "PUT /b/"+caTrustObject {
		t.Fatalf("server recorded %v, want one PutObject of %s", got, caTrustObject)
	}
}

// I2: a server signed by CA Y is refused with a verification error; nothing is written.
func TestCATrustIntegration_unknownCARefused(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")
	caY := sinktlstest.NewCA(t, "Y")
	f := &tlsFakeS3{}

	exportErr, probeErr := exportAndProbe(t, startTLS(t, caY, f), caX.PEM)
	for name, err := range map[string]error{"export": exportErr, "probe": probeErr} {
		var verr *tls.CertificateVerificationError
		if !errors.As(err, &verr) {
			t.Fatalf("%s error = %v, want a certificate verification error", name, err)
		}
	}
	if got := f.recorded(); len(got) != 0 {
		t.Fatalf("server signed by Y recorded %v", got)
	}
}

// I3: the appended pool keeps the system roots. The test re-runs itself as a child
// process with root P as the only system root and exports, with CA X configured, to a
// server signed by P. The child branch is part of this test, not a test of its own, so
// no integration test skips in the parent (task test-integration requires none).
func TestCATrustIntegration_systemRootsKept(t *testing.T) {
	if sinktlstest.InChild() {
		exportErr, probeErr := exportAndProbe(t, os.Getenv("S3_CATRUST_ENDPOINT"), []byte(os.Getenv("S3_CATRUST_CA")))
		if exportErr != nil || probeErr != nil {
			t.Fatalf("export = %v, probe = %v; want both to succeed against a system root", exportErr, probeErr)
		}

		return
	}

	caX := sinktlstest.NewCA(t, "X")
	rootP := sinktlstest.NewCA(t, "P")
	f := &tlsFakeS3{}

	sinktlstest.RunWithSystemRoots(t, t.Name(), rootP.PEM,
		"S3_CATRUST_ENDPOINT="+startTLS(t, rootP, f),
		"S3_CATRUST_CA="+string(caX.PEM),
	)

	if got := f.recorded(http.MethodPut); len(got) != 1 {
		t.Fatalf("server signed by P recorded %v, want one PutObject", got)
	}
}

// I4: a redirect from the trusted HTTPS server to plain http is not followed, for the
// probe's HeadBucket (which net/http would follow by default) and for PutObject.
func TestCATrustIntegration_plainRedirectRefused(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")

	plain := &tlsFakeS3{}
	plainSrv := httptest.NewServer(plain)
	t.Cleanup(plainSrv.Close)

	// The redirect target carries a marker that no error may echo: a refused redirect
	// names the setting, never the URL the server sent.
	const marker = "sentinel-redirect"
	f := &tlsFakeS3{redirectTo: plainSrv.URL + "/" + marker}
	endpoint := startTLS(t, caX, f)

	b, err := NewBackend(s3Spec(endpoint), caTrustCreds, caX.PEM)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	// net/http follows a 307 for HeadBucket (no body), so the probe reaches CheckRedirect.
	probeErr := TestConnection(t.Context(), s3Spec(endpoint), caTrustCreds, caX.PEM)
	if probeErr == nil {
		t.Fatal("probe followed a redirect to plain http and succeeded")
	}
	if !errors.Is(probeErr, ErrCAInsecureRedirect) || !kollecterrors.IsTerminal(probeErr) {
		t.Fatalf("probe error = %v (class %s), want terminal %v", probeErr, kollecterrors.ClassOf(probeErr), ErrCAInsecureRedirect)
	}
	// PutObject's body cannot be replayed, so net/http returns the 307 to the SDK
	// instead of following it; the export must still fail.
	if err := b.Export(t.Context(), []byte(`{"items":[]}`), caTrustObject); err == nil {
		t.Fatal("export followed a redirect to plain http and succeeded")
	}
	for name, err := range map[string]error{"probe": probeErr, "export": b.Export(t.Context(), []byte(`{"items":[]}`), caTrustObject)} {
		if text := err.Error(); strings.Contains(text, marker) || strings.Contains(text, strings.TrimPrefix(plainSrv.URL, "http://")) {
			t.Errorf("%s error %q names the redirect target", name, text)
		}
	}
	if len(f.recorded(http.MethodHead)) == 0 || len(f.recorded(http.MethodPut)) == 0 {
		t.Fatalf("the HTTPS server saw %v; the redirects were never issued", f.recorded())
	}
	if got := plain.recorded(); len(got) != 0 {
		t.Fatalf("plain http server received %v", got)
	}
}

// S1, S1b, S2 with a request counter: in this tier netguard admits loopback, so a
// client that skipped a check would reach the plain-http server. A CA with an http
// spec endpoint, an http SDK endpoint, or garbage CA PEM must send it no request.
func TestCATrustIntegration_rejectedBuildSendsNoRequest(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")

	plain := &tlsFakeS3{}
	plainSrv := httptest.NewServer(plain)
	t.Cleanup(plainSrv.Close)

	cases := []struct {
		name     string
		endpoint string
		env      string
		caPEM    []byte
		want     error
	}{
		{name: "http spec endpoint", endpoint: plainSrv.URL + "/b", caPEM: caX.PEM, want: ErrCAPlainHTTP},
		{name: "AWS_ENDPOINT_URL_S3 http", endpoint: "s3://b", env: "AWS_ENDPOINT_URL_S3", caPEM: caX.PEM, want: ErrCAPlainHTTP},
		{name: "AWS_ENDPOINT_URL http", endpoint: "s3://b", env: "AWS_ENDPOINT_URL", caPEM: caX.PEM, want: ErrCAPlainHTTP},
		{name: "garbage CA PEM", endpoint: plainSrv.URL + "/b", caPEM: []byte("not a certificate"), want: sinktls.ErrNoCACertificate},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateAWSEnv(t)
			if tc.env != "" {
				t.Setenv(tc.env, plainSrv.URL)
			}

			_, buildErr := NewBackend(s3Spec(tc.endpoint), caTrustCreds, tc.caPEM)
			probeErr := TestConnection(t.Context(), s3Spec(tc.endpoint), caTrustCreds, tc.caPEM)
			if got := plain.recorded(); len(got) != 0 {
				t.Fatalf("plain http server received %v", got)
			}
			for name, err := range map[string]error{"NewBackend": buildErr, "TestConnection": probeErr} {
				if !errors.Is(err, tc.want) || !kollecterrors.IsTerminal(err) {
					t.Fatalf("%s error = %v, want terminal %v", name, err, tc.want)
				}
			}
		})
	}
}
