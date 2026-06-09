// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"regexp"
	"strings"
)

const redactedPlaceholder = "***"

// httpURLUserinfoRE matches userinfo embedded in http(s) URLs, e.g.
// https://user:token@host or https://token@host. Other schemes (ssh://git@host)
// are left untouched: their userinfo is a plain username, never a credential.
var httpURLUserinfoRE = regexp.MustCompile(`(?i)\b(https?://)[^/\s'"]+@`)

// redactCredentials masks credentials in git CLI output before it is wrapped
// into an error (EC-P1-02): error text propagates via ClassifyExportError into
// CR status conditions and Kubernetes Events, which are persisted in etcd.
// It redacts userinfo in http(s) URLs across multiple occurrences and lines,
// and replaces any known secret values verbatim (empty strings are skipped).
func redactCredentials(msg string, secrets ...string) string {
	msg = httpURLUserinfoRE.ReplaceAllString(msg, "${1}"+redactedPlaceholder+"@")

	for _, secret := range secrets {
		if secret == "" {
			continue
		}

		msg = strings.ReplaceAll(msg, secret, redactedPlaceholder)
	}

	return msg
}

// redactionSecrets lists the secret values from auth that must never appear
// in error messages. URL-embedded (percent-encoded) forms are already covered
// by the userinfo regex in redactCredentials.
func redactionSecrets(auth Auth) []string {
	candidates := []string{
		auth.Token,
		strings.TrimSpace(auth.Token),
		auth.Password,
	}

	secrets := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c == "" {
			continue
		}

		secrets = append(secrets, c)
	}

	return secrets
}
