// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// TestNewGuardedPoolRedactsDSNOnParseError verifies that a DSN parse failure
// never echoes the connection string (SEC-01). pgx returns a
// *pgconn.ParseConfigError whose Error() text — and whose ConnString field —
// carry the DSN. On the URL host-split path pgx even prints the raw,
// unredacted host (an operator misconfiguration such as an IPv6 zone-id with a
// port, or a typo'd extra colon), so stripping the backtick-quoted echo is not
// enough. The returned error must contain none of the DSN tokens: password,
// user, host, or dbname.
func TestNewGuardedPoolRedactsDSNOnParseError(t *testing.T) {
	const (
		user = "tenantuser"
		pass = "hunter2SECRET" // fixture value, not a real credential
		db   = "tenantdb"
	)
	// DSNs are assembled from parts so the test source carries no inline
	// user:password@ URL literal (which would trip gosec G101).
	buildDSN := func(host, suffix string) string {
		return "postgres://" + user + ":" + pass + "@" + host + "/" + db + suffix
	}

	tests := []struct {
		name    string
		dsn     string
		secrets []string // substrings that must NOT appear in the error
	}{
		{
			name:    "invalid sslmode",
			dsn:     buildDSN("secret-db.internal:5432", "?sslmode=not-a-real-mode"),
			secrets: []string{pass, user, "secret-db.internal", db},
		},
		{
			// IPv6 zone-id + port: pgx's host:port split fails and echoes the
			// raw host AFTER the backtick-quoted DSN, past any backtick strip.
			name:    "ipv6 zone-id host leaks past backtick",
			dsn:     buildDSN("fe80::1%25eth0:5432", ""),
			secrets: []string{pass, user, "fe80::1", db},
		},
		{
			// Typo'd extra colon in host:port — same host-split leak path.
			name:    "extra colon host leaks past backtick",
			dsn:     buildDSN("secret-db.internal:5432:99", ""),
			secrets: []string{pass, user, "secret-db.internal", db},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newGuardedPool(context.Background(), tc.dsn)
			if err == nil {
				t.Fatalf("expected ParseConfig to fail for %q, got nil error", tc.dsn)
			}

			msg := err.Error()
			for _, secret := range tc.secrets {
				if strings.Contains(msg, secret) {
					t.Errorf("error string leaks DSN token %q: %s", secret, msg)
				}
			}
			// Also never echo the DSN verbatim.
			if strings.Contains(msg, tc.dsn) {
				t.Errorf("error string leaks the raw DSN: %s", msg)
			}
			// The error must still name its context.
			if !strings.Contains(msg, "parse postgres DSN") {
				t.Errorf("error dropped its context prefix, got: %s", msg)
			}
		})
	}
}

// assertNoDSNTokens fails if msg contains any of the sensitive substrings.
func assertNoDSNTokens(t *testing.T, msg string, tokens ...string) {
	t.Helper()
	for _, tok := range tokens {
		if strings.Contains(msg, tok) {
			t.Errorf("error string leaks token %q: %s", tok, msg)
		}
	}
}

// TestRedactedParseErrorFailsClosedForNonParseError covers SEC-01-FUP1: the
// backtick-strip fallback was a latent fail-open. It is unreachable in pinned
// pgx v5.10.0 (every pgxpool.ParseConfig fault is a *pgconn.ParseConfigError),
// but a synthesized non-ParseConfigError whose text echoes a backtick-quoted
// DSN followed by a raw host is exactly what a future pgx bump could reintroduce
// on a non-PCE path. The redactor must fail closed with the static message and
// never fold that free-form text back in.
func TestRedactedParseErrorFailsClosedForNonParseError(t *testing.T) {
	t.Parallel()

	const (
		user = "tenantuser"
		pass = "hunter2SECRET" // fixture value, not a real credential
		db   = "tenantdb"
	)
	// A plain errors.New is NOT a *pgconn.ParseConfigError, so errors.As fails
	// and (before the fix) execution falls through to the backtick strip, which
	// echoes the reason — including the host — after the quoted DSN.
	leaky := errors.New(
		"cannot parse `postgres://" + user + ":" + pass + "@secret-db.internal:5432/" + db +
			"`: host leak secret-db.internal",
	)

	got := redactedParseError(leaky)
	msg := got.Error()

	assertNoDSNTokens(t, msg, pass, user, "secret-db.internal", db, "host leak")
	if msg != "parse postgres DSN: invalid connection string" {
		t.Errorf("want static redacted message, got: %s", msg)
	}
}

// TestRedactedConnectErrorRedactsDialError covers SEC-01-FUP2: a post-parse
// dial failure surfaces as *pgconn.ConnectError whose text embeds the tenant
// user, database, and target host (via the wrapped perDialConnectError and the
// netguard resolver echo). redactedConnectError must strip all of it while
// leaving a non-nil failure so callers still see the connection failed.
func TestRedactedConnectErrorRedactsDialError(t *testing.T) {
	t.Parallel()

	const (
		user = "tenantuser"
		pass = "hunter2SECRET" // fixture value, not a real credential
		db   = "tenantdb"
	)
	// 127.0.0.1:1 is rejected by netguard as a non-globally-routable address
	// before any egress, so this is hermetic and instant.
	dsn := "postgres://" + user + ":" + pass + "@127.0.0.1:1/" + db

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := newGuardedPool(ctx, dsn)
	if err != nil {
		t.Fatalf("newGuardedPool returned an unexpected error: %v", err)
	}
	defer pool.Close()

	raw := pool.Ping(ctx)
	if raw == nil {
		t.Fatal("expected a dial failure pinging an unreachable host")
	}
	// Guard the fixture: the raw error must actually leak the host, otherwise
	// this test would pass vacuously.
	if !strings.Contains(raw.Error(), "127.0.0.1") {
		t.Fatalf("fixture no longer leaks the host, revisit the test: %v", raw)
	}

	got := redactedConnectError(raw)
	if got == nil {
		t.Fatal("redactedConnectError must remain non-nil so callers still see the failure")
	}
	assertNoDSNTokens(t, got.Error(), pass, user, "127.0.0.1", db)
}

// TestRedactedConnectErrorPassesThroughNonConnectError verifies non-connect
// errors are returned unchanged (only the ConnectError leak class is redacted).
func TestRedactedConnectErrorPassesThroughNonConnectError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("some non-connect failure")
	if got := redactedConnectError(sentinel); !errors.Is(got, sentinel) {
		t.Errorf("want pass-through of non-connect error, got: %v", got)
	}
}

// TestTestConnectionRedactsDialError exercises the connection.go Ping call site
// end-to-end: TestConnection must not leak the DSN host/credentials when the
// dial fails.
func TestTestConnectionRedactsDialError(t *testing.T) {
	t.Parallel()

	const (
		user = "tenantuser"
		pass = "hunter2SECRET" // fixture value, not a real credential
		db   = "tenantdb"
	)
	dsn := "postgres://" + user + ":" + pass + "@127.0.0.1:1/" + db

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := TestConnection(ctx, kollectdevv1alpha1.KollectSinkSpec{
		Type: "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory",
		},
	}, map[string][]byte{"dsn": []byte(dsn)})
	if err == nil {
		t.Fatal("expected a dial failure for an unreachable host")
	}
	assertNoDSNTokens(t, err.Error(), pass, user, "127.0.0.1", db)
}

// TestNewBackendRedactsDialError exercises the backend.go ensureTable call site
// end-to-end: NewBackend must not leak the DSN host/credentials when the dial
// backing ensureTable fails.
func TestNewBackendRedactsDialError(t *testing.T) {
	t.Parallel()

	const (
		user = "tenantuser"
		pass = "hunter2SECRET" // fixture value, not a real credential
		db   = "tenantdb"
	)
	dsn := "postgres://" + user + ":" + pass + "@127.0.0.1:1/" + db

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := NewBackend(ctx, kollectdevv1alpha1.KollectSinkSpec{
		Type: "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory",
		},
	}, map[string][]byte{"dsn": []byte(dsn)})
	if err == nil {
		t.Fatal("expected a dial failure constructing the backend")
	}
	assertNoDSNTokens(t, err.Error(), pass, user, "127.0.0.1", db)
}
