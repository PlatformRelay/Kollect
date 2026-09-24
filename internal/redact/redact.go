// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package redact is the shared error-redaction choke-point (K-23): every path
// that turns free-form error or driver text into a persisted Kubernetes status
// condition message or Event message routes through Text/Error here first.
//
// The contract is deliberately wider than git's redactCredentials (which keeps
// a narrower http(s)-only regex so ssh://git@host routing hints survive CLI
// output): status/Event text is readable by anyone with get on the object, so
// userinfo in ANY scheme (nats://, mongodb://, http://...) is masked, plus any
// caller-supplied secret values verbatim.
package redact

import (
	"regexp"
	"strings"
)

// Placeholder replaces every redacted fragment.
const Placeholder = "***"

// urlUserinfoRE matches "scheme://userinfo@" for any scheme. The userinfo run
// excludes '/' and whitespace so the host after '@' survives, and excludes
// quote/bracket characters so a stray '@' in surrounding prose cannot drag
// unrelated text into the match.
var urlUserinfoRE = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.\-]*://)[^/\s@'"()\[\]<>]+@`)

// Text masks credential-bearing URL userinfo in msg and replaces every
// non-empty secret value verbatim. Text without credentials is returned
// byte-identical. Multiple occurrences across multiple lines are masked.
func Text(msg string, secrets ...string) string {
	msg = urlUserinfoRE.ReplaceAllString(msg, "${1}"+Placeholder+"@")

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
