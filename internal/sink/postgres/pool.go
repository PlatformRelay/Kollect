// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/platformrelay/kollect/internal/sink/netguard"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
)

func newGuardedPool(ctx context.Context, dsn string, caPEM []byte) (*pgxpool.Pool, error) {
	poolCfg, err := poolConfig(dsn, caPEM)
	if err != nil {
		return nil, err
	}
	poolCfg.ConnConfig.DialFunc = netguard.DefaultDialer.DialContext

	return pgxpool.NewWithConfig(ctx, poolCfg)
}

// poolConfig parses dsn into a pool config. With caPEM it applies the spec.tls trust
// rules (SCT-2): the DSN is checked and, with no sslmode, set to verify-full before
// parsing, and every parsed TLS config trusts the system roots plus the CA. The CA is
// set on the parsed configs before the pool copies them, because pgx's verify-ca
// callback reads RootCAs from the config it was built with. Without caPEM the config
// is pgx's own.
func poolConfig(dsn string, caPEM []byte) (*pgxpool.Config, error) {
	if len(caPEM) == 0 {
		poolCfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			return nil, redactedParseError(err)
		}

		return poolCfg, nil
	}

	roots, err := sinktls.AppendedPool(caPEM)
	if err != nil {
		return nil, err
	}

	dsn, wantVerifyFull, err := caDSN(dsn)
	if err != nil {
		return nil, err
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, redactedParseError(err)
	}

	if err := applyCA(poolCfg, roots, wantVerifyFull); err != nil {
		return nil, err
	}

	return poolCfg, nil
}

// redactedParseError produces a DSN-free error for a connection-string parse
// failure so tenant credentials and connection targets can never reach an error
// string or log (SEC-01).
//
// pgx returns *pgconn.ParseConfigError for every connection-string fault. Both
// its Error() text and its ConnString field carry the DSN, and on the URL
// host:port-split path it echoes the raw, unredacted host *after* the
// backtick-quoted DSN (e.g. an IPv6 zone-id with a port, or a typo'd extra
// colon). We therefore return a STATIC message and never fold any of its
// free-form text back in — the typed error's ConnString/Unwrap remain available
// programmatically but never hit the returned string.
//
// The earlier backtick-strip fallback (SEC-01-FUP1) was a latent fail-open: it
// echoed the reason after the DSN for any non-*pgconn.ParseConfigError. That
// branch is unreachable in pinned pgx v5.10.0 — pgxpool.ParseConfig routes
// every fault through *pgconn.ParseConfigError (pgx.ParseConfig for
// connection-string faults, pgconn.NewParseConfigError for pool-param faults) —
// but a future pgx bump could reintroduce a non-PCE leaking path, so we now fail
// closed with the static message unconditionally rather than reflect any pgx
// text.
func redactedParseError(_ error) error {
	// Fail closed for EVERY parse fault: never fold any free-form pgx text (which
	// may echo the DSN or host) back into the returned string. Post-FUP1 the
	// *pgconn.ParseConfigError type check discriminated nothing — both it and the
	// fallthrough returned this identical static message — so it is collapsed to a
	// single unconditional return (SEC-01-FUP4). The typed error remains available
	// programmatically to callers via the original err; only the returned string
	// is redacted.
	return errors.New("parse postgres DSN: invalid connection string")
}

// redactedConnectError produces a host/credential-free error for a POST-parse
// dial failure (SEC-01-FUP2). Unlike a parse fault, a dial fault surfaces
// lazily — pgxpool.NewWithConfig connects on first Acquire — so it reaches the
// caller via pool.Ping / pool.Exec, not via newGuardedPool's return.
//
// pgx wraps such faults in *pgconn.ConnectError, whose Error() text embeds the
// tenant user and database, and — through the wrapped perDialConnectError and
// the netguard resolver's own message — the target host. We return a STATIC
// message for that type so none of that free-form text can reach a log or error
// string, while still returning a non-nil error so callers see the connection
// failed. Non-connect errors (context deadlines, server-side PgError, etc.) do
// not carry the host and pass through unchanged.
//
// The returned error also records, without any text, whether the cause was a
// certificate verification failure, so withCAHint can name it for a sink with a
// spec.tls CA.
func redactedConnectError(err error) error {
	var connErr *pgconn.ConnectError
	if !errors.As(err, &connErr) {
		return err
	}

	return &connectFailedError{tlsVerification: isCertificateVerificationError(err)}
}

// connectFailedError is the static, target-free connect failure.
type connectFailedError struct {
	tlsVerification bool
}

func (e *connectFailedError) Error() string {
	return "connect postgres: connection failed"
}

// withCAHint adds a static suffix to a connect failure caused by certificate
// verification when the sink configures a spec.tls CA, so a wrong CA is told apart
// from a network fault without naming the target (SCT-2). Without a CA, or for any
// other error, err is returned unchanged.
func withCAHint(err error, caConfigured bool) error {
	var connectErr *connectFailedError
	if !caConfigured || !errors.As(err, &connectErr) || !connectErr.tlsVerification {
		return err
	}

	return fmt.Errorf("%w: %s", err, tlsVerificationFailed)
}

// tlsVerificationFailed is the static suffix for a certificate verification failure.
const tlsVerificationFailed = "TLS certificate verification failed against the system roots and the spec.tls CA"

func isCertificateVerificationError(err error) bool {
	var (
		verifyErr    *tls.CertificateVerificationError
		authorityErr x509.UnknownAuthorityError
		hostnameErr  x509.HostnameError
		invalidErr   x509.CertificateInvalidError
	)

	return errors.As(err, &verifyErr) || errors.As(err, &authorityErr) ||
		errors.As(err, &hostnameErr) || errors.As(err, &invalidErr)
}
