// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"net/netip"
	"strings"
	"testing"

	"github.com/platformrelay/kollect/internal/sink/netguard"
)

type staticResolver []netip.Addr

func (r staticResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r, nil
}

func TestCLIResolutionPinsAuthorizedHTTPAddress(t *testing.T) {
	old := netguard.DefaultDialer
	netguard.DefaultDialer = netguard.NewDialer(staticResolver{netip.MustParseAddr("93.184.216.34")}, nil)
	t.Cleanup(func() { netguard.DefaultDialer = old })

	cli := &cliEnv{}
	if err := cli.guardResolution(t.Context(), "https://git.example/repo.git"); err != nil {
		t.Fatalf("guardResolution: %v", err)
	}
	joined := strings.Join(cli.extraEnv, "\n")
	if !strings.Contains(joined, "http.curloptResolve") ||
		!strings.Contains(joined, "+git.example:443:93.184.216.34") {
		t.Fatalf("git curl resolution was not pinned: %s", joined)
	}
}

func TestCLIResolutionRejectsMixedAnswers(t *testing.T) {
	old := netguard.DefaultDialer
	netguard.DefaultDialer = netguard.NewDialer(staticResolver{
		netip.MustParseAddr("93.184.216.34"),
		netip.MustParseAddr("10.0.0.8"),
	}, nil)
	t.Cleanup(func() { netguard.DefaultDialer = old })

	cli := &cliEnv{}
	if err := cli.guardResolution(t.Context(), "https://git.example/repo.git"); err == nil {
		t.Fatal("expected mixed public/private answer set to be rejected")
	}
	if len(cli.extraEnv) != 0 {
		t.Fatalf("CLI environment mutated before full answer-set authorization: %v", cli.extraEnv)
	}
}

func TestCLIResolutionPinsSSHAddressAndHostKeyAlias(t *testing.T) {
	old := netguard.DefaultDialer
	netguard.DefaultDialer = netguard.NewDialer(staticResolver{netip.MustParseAddr("2001:db8::20")}, nil)
	t.Cleanup(func() { netguard.DefaultDialer = old })

	cli := &cliEnv{extraEnv: []string{"GIT_SSH_COMMAND=ssh -i /tmp/key"}}
	if err := cli.guardResolution(t.Context(), "ssh://git@git.example/repo.git"); err != nil {
		t.Fatalf("guardResolution: %v", err)
	}
	joined := strings.Join(cli.extraEnv, "\n")
	if !strings.Contains(joined, "Hostname='2001:db8::20'") || !strings.Contains(joined, "HostKeyAlias='git.example'") {
		t.Fatalf("SSH resolution was not pinned with hostname verification: %s", joined)
	}
}

func TestCLIResolutionAllowsFileSchemeWithoutPinning(t *testing.T) {
	cli := &cliEnv{}
	if err := cli.guardResolution(t.Context(), "file:///tmp/repo.git"); err != nil {
		t.Fatalf("guardResolution(file): %v", err)
	}
	if len(cli.extraEnv) != 0 {
		t.Fatalf("file:// remotes must not mutate CLI env: %v", cli.extraEnv)
	}
}

func TestCLIResolutionRejectsUnknownScheme(t *testing.T) {
	cli := &cliEnv{}
	if err := cli.guardResolution(t.Context(), "git://git.example/repo.git"); err == nil {
		t.Fatal("expected unknown scheme to fail closed")
	}
	if len(cli.extraEnv) != 0 {
		t.Fatalf("CLI environment mutated for unsupported scheme: %v", cli.extraEnv)
	}
}
