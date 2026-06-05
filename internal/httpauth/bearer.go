// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel
package httpauth
import ("fmt"; "strings")
func BearerToken(header string) (string, error) {
	if header == "" { return "", fmt.Errorf("missing Authorization header") }
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) { return "", fmt.Errorf("expected Bearer token") }
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" { return "", fmt.Errorf("empty bearer token") }
	return token, nil
}
