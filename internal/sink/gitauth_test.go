// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"testing"
)

func TestGitAuthFromSecretData(t *testing.T) {
	t.Parallel()

	if auth := GitAuthFromSecretData(nil, ""); auth.Username != "" || auth.Token != "" {
		t.Fatalf("nil data: %+v", auth)
	}

	auth := GitAuthFromSecretData(map[string][]byte{
		"username": []byte("bot"),
		"password": []byte("pw"),
		"token":    []byte("tok"),
	}, "token")
	if auth.Username != "bot" || auth.Password != "pw" || auth.Token != "tok" {
		t.Fatalf("auth = %+v", auth)
	}
}
