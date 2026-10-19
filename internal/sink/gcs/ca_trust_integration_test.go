//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package gcs

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// fakeXML answers HeadBucket and PutObject for path-style bucket "b" of the
// S3-compatible XML API and counts the objects written.
type fakeXML struct {
	mu   sync.Mutex
	puts int
}

func (f *fakeXML) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodHead && r.URL.Path == "/b":
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut:
		f.mu.Lock()
		f.puts++
		f.mu.Unlock()
		w.Header().Set("ETag", `"etag"`)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeXML) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.puts
}

func gcsExportAndProbe(t *testing.T, signer *sinktlstest.CA, caPEM []byte, f *fakeXML) (exportErr, probeErr error) {
	t.Helper()

	srv := httptest.NewUnstartedServer(f)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{signer.Server(t)}, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: TypeName, Endpoint: srv.URL + "/b"}
	creds := map[string][]byte{"accessKeyID": []byte("AKIDTEST"), "secretAccessKey": []byte("test-secret")}

	b, err := NewBackend(spec, creds, caPEM)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	return b.Export(t.Context(), []byte(`{"items":[]}`), "inventory/default/inv.json"), TestConnection(t.Context(), spec, creds, caPEM)
}

// G1 (I1 through the GCS wrapper): a server signed only by private CA X is trusted.
func TestCATrustIntegration_privateCATrusted(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")
	f := &fakeXML{}

	exportErr, probeErr := gcsExportAndProbe(t, caX, caX.PEM, f)
	if exportErr != nil || probeErr != nil {
		t.Fatalf("export = %v, probe = %v; want both to succeed", exportErr, probeErr)
	}
	if f.count() != 1 {
		t.Fatalf("server recorded %d objects, want 1", f.count())
	}
}

// G1 (I2 through the GCS wrapper): a server signed by CA Y is refused.
func TestCATrustIntegration_unknownCARefused(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")
	caY := sinktlstest.NewCA(t, "Y")
	f := &fakeXML{}

	exportErr, probeErr := gcsExportAndProbe(t, caY, caX.PEM, f)
	for name, err := range map[string]error{"export": exportErr, "probe": probeErr} {
		var verr *tls.CertificateVerificationError
		if !errors.As(err, &verr) {
			t.Fatalf("%s error = %v, want a certificate verification error", name, err)
		}
	}
	if f.count() != 0 {
		t.Fatalf("server signed by Y recorded %d objects", f.count())
	}
}
