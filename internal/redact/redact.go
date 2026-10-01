// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package redact is the shared error-redaction choke-point (K-23): every path
// that turns free-form error or driver text into a persisted Kubernetes status
// condition message or Event message routes through Text/Error here first.
//
// Text masks these credential carriers, and only these:
//
//   - URL userinfo in any scheme (nats://, mongodb://, https://, ssh://...),
//     where the userinfo has no whitespace, quote or '/'; inside a double- or
//     single-quoted URL (how url.ParseError and most drivers quote one) the
//     userinfo may also contain spaces and, Go-escaped, quotes;
//   - the values of credential query parameters (access_token, token,
//     password, secret, api_key, sig, GitLab private_token and close variants);
//   - the credential after "Authorization: Bearer|Basic|Token|Digest";
//   - key=value DSN password fields (password=, passwd=, pwd=), bare or quoted;
//   - any caller-supplied secret value, verbatim.
//
// Not covered: unquoted userinfo containing whitespace or quotes, userinfo
// containing '/', and credentials in any other free-form shape. Parse sites
// must therefore still return static messages rather than echo raw input.
//
// The contract is deliberately wider than git's redactCredentials (which keeps
// a narrower http(s)-only regex so ssh://git@host routing hints survive CLI
// output): status/Event text is readable by anyone with get on the object.
// All patterns are RE2 (linear time), and Text is idempotent.
package redact

import (
	"regexp"
	"strings"
)

// Placeholder replaces every redacted fragment.
const Placeholder = "***"

// urlUserinfoRE matches "scheme://userinfo@" for any scheme. The userinfo run
// excludes '/' and whitespace (so the host after '@' survives and no match
// crosses lines or words) and quotes (so wrapped URLs stop at their quotes);
// everything else — parens, brackets, colons in passwords — must stay in the
// run or a password using those characters escapes redaction (same character
// class as the proven git redactor).
var urlUserinfoRE = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.\-]*://)[^/\s'"]+@`)

// dqURLUserinfoRE and sqURLUserinfoRE cover a URL that starts right after an
// opening quote. The quotes bound the match, so the userinfo run may contain
// spaces (and, in the Go-quoted form url.ParseError uses, escaped characters
// such as \"). '/' stays excluded so a quoted URL's path is never consumed.
var (
	dqURLUserinfoRE = regexp.MustCompile(`(?i)"([a-z][a-z0-9+.\-]*://)(?:[^"\\@/\n]|\\.)*@`)
	sqURLUserinfoRE = regexp.MustCompile(`(?i)'([a-z][a-z0-9+.\-]*://)[^'@/\n]*@`)
)

// queryCredentialRE masks the value of a credential-bearing query (or
// ;-separated) parameter. The value stops at the next separator, fragment,
// whitespace or quote.
var queryCredentialRE = regexp.MustCompile(
	`(?i)([?&;](?:access_token|refresh_token|id_token|private_token|token|password|passwd|pwd|` +
		`secret|client_secret|api_key|apikey|sig|signature)=)[^&;#\s'"]+`)

// authHeaderRE masks the credential of an Authorization header value while
// keeping the auth scheme, which is useful diagnostic context.
var authHeaderRE = regexp.MustCompile(
	`(?i)\b(authorization\s*[:=]\s*(?:bearer|basic|token|digest)\s+)[^\s'",;]+`)

// dsnPasswordRE masks key=value DSN password fields (libpq, ADO/ODBC style).
// A quoted value is masked whole, quotes included.
var dsnPasswordRE = regexp.MustCompile(
	`(?i)\b((?:password|passwd|pwd)\s*=\s*)(?:'[^']*'|"[^"]*"|[^\s;&'"]+)`)

// Text masks the credential carriers listed in the package doc and replaces
// every non-empty secret value verbatim. Text without credentials is returned
// byte-identical. Multiple occurrences across multiple lines are masked.
func Text(msg string, secrets ...string) string {
	msg = dqURLUserinfoRE.ReplaceAllString(msg, `"${1}`+Placeholder+"@")
	msg = sqURLUserinfoRE.ReplaceAllString(msg, `'${1}`+Placeholder+"@")
	msg = urlUserinfoRE.ReplaceAllString(msg, "${1}"+Placeholder+"@")
	msg = queryCredentialRE.ReplaceAllString(msg, "${1}"+Placeholder)
	msg = authHeaderRE.ReplaceAllString(msg, "${1}"+Placeholder)
	msg = dsnPasswordRE.ReplaceAllString(msg, "${1}"+Placeholder)

	for _, secret := range secrets {
		if secret == "" {
			continue
		}

		msg = strings.ReplaceAll(msg, secret, Placeholder)
	}

	return msg
}

// Error returns an error whose message is Text(err.Error(), secrets...). The
// original error remains reachable through Unwrap, so errors.Is/errors.As,
// the internal/errors taxonomy (ClassOf/IsTerminal) and apierrors.Is* keep
// working on the returned value; only the rendered string is redacted.
// A nil error maps to nil.
func Error(err error, secrets ...string) error {
	if err == nil {
		return nil
	}

	return &redactedError{msg: Text(err.Error(), secrets...), err: err}
}

type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string {
	return e.msg
}

func (e *redactedError) Unwrap() error {
	return e.err
}
