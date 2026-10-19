// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"crypto/x509"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/netguard"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

var caTrustCreds = map[string][]byte{"accessKeyID": []byte("AKIDTEST"), "secretAccessKey": []byte("test-secret")}

func s3Spec(endpoint string) kollectdevv1alpha1.KollectSinkSpec {
	return kollectdevv1alpha1.KollectSinkSpec{Type: "s3", Endpoint: endpoint}
}

// requireBuildRejected asserts that NewBackend and TestConnection both return an error
// matching want, classified terminal. TestConnection returning the build error (and not a
// HeadBucket error) shows it returned before sending any request.
func requireBuildRejected(t *testing.T, spec kollectdevv1alpha1.KollectSinkSpec, caPEM []byte, want error) []error {
	t.Helper()

	_, buildErr := NewBackend(spec, caTrustCreds, caPEM)
	probeErr := TestConnection(t.Context(), spec, caTrustCreds, caPEM)

	for name, err := range map[string]error{"NewBackend": buildErr, "TestConnection": probeErr} {
		if !errors.Is(err, want) {
			t.Fatalf("%s error = %v, want %v", name, err, want)
		}
		if !kollecterrors.IsTerminal(err) {
			t.Fatalf("%s error class = %s, want terminal: %v", name, kollecterrors.ClassOf(err), err)
		}
		if strings.Contains(err.Error(), "HeadBucket") {
			t.Fatalf("%s sent a request before rejecting: %v", name, err)
		}
	}

	return []error{buildErr, probeErr}
}

// S1: a CA next to an http spec endpoint is rejected before any request.
func TestCATrust_httpSpecEndpointRejected(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")

	if _, err := ConfigFromSpec(s3Spec("http://h/b"), caTrustCreds, caX.PEM); !errors.Is(err, ErrCAPlainHTTP) {
		t.Fatalf("ConfigFromSpec error = %v, want %v", err, ErrCAPlainHTTP)
	}

	for _, err := range requireBuildRejected(t, s3Spec("http://h/b"), caX.PEM, ErrCAPlainHTTP) {
		if msg := err.Error(); !strings.Contains(msg, "spec.tls") || !strings.Contains(msg, "http") {
			t.Fatalf("error %q should name spec.tls and http", msg)
		}
	}
}

// isolateAWSEnv points the SDK's shared config at empty files and clears endpoint env
// variables, so only what a case sets reaches LoadDefaultConfig.
func isolateAWSEnv(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("write empty config: %v", err)
	}

	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_ENDPOINT_URL_S3", "")
	t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "")

	return dir
}

// S1b: for an s3:// endpoint the SDK picks the endpoint from the environment or shared
// config; an http one is rejected when a CA is configured, and the S3-specific source wins.
func TestCATrust_sdkEndpointFromEnvironment(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")

	cases := []struct {
		name      string
		env       map[string]string
		config    string
		caPEM     []byte
		wantError bool
	}{
		{name: "AWS_ENDPOINT_URL_S3 http", env: map[string]string{"AWS_ENDPOINT_URL_S3": "http://h"}, caPEM: caX.PEM, wantError: true},
		{name: "AWS_ENDPOINT_URL http", env: map[string]string{"AWS_ENDPOINT_URL": "http://h"}, caPEM: caX.PEM, wantError: true},
		{
			name:  "both set, S3-specific http wins",
			env:   map[string]string{"AWS_ENDPOINT_URL": "https://h", "AWS_ENDPOINT_URL_S3": "http://h"},
			caPEM: caX.PEM, wantError: true,
		},
		{
			name:  "both set, S3-specific https wins",
			env:   map[string]string{"AWS_ENDPOINT_URL": "http://h", "AWS_ENDPOINT_URL_S3": "https://h"},
			caPEM: caX.PEM, wantError: false,
		},
		{
			name:   "shared config services section http",
			config: "[default]\nservices = local\n\n[services local]\ns3 =\n  endpoint_url = http://h\n",
			caPEM:  caX.PEM, wantError: true,
		},
		{name: "no CA keeps an http SDK endpoint (no-op control)", env: map[string]string{"AWS_ENDPOINT_URL_S3": "http://h"}, wantError: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolateAWSEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if tc.config != "" {
				cfgFile := filepath.Join(dir, "config")
				if err := os.WriteFile(cfgFile, []byte(tc.config), 0o600); err != nil {
					t.Fatalf("write shared config: %v", err)
				}
				t.Setenv("AWS_CONFIG_FILE", cfgFile)
			}

			if tc.wantError {
				requireBuildRejected(t, s3Spec("s3://b"), tc.caPEM, ErrCAPlainHTTP)

				return
			}

			if _, err := NewBackend(s3Spec("s3://b"), caTrustCreds, tc.caPEM); err != nil {
				t.Fatalf("NewBackend: %v", err)
			}
		})
	}
}

// S2: CA PEM that holds no certificate is a terminal error before any request.
func TestCATrust_garbageCARejected(t *testing.T) {
	t.Parallel()

	const garbage = "not a certificate"

	for _, err := range requireBuildRejected(t, s3Spec("https://h/b"), []byte(garbage), sinktls.ErrNoCACertificate) {
		if strings.Contains(err.Error(), garbage) {
			t.Fatalf("error echoes the CA input: %v", err)
		}
	}
}

// guardedTransport returns the client's *http.Transport, failing unless the client is
// the *http.Client newClient installed.
func guardedTransport(t *testing.T, cfg Config) *http.Transport {
	t.Helper()

	client, err := newClient(cfg)
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}

	hc, ok := client.Options().HTTPClient.(*http.Client)
	if !ok {
		t.Fatalf("HTTPClient is %T, want *http.Client", client.Options().HTTPClient)
	}

	tr, ok := hc.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport is %T, want *http.Transport", hc.Transport)
	}

	if reflect.ValueOf(tr.DialContext).Pointer() != reflect.ValueOf(netguard.DefaultDialer.DialContext).Pointer() {
		t.Fatal("DialContext is not netguard's; the CA path bypasses the SSRF guard")
	}
	if tr.Proxy != nil {
		t.Fatal("Proxy is set; the CA path re-enables proxies")
	}

	return tr
}

// S3: with a CA the client keeps netguard's dialer and no proxy, and trusts the
// appended pool; without a CA, RootCAs stays nil.
func TestCATrust_clientKeepsGuardAndUsesPool(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")

	cfg, err := ConfigFromSpec(s3Spec("https://h/b"), caTrustCreds, caX.PEM)
	if err != nil {
		t.Fatalf("ConfigFromSpec: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("Config.RootCAs is nil with a CA configured")
	}

	tr := guardedTransport(t, cfg)
	if tr.TLSClientConfig == nil || tr.TLSClientConfig.RootCAs != cfg.RootCAs {
		t.Fatal("transport RootCAs is not the appended pool")
	}

	leaf := caX.Server(t, "h").Leaf
	if _, verr := leaf.Verify(x509.VerifyOptions{Roots: tr.TLSClientConfig.RootCAs, DNSName: "h"}); verr != nil {
		t.Fatalf("leaf signed by CA X does not verify against the transport pool: %v", verr)
	}

	noCA, err := ConfigFromSpec(s3Spec("https://h/b"), caTrustCreds, nil)
	if err != nil {
		t.Fatalf("ConfigFromSpec without CA: %v", err)
	}
	if tr := guardedTransport(t, noCA); tr.TLSClientConfig != nil && tr.TLSClientConfig.RootCAs != nil {
		t.Fatal("RootCAs set without a CA; no-CA behaviour changed")
	}
}
