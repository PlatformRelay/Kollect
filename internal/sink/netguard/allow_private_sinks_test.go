// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package netguard

import (
	"context"
	"net"
	"net/netip"
	"testing"
)

// NET-01: validateAddress permits RFC1918 / IPv6-ULA only when the private-sink
// opt-in is on, and keeps every other denial (loopback, link-local incl. cloud
// metadata, unspecified, multicast, CGNAT, benchmark) enforced regardless.
func TestValidateAddress_allowPrivateSinksMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		addr         string
		allowPrivate bool
		wantAllowed  bool
	}{
		// Off (default, fail closed): private denied.
		{"rfc1918 off denied", "10.20.30.40", false, false},
		{"192.168 off denied", "192.168.1.10", false, false},
		{"ula off denied", "fd00::1", false, false},
		// On: private allowed.
		{"rfc1918 on allowed", "10.20.30.40", true, true},
		{"172.16 on allowed", "172.16.5.5", true, true},
		{"192.168 on allowed", "192.168.1.10", true, true},
		{"ula on allowed", "fd00::1", true, true},
		// On: hard denials remain (the discriminators that matter).
		{"metadata IP stays denied on", "169.254.169.254", true, false},
		// SEC-IMDS6: the AWS IPv6 IMDS literal is IPv6-ULA (IsPrivate) but must
		// stay denied under the opt-in, while generic ULA (fd00::1 above) stays
		// allowed — proving the carve-out is the single literal, not fd00::/8.
		{"ipv6 metadata stays denied on", "fd00:ec2::254", true, false},
		// SEC-IMDS6 F1: a zoned textual form of the exact IMDS literal must not
		// escape the deny — the zone is normalized away before the comparison.
		{"ipv6 metadata zoned stays denied on", "fd00:ec2::254%eth0", true, false},
		{"link-local stays denied on", "169.254.10.10", true, false},
		{"ipv6 link-local stays denied on", "fe80::1", true, false},
		{"loopback stays denied on", "127.0.0.1", true, false},
		{"ipv6 loopback stays denied on", "::1", true, false},
		{"unspecified stays denied on", "0.0.0.0", true, false},
		{"multicast stays denied on", "224.0.0.1", true, false},
		{"cgnat stays denied on", "100.64.0.1", true, false},
		{"benchmark stays denied on", "198.18.0.1", true, false},
		// Public always allowed.
		{"public off allowed", "93.184.216.34", false, true},
		{"public on allowed", "8.8.8.8", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAddress(netip.MustParseAddr(tt.addr), tt.allowPrivate)
			if tt.wantAllowed && err != nil {
				t.Fatalf("validateAddress(%s, %v) = %v, want allowed", tt.addr, tt.allowPrivate, err)
			}
			if !tt.wantAllowed && err == nil {
				t.Fatalf("validateAddress(%s, %v) = nil, want denied", tt.addr, tt.allowPrivate)
			}
		})
	}
}

// NET-01: with the opt-in on, a cluster DNS name resolving to an RFC1918
// ClusterIP dials through (the DR-2b use case that was blocked on v0.10.0),
// while the default deny still blocks it.
func TestDialer_allowPrivateSinks_dnsToRFC1918(t *testing.T) {
	t.Parallel()

	newDialer := func(allow bool) (*Dialer, *bool) {
		dialed := false
		d := NewDialer(
			&fakeResolver{answers: [][]netip.Addr{{netip.MustParseAddr("10.96.0.20")}}},
			func(context.Context, string, string) (net.Conn, error) {
				dialed = true

				return nil, nil //nolint:nilnil // fake dial: success is signalled via the dialed flag
			},
		)
		d.allowPrivateSinks = allow

		return d, &dialed
	}

	// Off: DNS -> RFC1918 is rejected before dial (fail closed).
	dOff, dialedOff := newDialer(false)
	if _, err := dOff.DialContext(context.Background(), "tcp", "postgres.db.svc.cluster.local:5432"); err == nil {
		t.Fatal("expected DNS->RFC1918 to be rejected with the opt-in off")
	}
	if *dialedOff {
		t.Fatal("socket opened to an RFC1918 address with the opt-in off")
	}

	// On: the same resolution is authorized and the dial proceeds.
	dOn, dialedOn := newDialer(true)
	if _, err := dOn.DialContext(context.Background(), "tcp", "postgres.db.svc.cluster.local:5432"); err != nil {
		t.Fatalf("expected DNS->RFC1918 to be allowed with the opt-in on: %v", err)
	}
	if !*dialedOn {
		t.Fatal("expected the dial to proceed to the RFC1918 ClusterIP with the opt-in on")
	}
}

// NET-01: even with the opt-in on, an answer set mixing a private address with a
// still-forbidden one (cloud metadata) is rejected as a whole (fail closed).
func TestDialer_allowPrivateSinks_mixedAnswerFailsClosed(t *testing.T) {
	t.Parallel()

	dialed := false
	d := NewDialer(
		&fakeResolver{answers: [][]netip.Addr{{
			netip.MustParseAddr("10.96.0.20"),      // private (allowed when on)
			netip.MustParseAddr("169.254.169.254"), // metadata (never allowed)
		}}},
		func(context.Context, string, string) (net.Conn, error) {
			dialed = true

			return nil, nil //nolint:nilnil // fake dial
		},
	)
	d.allowPrivateSinks = true

	if _, err := d.DialContext(context.Background(), "tcp", "sink.example:443"); err == nil {
		t.Fatal("expected a mixed private+metadata answer set to be rejected with the opt-in on")
	}
	if dialed {
		t.Fatal("socket opened despite a forbidden address in the answer set")
	}
}

// NET-01: the process-wide setter and default. DefaultDialer must fail closed
// until SetAllowPrivateSinks(true) is called, and the setter must be reversible.
func TestSetAllowPrivateSinks_processWideDefaultClosed(t *testing.T) {
	// Not parallel: mutates the process-wide DefaultDialer.
	t.Cleanup(func() { SetAllowPrivateSinks(false) })

	if DefaultDialer.allowPrivateSinks {
		t.Fatal("DefaultDialer must default to deny (allowPrivateSinks=false)")
	}
	SetAllowPrivateSinks(true)
	if !DefaultDialer.allowPrivateSinks {
		t.Fatal("SetAllowPrivateSinks(true) did not enable the opt-in")
	}
	SetAllowPrivateSinks(false)
	if DefaultDialer.allowPrivateSinks {
		t.Fatal("SetAllowPrivateSinks(false) did not restore deny")
	}
}
