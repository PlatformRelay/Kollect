// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package postgres

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgservicefile"
	"github.com/jackc/pgx/v5/pgxpool"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// acceptedSSLModes is the fix every weak-sslmode error names.
const acceptedSSLModes = "use sslmode=verify-full, verify-ca or require (verify-ca when the host does not match the certificate SAN)"

// pgSetting is one TLS-relevant setting with the place it came from, for error text.
type pgSetting struct {
	value  string
	source string
}

// validSSLModes are the sslmode values pgx accepts.
var validSSLModes = map[string]bool{
	"disable": true, "allow": true, "prefer": true, "require": true, "verify-ca": true, "verify-full": true,
}

// caDSN checks dsn against a configured CA before pgx parses it (sink-ca-trust
// design, Postgres steps 1 and 2): a root certificate from any other source is a
// conflict, an explicit disable/allow/prefer is rejected, and an sslmode set nowhere
// becomes verify-full. It returns the DSN to parse, with the effective mode pinned
// in it, and whether that mode is verify-full, which applyCA checks against what pgx
// parsed. Errors never contain the DSN.
func caDSN(dsn string) (string, bool, error) {
	settings, err := connStringSettings(dsn)
	if err != nil {
		return "", false, redactedParseError(err)
	}

	env := func(name string) (pgSetting, bool) {
		v := os.Getenv(name)
		return pgSetting{value: v, source: name}, v != ""
	}

	// As in pgx, a DSN key wins by being present, even with an empty value; an
	// empty variable is unset.
	service, present := settings["service"]
	if !present {
		service, present = env("PGSERVICE")
	}

	var svc map[string]string
	if present {
		svc = serviceSettings(settings, service.value)
	}

	if src, ok := rootCertSource(settings, svc); ok {
		return "", false, kollecterrors.Terminal(fmt.Errorf(
			"%w: %s; remove it or remove spec.tls", ErrCARootConflict, src))
	}

	// A key present with an empty value still counts as set: pgx then reads it as
	// prefer, so it is rejected below rather than upgraded. An empty variable is unset.
	mode, ok := settings["sslmode"]
	if !ok {
		if v, found := svc["sslmode"]; found {
			mode, ok = pgSetting{value: v, source: "the service file entry"}, true
		} else {
			mode, ok = env("PGSSLMODE")
		}
	}

	if !ok {
		return withSSLMode(dsn, "verify-full"), true, nil
	}

	switch {
	case mode.value == "", mode.value == "disable", mode.value == "allow", mode.value == "prefer":
		return "", false, kollecterrors.Terminal(fmt.Errorf(
			"%w: sslmode %q from %s; %s", ErrCAWeakSSLMode, mode.value, mode.source, acceptedSSLModes))
	case !validSSLModes[mode.value]:
		// pgx rejects it too; failing here also covers a DSN this reader splits
		// differently from pgx. The value is not echoed: a misread one can hold DSN text.
		return "", false, kollecterrors.Terminal(fmt.Errorf(
			"%w: the sslmode from %s is not a valid mode; %s", ErrCAWeakSSLMode, mode.source, acceptedSSLModes))
	}

	// The mode is pinned in the DSN even when it came from the DSN: pgx parses
	// require and prefer+sslnegotiation=direct to the same shape, so applyCA cannot
	// tell them apart. Pinned, pgx uses this mode, or a later DSN mode the reader
	// missed. Such a missed mode is never weaker than this one: disable, allow and
	// prefer without direct leave a plaintext config, which applyCA rejects; prefer
	// with direct gets require's chain check, as require and verify-ca have; under
	// verify-full, applyCA rejects any config that skips the hostname.
	return withSSLMode(dsn, mode.value), mode.value == "verify-full", nil
}

// rootCertSource names the first root certificate source pgx would read besides
// spec.tls, in pgx's precedence order, whether or not the file is readable.
func rootCertSource(settings map[string]pgSetting, svc map[string]string) (string, bool) {
	// An empty DSN value makes pgx ignore the other sources; they are still rejected
	// here, which is stricter than pgx and keeps the conflict rule simple.
	if s, ok := settings["sslrootcert"]; ok && s.value != "" {
		return "sslrootcert in " + s.source, true
	}

	if v := svc["sslrootcert"]; v != "" {
		return "sslrootcert in the service file entry", true
	}

	if os.Getenv("PGSSLROOTCERT") != "" {
		return "PGSSLROOTCERT", true
	}

	if home, err := os.UserHomeDir(); err == nil {
		if _, err := os.Stat(filepath.Join(home, ".postgresql", "root.crt")); err == nil {
			return "~/.postgresql/root.crt", true
		}
	}

	return "", false
}

// serviceSettings returns the entry that pgx would merge for service, from the
// servicefile setting, PGSERVICEFILE or ~/.pg_service.conf. An unreadable file or a
// missing entry returns nil: pgx then fails the parse itself.
func serviceSettings(settings map[string]pgSetting, service string) map[string]string {
	fileSetting, present := settings["servicefile"]
	path := fileSetting.value
	if !present {
		path = os.Getenv("PGSERVICEFILE")
	}

	if !present && path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}

		path = filepath.Join(home, ".pg_service.conf")
	}

	file, err := pgservicefile.ReadServicefile(path)
	if err != nil {
		return nil
	}

	entry, err := file.GetService(service)
	if err != nil {
		return nil
	}

	return entry.Settings
}

// withSSLMode adds sslmode=mode in front of the DSN's own keys, so a later sslmode in
// the DSN still wins. The keyword form gets the pair in front, so a trailing
// backslash in the DSN cannot swallow it; the URL form gets it as the first query
// parameter. mode is one of validSSLModes, never DSN text.
func withSSLMode(dsn, mode string) string {
	if !isURLDSN(dsn) {
		return "sslmode=" + mode + " " + dsn
	}

	if i := queryIndex(dsn); i >= 0 {
		return dsn[:i+1] + "sslmode=" + mode + "&" + dsn[i+1:]
	}

	return dsn + "?sslmode=" + mode
}

func isURLDSN(dsn string) bool {
	return strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
}

// queryIndex returns the index of the '?' that starts a URL DSN's query, or -1. It
// follows pgx's parse_url.go: the user-info part ends at the first '@' before any
// '/', so a '?' in a password does not count; a host that starts with '[' runs to
// the next ']' whatever it holds; the database name runs to the first '?'.
func queryIndex(dsn string) int {
	rest := strings.TrimPrefix(strings.TrimPrefix(dsn, "postgresql://"), "postgres://")
	offset := len(dsn) - len(rest)

	if i := strings.IndexAny(rest, "@/"); i >= 0 && rest[i] == '@' {
		offset += i + 1
		rest = rest[i+1:]
	}

	for i := 0; i < len(rest); i++ {
		switch {
		case rest[i] == '[' && (i == 0 || rest[i-1] == ','):
			end := strings.IndexByte(rest[i:], ']')
			if end < 0 {
				return -1
			}

			i += end
		case rest[i] == '?':
			return offset + i
		case rest[i] == '/':
			if j := strings.IndexByte(rest[i:], '?'); j >= 0 {
				return offset + i + j
			}

			return -1
		}
	}

	return -1
}

// connStringSettings reads the keys of a keyword or URL DSN the way pgx does
// (pgconn/config.go parseKeywordValueSettings, parse_url.go parseURLQueryParams):
// last occurrence wins, and the URL alias ssl=true means sslmode=require.
func connStringSettings(dsn string) (map[string]pgSetting, error) {
	if !isURLDSN(dsn) {
		return keywordSettings(dsn)
	}

	settings := make(map[string]pgSetting)

	i := queryIndex(dsn)
	if i < 0 {
		return settings, nil
	}

	sslWasLast := false
	for _, pair := range strings.Split(dsn[i+1:], "&") {
		if pair == "" {
			continue
		}

		rawKey, rawValue, found := strings.Cut(pair, "=")
		if !found {
			return nil, errors.New("query parameter without '='")
		}

		key, err := url.PathUnescape(strings.Trim(rawKey, " "))
		if err != nil {
			return nil, err
		}

		value, err := url.PathUnescape(strings.Trim(rawValue, " "))
		if err != nil {
			return nil, err
		}

		switch key {
		case "ssl":
			sslWasLast = true
		case "sslmode":
			sslWasLast = false
		}

		settings[key] = pgSetting{value: value, source: "the DSN"}
	}

	if s, ok := settings["ssl"]; ok && s.value == "true" {
		delete(settings, "ssl")
		if sslWasLast {
			settings["sslmode"] = pgSetting{value: "require", source: "the DSN (ssl=true)"}
		}
	}

	return settings, nil
}

// keywordSettings mirrors pgx's keyword/value reader: whitespace-separated
// key=value pairs, values optionally single-quoted, backslash escapes the next byte.
func keywordSettings(s string) (map[string]pgSetting, error) {
	const space = " \t\n\r\v\f"

	settings := make(map[string]pgSetting)
	s = strings.TrimLeft(s, space)

	for s != "" {
		eq := strings.IndexByte(s, '=')
		if eq < 0 {
			return nil, errors.New("invalid keyword/value")
		}

		key := strings.Trim(s[:eq], space)
		s = strings.TrimLeft(s[eq+1:], space)

		var raw string
		if strings.HasPrefix(s, "'") {
			end := valueEnd(s[1:], func(c byte) bool { return c == '\'' })
			if end >= len(s)-1 {
				return nil, errors.New("unterminated quoted string")
			}

			raw, s = s[1:end+1], s[end+2:]
		} else {
			end := valueEnd(s, func(c byte) bool { return strings.IndexByte(space, c) >= 0 })
			if end > len(s) {
				end = len(s)
			}

			raw, s = s[:end], s[end:]
		}

		s = strings.TrimLeft(s, space)
		settings[key] = pgSetting{value: unescape(raw), source: "the DSN"}
	}

	return settings, nil
}

// valueEnd returns the index of the first byte in s that stop accepts, skipping each
// byte after a backslash; len(s) or more when there is none.
func valueEnd(s string, stop func(byte) bool) int {
	i := 0
	for ; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}

		if stop(s[i]) {
			return i
		}
	}

	return i
}

func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			if i == len(s) {
				break
			}
		}

		b.WriteByte(s[i])
	}

	return b.String()
}

// applyCA sets roots on the main TLS config and every fallback after pgx parsed the
// DSN (design step 3), and rejects what the pre-parse check should have caught: a
// plaintext config, a root pool pgx loaded from a source it missed, or, when caDSN
// expected verify-full, a config that skips hostname verification (pgx read the DSN
// differently). require without a root certificate gets the chain check pgx uses for
// verify-ca. RootCAs is set on the parsed configs, which pgx's own verify-ca callback
// reads at handshake.
func applyCA(cfg *pgxpool.Config, roots *x509.CertPool, wantVerifyFull bool) error {
	configs := make([]*tls.Config, 0, 1+len(cfg.ConnConfig.Fallbacks))
	configs = append(configs, cfg.ConnConfig.TLSConfig)
	for _, fb := range cfg.ConnConfig.Fallbacks {
		configs = append(configs, fb.TLSConfig)
	}

	for _, c := range configs {
		if c == nil {
			return kollecterrors.Terminal(fmt.Errorf(
				"%w: a connection would not use TLS; %s", ErrCAWeakSSLMode, acceptedSSLModes))
		}

		if c.RootCAs != nil {
			return kollecterrors.Terminal(fmt.Errorf(
				"%w: pgx loaded a root certificate; remove it or remove spec.tls", ErrCARootConflict))
		}

		if wantVerifyFull && c.InsecureSkipVerify {
			return kollecterrors.Terminal(fmt.Errorf(
				"%w: the parsed connection does not verify the hostname, although the DSN reads as verify-full; %s",
				ErrCAWeakSSLMode, acceptedSSLModes))
		}
	}

	for _, c := range configs {
		c.RootCAs = roots
		if c.InsecureSkipVerify && c.VerifyPeerCertificate == nil {
			// pgx sets no ClientSessionCache, so a client session never resumes and
			// the callback runs on every handshake, as pgx's own verify-ca does.
			c.VerifyPeerCertificate = chainCheck(roots) //nolint:gosec // G123: no session resumption, see above
		}
	}

	return nil
}

// chainCheck verifies the peer chain against roots without a hostname, as libpq
// does for require with a root certificate and for verify-ca.
func chainCheck(roots *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	return func(raw [][]byte, _ [][]*x509.Certificate) error {
		if len(raw) == 0 {
			return errors.New("server presented no certificate")
		}

		certs := make([]*x509.Certificate, len(raw))
		for i, der := range raw {
			cert, err := x509.ParseCertificate(der)
			if err != nil {
				return fmt.Errorf("parse server certificate: %w", err)
			}

			certs[i] = cert
		}

		opts := x509.VerifyOptions{Roots: roots, Intermediates: x509.NewCertPool()}
		for _, cert := range certs[1:] {
			opts.Intermediates.AddCert(cert)
		}

		_, err := certs[0].Verify(opts)

		return err
	}
}
