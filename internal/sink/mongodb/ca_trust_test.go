// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/dns"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/netguard"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// leakSentinels are the URI parts the errors this change adds must never contain.
var leakSentinels = []string{"sentinel-user", "sentinel-pass", "sentinel-host", "sentinel-db"}

func caTrustUnitSpec() kollectdevv1alpha1.KollectSinkSpec {
	return kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "items",
		},
	}
}

// failOnDNS replaces the driver's resolver functions with ones that fail t, so a
// test proves that no SRV or TXT lookup ran. Tests that call it must not be parallel.
func failOnDNS(t *testing.T) {
	t.Helper()

	r := dns.DefaultResolver
	srv, txt := r.LookupSRV, r.LookupTXT
	r.LookupSRV = func(_, _, name string) (string, []*net.SRV, error) {
		t.Errorf("DNS SRV lookup ran for a URI under test")
		return "", nil, errors.New("dns disabled by test")
	}
	r.LookupTXT = func(string) ([]string, error) {
		t.Errorf("DNS TXT lookup ran for a URI under test")
		return nil, errors.New("dns disabled by test")
	}

	t.Cleanup(func() { r.LookupSRV, r.LookupTXT = srv, txt })
}

// buildAndProbe runs NewBackend and TestConnection on uri with caPEM and returns
// both errors. A short deadline keeps an unexpected connect attempt bounded.
func buildAndProbe(t *testing.T, uri string, caPEM []byte) (buildErr, probeErr error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	secret := map[string][]byte{"uri": []byte(uri)}

	backend, buildErr := NewBackend(ctx, caTrustUnitSpec(), secret, caPEM)
	if backend != nil {
		backend.Close()
	}

	return buildErr, TestConnection(ctx, caTrustUnitSpec(), secret, caPEM)
}

// requireCAError asserts err wraps want, is terminal, names param and spec.tls, and
// carries no sentinel.
func requireCAError(t *testing.T, what string, err, want error, param string) {
	t.Helper()

	if err == nil {
		t.Errorf("%s: no error; want %v", what, want)
		return
	}

	if !errors.Is(err, want) {
		t.Errorf("%s: error %q does not wrap %q", what, err, want)
	}

	if !kollecterrors.IsTerminal(err) {
		t.Errorf("%s: error %q is not terminal", what, err)
	}

	for _, s := range []string{param, "spec.tls"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("%s: error %q does not name %q", what, err, s)
		}
	}

	for _, s := range leakSentinels {
		if strings.Contains(err.Error(), s) {
			t.Errorf("%s: error %q leaks %q", what, err, s)
		}
	}
}

//nolint:gosec // G101: sentinel values the error-text assertions look for, not credentials
const sentinelURI = "mongodb://sentinel-user:sentinel-pass@sentinel-host/sentinel-db"

// M1, M2: a URI CA file next to spec.tls is rejected in every spelling, on build and probe.
func TestMongoCATrust_M1M2_uriCAFileRejected(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")

	for _, tc := range []struct {
		query string
		param string
	}{
		{"?tls=true&tlsCAFile=/x", "tlsCAFile"},
		{"?tls=true&sslCertificateAuthorityFile=/x", "sslCertificateAuthorityFile"},
		{"?tls=true&TLSCAFILE=/x", "tlsCAFile"},
		{"?tls=true&tlsCAFile=/a&tlsCAFile=/b", "tlsCAFile"},
		{"?tls=true&tlsCAFile=", "tlsCAFile"},
		{"?tls=true;tlsCAFile=/x", "tlsCAFile"},
		{"?tls=true&tls%43AFile=/x", "tlsCAFile"},
		{"?tlsCAFile=/x", "tlsCAFile"},
	} {
		buildErr, probeErr := buildAndProbe(t, sentinelURI+tc.query, caX.PEM)
		requireCAError(t, tc.query+" build", buildErr, ErrCAFileConflict, tc.param)
		requireCAError(t, tc.query+" probe", probeErr, ErrCAFileConflict, tc.param)
	}

	// A query the reader cannot read is refused too, without echoing the URI.
	buildErr, probeErr := buildAndProbe(t, sentinelURI+"?tls=true&sentinel-host=%zz", caX.PEM)
	requireCAError(t, "unreadable option build", buildErr, ErrCAURIInvalid, "spec.tls")
	requireCAError(t, "unreadable option probe", probeErr, ErrCAURIInvalid, "spec.tls")
}

// M3: TLS off is rejected, and the SRV case does no DNS lookup.
func TestMongoCATrust_M3_tlsOffRejectedWithoutDNS(t *testing.T) {
	failOnDNS(t)

	caX := sinktlstest.NewCA(t, "X")

	for _, tc := range []struct {
		uri   string
		param string
	}{
		{"mongodb://sentinel-host/", "tls"},
		{"mongodb://sentinel-host", "tls"},
		{"mongodb://sentinel-host/?tls=false", "tls"},
		{"mongodb://sentinel-host/?ssl=false", "ssl"},
		{"mongodb://sentinel-host/?TLS=false", "tls"},
		{"mongodb://sentinel-host/?tlsCertificateKeyFile=/k&tls=false", "tls"},
		{"mongodb+srv://sentinel-host.invalid/?tls=false", "tls"},
		{"mongodb+srv://sentinel-host.invalid/?ssl=false", "ssl"},
	} {
		buildErr, probeErr := buildAndProbe(t, tc.uri, caX.PEM)
		requireCAError(t, tc.uri+" build", buildErr, ErrCATLSOff, tc.param)
		requireCAError(t, tc.uri+" probe", probeErr, ErrCATLSOff, tc.param)
	}
}

// M4: verification turned off by any alias is rejected; an explicit false is accepted.
func TestMongoCATrust_M4_insecureRejected(t *testing.T) {
	failOnDNS(t)

	caX := sinktlstest.NewCA(t, "X")

	for _, tc := range []struct {
		query string
		param string
	}{
		{"?tls=true&tlsInsecure=true", "tlsInsecure"},
		{"?tls=true&sslInsecure=true", "sslInsecure"},
		{"?tls=true&TLSINSECURE=true", "tlsInsecure"},
		{"?tls=true&tlsInsecure=false&tlsInsecure=true", "tlsInsecure"},
		{"?tls=true&tlsInsecure=maybe", "tlsInsecure"},
		{"?tls=true&tlsInsecure=", "tlsInsecure"},
		{"?tls=true&tlsAllowInvalidCertificates=true", "tlsAllowInvalidCertificates"},
		{"?tls=true&TLSALLOWINVALIDHOSTNAMES=true", "tlsAllowInvalidHostnames"},
		{"?tls=true&tlsAllowInvalidHostnames=maybe", "tlsAllowInvalidHostnames"},
	} {
		buildErr, probeErr := buildAndProbe(t, sentinelURI+tc.query, caX.PEM)
		requireCAError(t, tc.query+" build", buildErr, ErrCAInsecure, tc.param)
		requireCAError(t, tc.query+" probe", probeErr, ErrCAInsecure, tc.param)
	}

	for _, uri := range []string{
		"mongodb://h/?tls=true&tlsInsecure=false",
		"mongodb://h/?tls=true&sslInsecure=false",
		"mongodb://h/?tls=true&tlsInsecure=true&tlsInsecure=false",
		"mongodb://h/?tls=true&tlsAllowInvalidCertificates=false&tlsAllowInvalidHostnames=false",
	} {
		if err := checkCAURI(uri); err != nil {
			t.Errorf("checkCAURI(%q) = %v; want accepted", uri, err)
		}

		opts, err := clientOptions(uri, caX.PEM)
		if err != nil {
			t.Errorf("clientOptions(%q) = %v; want accepted", uri, err)
			continue
		}

		if opts.TLSConfig == nil || opts.TLSConfig.RootCAs == nil || opts.TLSConfig.InsecureSkipVerify {
			t.Errorf("clientOptions(%q): TLSConfig %+v; want the CA pool and verification on", uri, opts.TLSConfig)
		}
	}
}

// M5: garbage CA PEM is a terminal error before any connect.
func TestMongoCATrust_M5_garbagePEMRejected(t *testing.T) {
	t.Parallel()

	for _, caPEM := range [][]byte{[]byte("not a certificate"), []byte("  \n")} {
		buildErr, probeErr := buildAndProbe(t, "mongodb://127.0.0.1:1/?tls=true", caPEM)
		for name, err := range map[string]error{"build": buildErr, "probe": probeErr} {
			if !errors.Is(err, sinktls.ErrNoCACertificate) || !kollecterrors.IsTerminal(err) {
				t.Errorf("%s with PEM %q: error %v; want terminal %v", name, caPEM, err, sinktls.ErrNoCACertificate)
			}
		}
	}
}

// writeClientCertKey writes a combined client certificate and key PEM signed by ca to
// a temp file, as tlsCertificateKeyFile expects.
func writeClientCertKey(t *testing.T, ca *sinktlstest.CA) string {
	t.Helper()

	cert := ca.Server(t, "client")
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	data = append(data, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})...)

	path := filepath.Join(t.TempDir(), "client.pem")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// M6: the CA pool goes on the driver-built TLS config and the URI's client
// certificate stays; without a CA the options are what they were.
func TestMongoCATrust_M6_clientOptionsKeepURISettings(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	caY := sinktlstest.NewCA(t, "Y")
	uri := "mongodb://h/?tls=true&tlsCertificateKeyFile=" + writeClientCertKey(t, caY)

	opts, err := clientOptions(uri, caX.PEM)
	if err != nil {
		t.Fatalf("clientOptions with CA: %v", err)
	}

	tc := opts.TLSConfig
	if tc == nil || tc.RootCAs == nil {
		t.Fatalf("TLSConfig %+v; want RootCAs set", tc)
	}

	if len(tc.Certificates) != 1 {
		t.Errorf("client certificates = %d; want the URI's one", len(tc.Certificates))
	}

	if tc.InsecureSkipVerify {
		t.Error("InsecureSkipVerify set")
	}

	verify := func(ca *sinktlstest.CA) error {
		_, verr := ca.Server(t).Leaf.Verify(x509.VerifyOptions{Roots: tc.RootCAs, DNSName: "localhost"})
		return verr
	}

	if verr := verify(caX); verr != nil {
		t.Errorf("leaf signed by X does not verify against the pool: %v", verr)
	}

	if verify(caY) == nil {
		t.Error("leaf signed by Y verifies; want only system roots plus X")
	}

	if opts.Dialer != netguard.DefaultDialer {
		t.Error("dialer is not netguard's")
	}

	// No-op control: without a CA the options match today's construction.
	plain, err := clientOptions(uri, nil)
	if err != nil {
		t.Fatalf("clientOptions without CA: %v", err)
	}

	today := options.Client().ApplyURI(uri)
	if plain.TLSConfig == nil || plain.TLSConfig.RootCAs != nil || len(plain.TLSConfig.Certificates) != len(today.TLSConfig.Certificates) {
		t.Errorf("no-CA TLSConfig %+v; want today's %+v", plain.TLSConfig, today.TLSConfig)
	}

	if plain.Dialer != netguard.DefaultDialer {
		t.Error("no-CA dialer is not netguard's")
	}

	off, err := clientOptions("mongodb://h/", nil)
	if err != nil || off.TLSConfig != nil {
		t.Errorf("no-CA plaintext URI: err %v, TLSConfig %+v; want unchanged (no TLS, no error)", err, off.TLSConfig)
	}
}

// M7: the post-ApplyURI backstop rejects a driver config the key check should have
// stopped: a URI CA file, verification off, or no TLS.
func TestMongoCATrust_M7_driverConfigBackstop(t *testing.T) {
	t.Parallel()

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, sinktlstest.NewCA(t, "Z").PEM, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		uri   string
		want  error
		param string
	}{
		{"mongodb://h/?tlsCAFile=" + caFile, ErrCAFileConflict, "tlsCAFile"},
		{"mongodb://h/?tls=true&tlsInsecure=true", ErrCAInsecure, "tlsInsecure"},
		{"mongodb://h/", ErrCATLSOff, "tls"},
	} {
		opts := options.Client().ApplyURI(tc.uri)
		if err := opts.Validate(); err != nil {
			t.Fatalf("ApplyURI(%q): %v", tc.uri, err)
		}

		requireCAError(t, tc.uri, checkDriverTLS(opts.TLSConfig), tc.want, tc.param)
	}

	if err := checkDriverTLS(options.Client().ApplyURI("mongodb://h/?tls=true").TLSConfig); err != nil {
		t.Errorf("tls=true: %v; want accepted", err)
	}

	// Through the build path with a readable CA file: rejected whichever check fires.
	caX := sinktlstest.NewCA(t, "X")
	buildErr, probeErr := buildAndProbe(t, "mongodb://127.0.0.1:1/?tls=true&tlsCAFile="+caFile, caX.PEM)
	requireCAError(t, "readable tlsCAFile build", buildErr, ErrCAFileConflict, "tlsCAFile")
	requireCAError(t, "readable tlsCAFile probe", probeErr, ErrCAFileConflict, "tlsCAFile")
}

// M8: default TLS on SRV and ssl=true are accepted without a DNS lookup.
func TestMongoCATrust_M8_srvAndSSLAcceptedWithoutDNS(t *testing.T) {
	failOnDNS(t)

	for _, uri := range []string{
		"mongodb+srv://h.invalid/",
		"mongodb+srv://h.invalid",
		"mongodb+srv://u:p@h.invalid/db?retryWrites=true",
		"mongodb://h/?ssl=true",
		"mongodb://h/?tls=true",
		"mongodb://h/?tlsCertificateKeyFile=/k",
		"mongodb://h/?tlsDisableOCSPEndpointCheck=true",
	} {
		if err := checkCAURI(uri); err != nil {
			t.Errorf("checkCAURI(%q) = %v; want accepted", uri, err)
		}
	}
}

// readURITLS must read a URI the way the driver does. Compared against
// connstring.Parse on non-SRV URIs (no DNS), including edge spellings: present but
// empty keys, escapes, ';' separators, '@' and '?' in odd places.
func TestMongoCATrust_readerMatchesDriver(t *testing.T) {
	t.Parallel()

	for _, uri := range []string{
		"mongodb://h/",
		"mongodb://h",
		"mongodb://h/db",
		"mongodb://h/?tls=true",
		"mongodb://h/db?tls=true&tlsCAFile=/a",
		"mongodb://h/?tlsCAFile=",
		"mongodb://h/?TlsCaFile=/a&tls=false",
		"mongodb://h/?tls=true&tlsInsecure=true&tlsInsecure=false",
		"mongodb://h/?tls=true;sslInsecure=true",
		"mongodb://h/?tls%3Dtrue",
		"mongodb://h/?tl%73=true",
		"mongodb://h/?tls=tr%75e",
		"mongodb://h/?tls+=true",
		"mongodb://h/?&&tls=true&&",
		"mongodb://h/?tls=true&=x",
		"mongodb://h/?tls",
		"mongodb://h/?tls=true&a=%zz",
		"mongodb://h?tls=true",
		"mongodb://u:p%3Fq@h/?tls=true",
		"mongodb://u:p@h/?appName=a@b&tls=true",
		"mongodb://h/?appName=a@b&tls=true",
		"mongodb://h/?tls=true&tlsCAFile=/x&appName=a@b",
		"mongodb://u:p:x@h/?tls=true",
		"mongodb://u/x:p@h/?tls=true",
		"mongodb://h/d%3Fb?tls=true",
		"mongodb://h1,h2/?ssl=true",
		"mongodb://h/?tlsCertificateKeyFile=/k",
		"mongodb://h/?tlsCertificateFile=/c&tlsPrivateKeyFile=/k",
		"mongodb://h/?sslClientCertificateKeyFile=/k&ssl=false",
		"mongodb://h/?tlsDisableOCSPEndpointCheck=false",
		"mongodb://h/?tls=true&ssl=false",
		"mongodb://h/?tls=TRUE",
		"mongodb://h/?tls=",
		"mongodb://h/?sslInsecure=",
		"MONGODB://h/?tls=true",
		"postgres://h/?tls=true",
	} {
		got, gotErr := readURITLS(uri)

		cs, csErr := connstring.Parse(uri)
		if csErr != nil {
			// The driver refuses the URI; the reader must refuse it too, or the
			// check would judge a URI the driver never reads that way.
			if gotErr == nil {
				t.Errorf("%q: reader accepted %+v; driver error %v", uri, got, csErr)
			}

			continue
		}

		if gotErr != nil {
			t.Errorf("%q: reader error %v; driver parsed it", uri, gotErr)
			continue
		}

		if got.tls != cs.SSL || (got.caFileKey != "") != cs.SSLCaFileSet || got.insecure != cs.SSLInsecure {
			t.Errorf("%q: reader tls=%v caFile=%q insecure=%v; driver SSL=%v SSLCaFileSet=%v SSLInsecure=%v",
				uri, got.tls, got.caFileKey, got.insecure, cs.SSL, cs.SSLCaFileSet, cs.SSLInsecure)
		}
	}
}

// SCT-3 "the errors this requirement adds SHALL NOT contain the URI, host or
// credentials": a URI the driver refuses after the TLS checks passed (a bad port, an
// SRV lookup failure, whose resolver error names the host) is reported without the
// driver's text, on build and probe.
func TestMongoCATrust_driverURIErrorIsTargetFree(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")

	r := dns.DefaultResolver
	srv, txt := r.LookupSRV, r.LookupTXT
	r.LookupSRV = func(_, _, name string) (string, []*net.SRV, error) {
		return "", nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	}
	r.LookupTXT = func(name string) ([]string, error) {
		return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	}
	t.Cleanup(func() { r.LookupSRV, r.LookupTXT = srv, txt })

	for _, uri := range []string{
		"mongodb://sentinel-user:sentinel-pass@sentinel-host:1:2/sentinel-db?tls=true",
		"mongodb+srv://sentinel-user:sentinel-pass@sentinel-host.invalid/sentinel-db",
	} {
		buildErr, probeErr := buildAndProbe(t, uri, caX.PEM)

		for name, err := range map[string]error{"build": buildErr, "probe": probeErr} {
			if err == nil {
				t.Errorf("%s: accepted; want the driver's refusal", name)
				continue
			}

			for _, s := range leakSentinels {
				if strings.Contains(err.Error(), s) {
					t.Errorf("%s: error %q leaks %q", name, err, s)
				}
			}
		}
	}
}
