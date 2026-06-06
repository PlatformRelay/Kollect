// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"testing"
)

func TestGitSSHKnownHostsFromSecretData(t *testing.T) {
	t.Parallel()

	if got := GitSSHKnownHostsFromSecretData(nil); got != nil {
		t.Fatalf("nil data = %q", got)
	}

	data := map[string][]byte{"known_hosts": []byte("host key")}
	if got := GitSSHKnownHostsFromSecretData(data); string(got) != "host key" {
		t.Fatalf("got %q", got)
	}
}

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
