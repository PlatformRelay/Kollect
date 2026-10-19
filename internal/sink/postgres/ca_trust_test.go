// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// sentinelKV is a keyword DSN whose every identifying value is a sentinel that no
// error may contain (sink-ca-trust test rules).
const sentinelKV = "host=sentinel-host user=sentinel-user password=sentinel-pass dbname=sentinel-db"

// sentinelURL is sentinelKV in URL form, without a query. It is assembled from parts
// so the source carries no user:password@ URL literal (gosec G101).
var sentinelURL = "postgres://" + "sentinel-user" + ":" + "sentinel-pass" + "@sentinel-host/sentinel-db"

var sentinels = []string{"sentinel-host", "sentinel-user", "sentinel-pass", "sentinel-db"}

// isolatePGEnv points HOME at an empty directory and clears every PG* variable that
// feeds pgx's TLS settings, so the host's own files and environment cannot leak in.
// It returns the HOME directory. Tests that call it cannot run in parallel.
func isolatePGEnv(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	for _, k := range []string{
		"PGSSLMODE", "PGSSLROOTCERT", "PGSERVICE", "PGSERVICEFILE",
		"PGSSLNEGOTIATION", "PGSSLCERT", "PGSSLKEY", "PGHOST", "PGPORT",
	} {
		t.Setenv(k, "")
	}

	return home
}

// assertRejected checks the shape every CA-conflict error shares: terminal, naming
// each of wants, and free of the DSN's sentinel values.
func assertRejected(t *testing.T, err error, sentinel error, wants ...string) {
	t.Helper()

	if err == nil {
		t.Fatalf("accepted; want a terminal error naming %v", wants)
	}

	if sentinel != nil && !errors.Is(err, sentinel) {
		t.Errorf("error %q is not %v", err, sentinel)
	}

	if !kollecterrors.IsTerminal(err) {
		t.Errorf("error %q is not terminal", err)
	}

	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not name %q", err, w)
		}
	}

	for _, s := range sentinels {
		if strings.Contains(err.Error(), s) {
			t.Errorf("error %q leaks DSN value %q", err, s)
		}
	}
}

func writeFile(t *testing.T, path string, data []byte) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// tlsConfigs returns the main TLS config followed by every fallback's, with each
// config's host.
func tlsConfigs(cfg *pgxpool.Config) (hosts []string, configs []*tls.Config) {
	hosts = append(hosts, cfg.ConnConfig.Host)
	configs = append(configs, cfg.ConnConfig.TLSConfig)

	for _, fb := range cfg.ConnConfig.Fallbacks {
		hosts = append(hosts, fb.Host)
		configs = append(configs, fb.TLSConfig)
	}

	return hosts, configs
}

// D1, D2: a DSN sslrootcert next to a sink CA is a terminal conflict that names
// sslrootcert and spec.tls and echoes nothing of the DSN.
func TestPoolConfig_rejectsDSNRootCertWithCA(t *testing.T) {
	isolatePGEnv(t)
	caX := sinktlstest.NewCA(t, "X")

	for name, dsn := range map[string]string{
		"keyword": sentinelKV + " sslmode=verify-full sslrootcert=/x",
		"url":     sentinelURL + "?sslmode=verify-full&sslrootcert=/x",
		"system":  sentinelKV + " sslmode=verify-full sslrootcert=system",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := poolConfig(dsn, caX.PEM)
			assertRejected(t, err, ErrCARootConflict, "sslrootcert", "spec.tls", "DSN")
		})
	}
}

// D3: an explicit weak sslmode in the DSN is rejected naming the mode, its source
// and the accepted modes; the verifying modes are accepted.
func TestPoolConfig_explicitSSLModeWithCA(t *testing.T) {
	isolatePGEnv(t)
	caX := sinktlstest.NewCA(t, "X")

	for _, mode := range []string{"disable", "allow", "prefer"} {
		for form, dsn := range map[string]string{
			"keyword": sentinelKV + " sslmode=" + mode,
			"url":     sentinelURL + "?sslmode=" + mode,
		} {
			t.Run(mode+"/"+form, func(t *testing.T) {
				_, err := poolConfig(dsn, caX.PEM)
				assertRejected(t, err, ErrCAWeakSSLMode,
					`"`+mode+`"`, "DSN", "verify-full", "verify-ca", "require", "spec.tls")
			})
		}
	}

	for _, mode := range []string{"require", "verify-ca", "verify-full"} {
		t.Run(mode, func(t *testing.T) {
			if _, err := poolConfig(sentinelKV+" sslmode="+mode, caX.PEM); err != nil {
				t.Fatalf("sslmode=%s with a CA: %v", mode, err)
			}
		})
	}
}

// D3b: with a CA and no sslmode anywhere, the sink verifies chain and hostname on
// every host and keeps no plaintext or unverified fallback.
func TestPoolConfig_unsetSSLModeBecomesVerifyFull(t *testing.T) {
	isolatePGEnv(t)
	caX := sinktlstest.NewCA(t, "X")

	for name, dsn := range map[string]string{
		"keyword":          sentinelKV,
		"url":              sentinelURL,
		"url with query":   sentinelURL + "?application_name=kollect",
		"direct":           sentinelKV + " sslnegotiation=direct",
		"url direct":       sentinelURL + "?sslnegotiation=direct",
		"two hosts":        "host=sentinel-host,sentinel-host2 user=sentinel-user dbname=sentinel-db",
		"two hosts in url": "postgres://sentinel-user@sentinel-host,sentinel-host2/sentinel-db",
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := poolConfig(dsn, caX.PEM)
			if err != nil {
				t.Fatalf("poolConfig: %v", err)
			}

			hosts, configs := tlsConfigs(cfg)
			for i, c := range configs {
				if c == nil {
					t.Fatalf("config %d (%s) is plaintext", i, hosts[i])
				}

				if c.InsecureSkipVerify {
					t.Errorf("config %d (%s): InsecureSkipVerify, want verify-full", i, hosts[i])
				}

				if c.ServerName != hosts[i] {
					t.Errorf("config %d: ServerName %q, want %q", i, c.ServerName, hosts[i])
				}
			}
		})
	}
}

// D4: a root certificate from any source other than spec.tls is a conflict naming
// that source, whether the file is valid, missing or malformed.
func TestPoolConfig_rejectsOtherRootSourcesWithCA(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")
	caOther := sinktlstest.NewCA(t, "other")
	dsn := sentinelKV + " sslmode=verify-full"

	files := map[string]func(dir string) string{
		"valid":   func(dir string) string { return writeFile(t, filepath.Join(dir, "valid.pem"), caOther.PEM) },
		"missing": func(dir string) string { return filepath.Join(dir, "missing.pem") },
		"malformed": func(dir string) string {
			return writeFile(t, filepath.Join(dir, "bad.pem"), []byte("not a certificate"))
		},
	}

	for name, file := range files {
		t.Run("PGSSLROOTCERT "+name, func(t *testing.T) {
			home := isolatePGEnv(t)
			t.Setenv("PGSSLROOTCERT", file(home))

			_, err := poolConfig(dsn, caX.PEM)
			assertRejected(t, err, ErrCARootConflict, "PGSSLROOTCERT", "spec.tls")
		})

		for _, selectBy := range []string{"DSN service", "PGSERVICE"} {
			t.Run("service file "+name+" via "+selectBy, func(t *testing.T) {
				home := isolatePGEnv(t)
				svc := writeFile(t, filepath.Join(home, "svc.conf"),
					[]byte("[s1]\nsslrootcert="+file(home)+"\n"))
				t.Setenv("PGSERVICEFILE", svc)

				d := dsn
				if selectBy == "PGSERVICE" {
					t.Setenv("PGSERVICE", "s1")
				} else {
					d += " service=s1"
				}

				_, err := poolConfig(d, caX.PEM)
				assertRejected(t, err, ErrCARootConflict, "sslrootcert", "service file", "spec.tls")
			})
		}
	}

	t.Run("home root.crt", func(t *testing.T) {
		home := isolatePGEnv(t)
		writeFile(t, filepath.Join(home, ".postgresql", "root.crt"), caOther.PEM)

		_, err := poolConfig(dsn, caX.PEM)
		assertRejected(t, err, ErrCARootConflict, "root.crt", "spec.tls")
	})
}

// An explicit service key in the DSN selects the entry even when empty, overriding
// PGSERVICE, as pgx does; the entry's sslmode then counts as explicit.
func TestPoolConfig_serviceSelectionMatchesPgx(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")

	cases := []struct {
		name, dsn, pgservice string
		wantFull             bool
	}{
		{"empty DSN service overrides PGSERVICE", sentinelKV + " service='' sslnegotiation=direct", "s", true},
		{"PGSERVICE selects the entry", sentinelKV + " sslnegotiation=direct", "s", false},
		{"DSN service selects the entry", sentinelKV + " service=s", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := isolatePGEnv(t)
			t.Setenv("PGSERVICEFILE", writeFile(t, filepath.Join(home, "svc.conf"),
				[]byte("[s]\nsslmode=require\n\n[]\napplication_name=empty\n")))
			t.Setenv("PGSERVICE", tc.pgservice)

			cfg, err := poolConfig(tc.dsn, caX.PEM)
			if err != nil {
				t.Fatalf("poolConfig: %v", err)
			}

			hosts, configs := tlsConfigs(cfg)
			for i, c := range configs {
				if c == nil {
					t.Fatalf("config %d (%s) is plaintext", i, hosts[i])
				}

				if full := !c.InsecureSkipVerify; full != tc.wantFull {
					t.Errorf("config %d (%s): verify-full = %v, want %v", i, hosts[i], full, tc.wantFull)
				}
			}
		})
	}
}

// D5: PGSSLMODE counts as an explicit mode: a weak one is rejected naming
// PGSSLMODE, a verifying one is kept as it is, not upgraded.
func TestPoolConfig_envSSLModeWithCA(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")

	t.Run("prefer", func(t *testing.T) {
		isolatePGEnv(t)
		t.Setenv("PGSSLMODE", "prefer")

		_, err := poolConfig(sentinelKV, caX.PEM)
		assertRejected(t, err, ErrCAWeakSSLMode,
			`"prefer"`, "PGSSLMODE", "verify-full", "verify-ca", "require")
	})

	t.Run("verify-ca", func(t *testing.T) {
		isolatePGEnv(t)
		t.Setenv("PGSSLMODE", "verify-ca")

		cfg, err := poolConfig(sentinelKV, caX.PEM)
		if err != nil {
			t.Fatalf("PGSSLMODE=verify-ca with a CA: %v", err)
		}

		c := cfg.ConnConfig.TLSConfig
		if c == nil || !c.InsecureSkipVerify || c.VerifyPeerCertificate == nil {
			t.Fatalf("want the verify-ca shape (chain callback, no hostname check), got %+v", c)
		}
	})
}

// D6: every TLS config, main and fallbacks, trusts the appended pool; require gets
// a chain check; without a CA the parsed config is pgx's own.
func TestPoolConfig_appliesCAToEveryConfig(t *testing.T) {
	isolatePGEnv(t)
	caX := sinktlstest.NewCA(t, "X")
	caY := sinktlstest.NewCA(t, "Y")
	leafX := caX.Server(t, "localhost").Leaf
	leafY := caY.Server(t, "localhost").Leaf
	twoHosts := "host=sentinel-host,sentinel-host2 user=sentinel-user dbname=sentinel-db sslmode="

	t.Run("verify-full roots", func(t *testing.T) {
		cfg, err := poolConfig(twoHosts+"verify-full", caX.PEM)
		if err != nil {
			t.Fatal(err)
		}

		hosts, configs := tlsConfigs(cfg)
		if len(configs) != 2 {
			t.Fatalf("got %d TLS configs, want 2", len(configs))
		}

		for i, c := range configs {
			if c == nil || c.RootCAs == nil {
				t.Fatalf("config %d (%s) has no RootCAs", i, hosts[i])
			}

			if _, err := leafX.Verify(x509.VerifyOptions{Roots: c.RootCAs, DNSName: "localhost"}); err != nil {
				t.Errorf("config %d (%s): leaf signed by X does not verify: %v", i, hosts[i], err)
			}
		}
	})

	for _, mode := range []string{"require", "verify-ca"} {
		t.Run(mode+" chain check", func(t *testing.T) {
			cfg, err := poolConfig(twoHosts+mode, caX.PEM)
			if err != nil {
				t.Fatal(err)
			}

			hosts, configs := tlsConfigs(cfg)
			for i, c := range configs {
				if c == nil || c.VerifyPeerCertificate == nil {
					t.Fatalf("config %d (%s): no chain check", i, hosts[i])
				}

				if err := c.VerifyPeerCertificate([][]byte{leafX.Raw}, nil); err != nil {
					t.Errorf("config %d: leaf signed by X rejected: %v", i, err)
				}

				if err := c.VerifyPeerCertificate([][]byte{leafY.Raw}, nil); err == nil {
					t.Errorf("config %d: leaf signed by Y accepted", i)
				}
			}
		})
	}

	for _, mode := range []string{"", "disable", "allow", "prefer", "require", "verify-ca", "verify-full"} {
		t.Run("no CA "+mode, func(t *testing.T) {
			dsn := twoHosts + mode
			got, err := poolConfig(dsn, nil)
			if err != nil {
				t.Fatal(err)
			}

			want, err := pgxpool.ParseConfig(dsn)
			if err != nil {
				t.Fatal(err)
			}

			if g, w := tlsShape(got), tlsShape(want); g != w {
				t.Errorf("no-CA config changed:\n got %s\nwant %s", g, w)
			}
		})
	}
}

// tlsShape renders the TLS-relevant fields of a parsed config for comparison.
func tlsShape(cfg *pgxpool.Config) string {
	hosts, configs := tlsConfigs(cfg)

	var b strings.Builder
	for i, c := range configs {
		b.WriteString(hosts[i])
		if c == nil {
			b.WriteString("=plain ")
			continue
		}

		b.WriteString("={")
		if c.InsecureSkipVerify {
			b.WriteString("ISV ")
		}
		if c.VerifyPeerCertificate != nil {
			b.WriteString("VPC ")
		}
		if c.RootCAs != nil {
			b.WriteString("roots ")
		}
		b.WriteString("SN=" + c.ServerName + "} ")
	}

	return b.String()
}

// D7: CA PEM without a certificate fails before any dial, on the config path and
// through both public entry points.
func TestPoolConfig_rejectsGarbageCA(t *testing.T) {
	isolatePGEnv(t)
	garbage := []byte("not a certificate")
	dsn := sentinelKV + " sslmode=verify-full"

	_, err := poolConfig(dsn, garbage)
	assertRejected(t, err, sinktls.ErrNoCACertificate, "spec.tls")

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type: "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory",
		},
	}
	secret := map[string][]byte{"dsn": []byte(dsn)}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = TestConnection(ctx, spec, secret, garbage)
	assertRejected(t, err, sinktls.ErrNoCACertificate, "spec.tls")

	_, err = NewBackend(ctx, spec, secret, garbage)
	assertRejected(t, err, sinktls.ErrNoCACertificate, "spec.tls")
}

// pipeTLSServer returns a DialFunc whose connection is an in-memory pipe to a
// Postgres-like peer: it answers the SSLRequest with 'S' and then runs a TLS
// handshake with cert. With cert nil it closes the connection instead. No socket is
// opened, so netguard's loopback rule does not apply.
func pipeTLSServer(cert *tls.Certificate) pgconn.DialFunc {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		client, server := net.Pipe()

		go func() {
			defer func() { _ = server.Close() }()

			if cert == nil {
				return
			}

			buf := make([]byte, 8)
			if _, err := io.ReadFull(server, buf); err != nil {
				return
			}

			if _, err := server.Write([]byte{'S'}); err != nil {
				return
			}

			// net.Pipe is unbuffered: once the client rejects the certificate it
			// stops reading while the server is still writing its flight, so a short
			// deadline ends the server instead of waiting for the client's timeout.
			_ = server.SetDeadline(time.Now().Add(200 * time.Millisecond))
			_ = tls.Server(server, &tls.Config{
				Certificates: []tls.Certificate{*cert},
				MinVersion:   tls.VersionTLS12,
			}).HandshakeContext(ctx)
		}()

		return client, nil
	}
}

func connectThroughPipe(t *testing.T, roots *x509.CertPool, cert *tls.Certificate) error {
	t.Helper()

	cfg, err := pgconn.ParseConfig(sentinelKV + " sslmode=verify-full")
	if err != nil {
		t.Fatal(err)
	}

	cfg.TLSConfig.RootCAs = roots
	cfg.DialFunc = pipeTLSServer(cert)
	cfg.LookupFunc = func(_ context.Context, host string) ([]string, error) { return []string{host}, nil }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgconn.ConnectConfig(ctx, cfg)
	if err == nil {
		_ = conn.Close(ctx)
		t.Fatal("connected; want a connect error")
	}

	var connErr *pgconn.ConnectError
	if !errors.As(err, &connErr) {
		t.Fatalf("got %T %v, want *pgconn.ConnectError", err, err)
	}

	return err
}

// D8: a certificate verification failure keeps the static connect text and adds a
// static suffix naming spec.tls; no target value appears. Other connect failures
// keep the plain static text.
func TestRedactedConnectError_namesTLSVerification(t *testing.T) {
	t.Parallel()

	caX := sinktlstest.NewCA(t, "X")
	caY := sinktlstest.NewCA(t, "Y")

	roots, err := sinktls.AppendedPool(caX.PEM)
	if err != nil {
		t.Fatal(err)
	}

	certY := caY.Server(t, "sentinel-host")
	verifyErr := redactedConnectError(connectThroughPipe(t, roots, &certY))
	got := withCAHint(verifyErr, true)

	for _, w := range []string{"connect postgres: connection failed", "TLS certificate verification failed", "spec.tls"} {
		if !strings.Contains(got.Error(), w) {
			t.Errorf("redacted error %q does not contain %q", got, w)
		}
	}

	for _, s := range sentinels {
		if strings.Contains(got.Error(), s) {
			t.Errorf("redacted error %q leaks %q", got, s)
		}
	}

	if noCA := withCAHint(verifyErr, false); noCA.Error() != "connect postgres: connection failed" {
		t.Errorf("verification failure without a CA = %q, want the unchanged static text", noCA)
	}

	plain := withCAHint(redactedConnectError(connectThroughPipe(t, roots, nil)), true)
	if plain.Error() != "connect postgres: connection failed" {
		t.Errorf("non-TLS connect failure = %q, want the plain static text", plain)
	}
}

// The parsed-config backstops catch what the pre-parse reader misses: a plaintext
// config, a root pool pgx loaded, and a config without hostname verification when
// the reader expected verify-full.
func TestApplyCA_backstops(t *testing.T) {
	home := isolatePGEnv(t)
	caX := sinktlstest.NewCA(t, "X")
	rootFile := writeFile(t, filepath.Join(home, "root.pem"), caX.PEM)

	roots, err := sinktls.AppendedPool(caX.PEM)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		dsn      string
		wantFull bool
		want     error
	}{
		{"plaintext fallback", sentinelKV + " sslmode=prefer", false, ErrCAWeakSSLMode},
		{"plaintext main", sentinelKV + " sslmode=disable", false, ErrCAWeakSSLMode},
		{"root loaded by pgx", sentinelKV + " sslmode=verify-ca sslrootcert=" + rootFile, false, ErrCARootConflict},
		{"expected verify-full, parsed require", sentinelKV + " sslmode=require", true, ErrCAWeakSSLMode},
		{"verify-full as expected", sentinelKV + " sslmode=verify-full", true, nil},
		{"require as expected", sentinelKV + " sslmode=require", false, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := pgxpool.ParseConfig(tc.dsn)
			if err != nil {
				t.Fatal(err)
			}

			err = applyCA(cfg, roots, tc.wantFull)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("applyCA: %v", err)
				}

				return
			}

			assertRejected(t, err, tc.want, "spec.tls")
		})
	}
}

// The DSN reader agrees with pgx on the spellings that decide the sslmode: empty
// values, the URL ssl=true alias and its order, repeated keys, quoting, escapes, a
// '?' in the password, a bracketed host holding '?', a trailing backslash.
func TestPoolConfig_dsnReaderMatchesPgx(t *testing.T) {
	isolatePGEnv(t)
	caX := sinktlstest.NewCA(t, "X")

	const (
		upgraded = "verify-full"
		kept     = "kept"
		rejected = "rejected"
	)

	cases := []struct {
		name, dsn, want string
	}{
		{"empty keyword sslmode", sentinelKV + " sslmode=", rejected},
		{"empty URL sslmode", sentinelURL + "?sslmode=", rejected},
		{"ssl=true alias", sentinelURL + "?ssl=true", kept},
		{"ssl=true then prefer", sentinelURL + "?ssl=true&sslmode=prefer", rejected},
		{"prefer then ssl=true", sentinelURL + "?sslmode=prefer&ssl=true", kept},
		{"repeated, verify-full last", sentinelKV + " sslmode=prefer sslmode=verify-full", upgraded},
		{"repeated, prefer last", sentinelKV + " sslmode=verify-full sslmode=prefer", rejected},
		{"quoted value", sentinelKV + " sslmode='prefer'", rejected},
		{"escaped value", sentinelKV + ` sslmode=pre\fer`, rejected},
		{"key inside a quoted value", sentinelKV + " application_name='x sslmode=require'", upgraded},
		{"invalid value", sentinelKV + " sslmode=sentinel-db", rejected},
		{"fragment after the mode", sentinelURL + "?sslmode=disable#x", rejected},
		{"question mark in the password", "postgres://sentinel-user:sentinel?pass@sentinel-host/sentinel-db", upgraded},
		{"bracketed host holding a query", "postgres://sentinel-user@[?sslmode=require]/sentinel-db?host=sentinel-host&sslnegotiation=direct", upgraded},
		{"trailing backslash", sentinelKV + ` application_name=x\`, upgraded},
		{"fragment without a query", sentinelURL + "#x", upgraded},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The reader itself decides, not only the backstops after pgx parsed.
			_, wantFull, readErr := caDSN(tc.dsn)
			if (readErr != nil) != (tc.want == rejected) || (readErr == nil && wantFull != (tc.want == upgraded)) {
				t.Errorf("caDSN: verify-full %v, error %v; want %s", wantFull, readErr, tc.want)
			}

			cfg, err := poolConfig(tc.dsn, caX.PEM)

			if tc.want == rejected {
				assertRejected(t, err, ErrCAWeakSSLMode, "spec.tls")
				return
			}

			if err != nil {
				t.Fatalf("poolConfig: %v", err)
			}

			hosts, configs := tlsConfigs(cfg)
			for i, c := range configs {
				if c == nil {
					t.Fatalf("config %d (%s) is plaintext", i, hosts[i])
				}

				if full := !c.InsecureSkipVerify; full != (tc.want == upgraded) {
					t.Errorf("config %d (%s): verify-full = %v, want %s", i, hosts[i], full, tc.want)
				}
			}
		})
	}
}

// Export on a backend with a CA names a certificate verification failure, as the
// probe and NewBackend do; without a CA the error text is unchanged.
func TestExport_namesTLSVerificationOnlyWithCA(t *testing.T) {
	isolatePGEnv(t)
	caX := sinktlstest.NewCA(t, "X")
	certY := sinktlstest.NewCA(t, "Y").Server(t, "sentinel-host")

	payload := []byte(`[{"targetNamespace":"apps","targetName":"web","namespace":"apps","name":"demo","version":"v1","kind":"Deployment","uid":"u"}]`)

	for _, withCA := range []bool{true, false} {
		cfg, err := poolConfig(sentinelKV+" sslmode=verify-full", caX.PEM)
		if err != nil {
			t.Fatal(err)
		}

		cfg.ConnConfig.DialFunc = pipeTLSServer(&certY)
		cfg.ConnConfig.LookupFunc = func(_ context.Context, host string) ([]string, error) { return []string{host}, nil }

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			cancel()
			t.Fatal(err)
		}

		b := &Backend{cfg: Config{Schema: "public", Table: "inventory"}, pool: pool, caConfigured: withCA}
		err = b.Export(ctx, payload, "inventory/apps/demo.json")
		pool.Close()
		cancel()

		if !errors.Is(err, ErrBeginTxFailed) {
			t.Fatalf("withCA=%v: Export error %v, want ErrBeginTxFailed", withCA, err)
		}

		if got := strings.Contains(err.Error(), "TLS certificate verification failed"); got != withCA {
			t.Errorf("withCA=%v: Export error %q names TLS verification = %v", withCA, err, got)
		}

		for _, s := range sentinels {
			if strings.Contains(err.Error(), s) {
				t.Errorf("Export error %q leaks %q", err, s)
			}
		}
	}
}
