// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import "testing"

// Engine-convergence KEX tests (T04, GTE-2). The offer is read off the production auth path
// (sshAuthMethod -> publicKeysAuth.ClientConfig), not off the package slice, so the test fails
// if the wiring to the client config is broken as well as if the list itself is wrong.
func kexOfferForConvergenceTest(t *testing.T) []string {
	t.Helper()

	key := testEd25519PrivateKeyPEM(t)
	auth, err := sshAuthMethod("kollect", key, SSHConfig{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("sshAuthMethod: %v", err)
	}

	cfg, err := auth.ClientConfig()
	if err != nil {
		t.Fatalf("ClientConfig: %v", err)
	}

	return cfg.KeyExchanges
}

func kexOfferContains(t *testing.T, offer []string, want string) {
	t.Helper()

	for _, name := range offer {
		if name == want {
			return
		}
	}

	t.Fatalf("KEX offer missing %s (want present in the go-git SSH offer)", want)
}

// Engine-convergence red (T04, GTE-2 scenario 1): the go-git SSH key-exchange offer must carry
// the modern algorithms x/crypto already implements. Red until T09 extends the pin
// (ssh_auth.go defaultSSHKeyExchangeAlgorithms): today the offer holds only the legacy eight.
func TestSSHKeyExchangeOffer_carriesModernAlgorithms(t *testing.T) {
	t.Parallel()

	offer := kexOfferForConvergenceTest(t)

	kexOfferContains(t, offer, "curve25519-sha256")
	kexOfferContains(t, offer, "mlkem768x25519-sha256")
	kexOfferContains(t, offer, "diffie-hellman-group16-sha512")
}

// existingKEXNamesInOrder pins today's eight algorithm names in their current relative order
// (ssh_auth.go at T04). GTE-2 scenario 2: existing preferences are not demoted.
var existingKEXNamesInOrder = []string{
	"curve25519-sha256",
	"curve25519-sha256@libssh.org",
	"ecdh-sha2-nistp256",
	"ecdh-sha2-nistp384",
	"ecdh-sha2-nistp521",
	"diffie-hellman-group-exchange-sha256",
	"diffie-hellman-group14-sha256",
	"diffie-hellman-group14-sha1",
}

// Guard (green-by-construction, GTE-2 scenario 2): the eight existing names keep their current
// relative order in the offer. Passes at HEAD and must keep passing when T09 appends the modern
// algorithms, so the extension never demotes an existing preference.
func TestSSHKeyExchangeOffer_existingEightKeepRelativeOrder(t *testing.T) {
	t.Parallel()

	offer := kexOfferForConvergenceTest(t)

	positions := make(map[string]int, len(offer))
	for i, name := range offer {
		positions[name] = i
	}

	last := -1
	for _, name := range existingKEXNamesInOrder {
		pos, ok := positions[name]
		if !ok {
			t.Fatalf("KEX offer dropped existing algorithm %s (want all eight kept)", name)
		}
		if pos <= last {
			t.Fatalf("KEX offer reordered existing algorithm %s (position %d, want after %d)", name, pos, last)
		}
		last = pos
	}
}
