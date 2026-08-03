// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
// colon). We therefore return a STATIC message for that type and never fold any
// of its free-form text back in — the typed error's ConnString/Unwrap remain
// available programmatically but never hit the returned string.
//
// The backtick-strip fallback covers any hypothetical non-ParseConfigError
// (pgxpool wraps all of its own pool-param faults in ParseConfigError, so this
// is currently unreachable defense-in-depth): keep the reason after the DSN
// echo, and fall back to the static message rather than risk failing open.
func redactedParseError(err error) error {
	var parseErr *pgconn.ParseConfigError
	if errors.As(err, &parseErr) {
		return errors.New("parse postgres DSN: invalid connection string")
	}

	msg := err.Error()
	// A well-formed DSN (URL or keyword form) never contains a backtick, so
	// LastIndex reliably lands on the DSN echo's closing delimiter.
	if i := strings.LastIndex(msg, "`"); i >= 0 {
		if reason := strings.TrimPrefix(msg[i+1:], ": "); reason != "" {
			return fmt.Errorf("parse postgres DSN: %s", reason)
		}
	}
	return errors.New("parse postgres DSN: invalid connection string")
}
