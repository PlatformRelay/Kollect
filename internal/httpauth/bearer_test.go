// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel
package httpauth
import "testing"
func TestBearerTokenErrors(t *testing.T) {
	t.Parallel()
	for _, h := range []string{"", "Token abc", "Bearer "} {
		if _, err := BearerToken(h); err == nil { t.Fatalf("%q: want error", h) }
	}
}
func TestBearerTokenSuccess(t *testing.T) {
	t.Parallel()
	if tok, err := BearerToken("Bearer tok-123"); err != nil || tok != "tok-123" { t.Fatal(err) }
}
