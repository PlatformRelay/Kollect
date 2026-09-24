// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import "strings"

const scrubReason = "sensitive-key"

var defaultScrubKeys = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"apikey",
	"api_key",
	"privatekey",
	"private_key",
	"credential",
	"auth",
	"clientsecret",
	"connectionstring",
	"authorization",
	"proxyauthorization",
	"bearertoken",
}

// highRiskStems are matched by prefix as well as exact/suffix: the bearer
// material is the value, so key spellings such as "authorizationHeader" or
// "bearer_token" (K-24) must redact too. Costs the occasional innocuous
// "authorizer"-style attribute; redaction is the safe side of that trade.
var highRiskStems = []string{"authorization", "bearer"}

func redactedValue() map[string]any {
	return map[string]any{
		"redacted": true,
		"reason":   scrubReason,
	}
}

// Scrubber redacts sensitive keys from extracted attribute values before store insert (ADR-0303).
type Scrubber struct {
	keys map[string]struct{}
}

// NewScrubber returns a scrubber with built-in sensitive keys plus optional operator extensions.
func NewScrubber(extraKeys []string) *Scrubber {
	s := &Scrubber{keys: make(map[string]struct{}, len(defaultScrubKeys)+len(extraKeys))}

	for _, key := range defaultScrubKeys {
		s.keys[normalizeScrubKey(key)] = struct{}{}
	}

	for _, key := range extraKeys {
		if normalized := normalizeScrubKey(key); normalized != "" {
			s.keys[normalized] = struct{}{}
		}
	}

	return s
}

func normalizeScrubKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

// ScrubAttributes redacts sensitive material in every extracted attribute value.
func (s *Scrubber) ScrubAttributes(attrs map[string]any) map[string]any {
	if len(attrs) == 0 {
		return attrs
	}

	out := make(map[string]any, len(attrs))
	for name, val := range attrs {
		out[name] = s.Scrub(val)
	}

	return out
}

// Scrub recursively redacts sensitive keys in maps and slices.
func (s *Scrubber) Scrub(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return s.scrubMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, elem := range typed {
			out[i] = s.Scrub(elem)
		}

		return out
	default:
		return v
	}
}

func (s *Scrubber) scrubMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for key, val := range m {
		if s.shouldRedactKey(key, val) {
			out[key] = redactedValue()

			continue
		}

		out[key] = s.Scrub(val)
	}

	return out
}

func (s *Scrubber) shouldRedactKey(key string, val any) bool {
	if s.isSensitiveKey(key) {
		return true
	}

	return isCredentialCarrier(key, val)
}

func (s *Scrubber) isSensitiveKey(key string) bool {
	normalized := normalizeScrubKey(key)
	if normalized == "" {
		return false
	}

	if _, ok := s.keys[normalized]; ok {
		return true
	}

	for deny := range s.keys {
		if len(deny) > 2 && strings.HasSuffix(normalized, deny) {
			return true
		}
	}

	for _, stem := range highRiskStems {
		if strings.HasPrefix(normalized, stem) {
			return true
		}
	}

	return false
}

func isCredentialCarrier(key string, val any) bool {
	m, ok := val.(map[string]any)
	if !ok {
		return false
	}

	switch normalizeScrubKey(key) {
	case "valuefrom":
		_, has := m["secretKeyRef"]

		return has
	case "secretkeyref", "secretref":
		return true
	case "tls":
		_, hasKey := m["key"]

		return hasKey
	default:
		return false
	}
}
