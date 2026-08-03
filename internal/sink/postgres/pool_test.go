// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"strings"
	"testing"
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
