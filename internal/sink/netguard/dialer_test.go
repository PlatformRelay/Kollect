// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"testing"
)

type fakeResolver struct {
	answers [][]netip.Addr
	calls   int
}

func TestHTTPClientGuardsEveryTransportDialAndDisablesProxyResolution(t *testing.T) {
	t.Parallel()

	client := HTTPClient(0)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", client.Transport)
	}
	if transport.DialContext == nil {
		t.Fatal("HTTP transport has no guarded DialContext")
	}
	if transport.Proxy != nil {
		t.Fatal("HTTP proxy could resolve redirect targets outside the guarded dialer")
	}
}

func (r *fakeResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	answer := r.answers[r.calls]
	r.calls++

	return answer, nil
}

func TestDialerRejectsUnsafeResolvedAddressesBeforeDial(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		addrs []netip.Addr
	}{
		{name: "loopback", addrs: []netip.Addr{netip.MustParseAddr("127.0.0.1")}},
		{name: "RFC1918", addrs: []netip.Addr{netip.MustParseAddr("10.20.30.40")}},
		{name: "link-local metadata", addrs: []netip.Addr{netip.MustParseAddr("169.254.169.254")}},
		{name: "IPv6 loopback", addrs: []netip.Addr{netip.MustParseAddr("::1")}},
		{name: "IPv6 ULA", addrs: []netip.Addr{netip.MustParseAddr("fd00::1")}},
		{name: "mixed answers", addrs: []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
			netip.MustParseAddr("192.168.1.10"),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dialed := false
			d := NewDialer(&fakeResolver{answers: [][]netip.Addr{tt.addrs}}, func(
				context.Context, string, string,
			) (net.Conn, error) {
				dialed = true

				return nil, errors.New("unexpected dial")
			})

			if _, err := d.DialContext(context.Background(), "tcp", "sink.example:443"); err == nil {
				t.Fatal("expected unsafe resolved address to be rejected")
			}
			if dialed {
				t.Fatal("socket opened before all DNS answers were authorized")
			}
		})
	}
}

func TestDialerUsesAuthorizedResolvedIPAndRechecksDNSPerDial(t *testing.T) {
	t.Parallel()

	resolver := &fakeResolver{answers: [][]netip.Addr{
		{netip.MustParseAddr("93.184.216.34")},
		{netip.MustParseAddr("127.0.0.1")},
	}}
	dialed := make([]string, 0, 1)
	d := NewDialer(resolver, func(_ context.Context, _, address string) (net.Conn, error) {
		dialed = append(dialed, address)

		return nil, errors.New("fixture stops after authorized dial")
	})

	_, _ = d.DialContext(context.Background(), "tcp", "sink.example:443")
	if len(dialed) != 1 || dialed[0] != "93.184.216.34:443" {
		t.Fatalf("dialed addresses = %v, want the authorized resolved IP", dialed)
	}

	if _, err := d.DialContext(context.Background(), "tcp", "sink.example:443"); err == nil {
		t.Fatal("expected rebound loopback answer to be rejected")
	}
	if len(dialed) != 1 {
		t.Fatalf("rebound address reached socket dial: %v", dialed)
	}
}

func TestDialerRejectsMetadataHostnameWithoutResolution(t *testing.T) {
	t.Parallel()

	resolver := &fakeResolver{answers: [][]netip.Addr{{netip.MustParseAddr("93.184.216.34")}}}
	d := NewDialer(resolver, nil)
	if _, err := d.DialContext(context.Background(), "tcp", "metadata.google.internal:80"); err == nil {
		t.Fatal("expected metadata hostname to be rejected")
	}
	if resolver.calls != 0 {
		t.Fatalf("metadata hostname unexpectedly resolved %d times", resolver.calls)
	}
}
