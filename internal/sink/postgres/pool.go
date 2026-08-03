// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/platformrelay/kollect/internal/sink/netguard"
)

func newGuardedPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, redactedParseError(err)
	}
	poolCfg.ConnConfig.DialFunc = netguard.DefaultDialer.DialContext

	return pgxpool.NewWithConfig(ctx, poolCfg)
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
func redactedParseError(err error) error {
	var parseErr *pgconn.ParseConfigError
	if errors.As(err, &parseErr) {
		return errors.New("parse postgres DSN: invalid connection string")
	}

	// Unreachable today; fail closed rather than fold any free-form pgx text
	// (which may echo the DSN or host) back into the returned string.
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
func redactedConnectError(err error) error {
	var connErr *pgconn.ConnectError
	if errors.As(err, &connErr) {
		return errors.New("connect postgres: connection failed")
	}

	return err
}
