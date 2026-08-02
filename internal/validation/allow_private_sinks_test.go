// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"net/netip"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func endpointPath() *field.Path { return field.NewPath("spec").Child("endpoint") }

// allowPrivateSinksTestMu serializes tests that mutate the process-global NET-01
// opt-in. Production stays on atomic.Bool for concurrent admission reads; tests
// that flip it must not overlap set→assert→restore windows, and must NOT run
// under t.Parallel (they would race default-false-assuming parallel tests on the
// value, even though atomic access keeps the data-race detector quiet).
var allowPrivateSinksTestMu sync.Mutex

// withAllowPrivateSinks installs allow as the admission opt-in for the duration
// of t, restoring the deny default on cleanup. Do not call t.Parallel in tests
// that use this helper.
func withAllowPrivateSinks(t *testing.T, allow bool) {
	t.Helper()

	allowPrivateSinksTestMu.Lock()
	SetAllowPrivateSinks(allow)
	t.Cleanup(func() {
		SetAllowPrivateSinks(false)
		allowPrivateSinksTestMu.Unlock()
	})
}

// NET-01: admission denies literal private IPs by default, permits RFC1918/ULA
// only when the opt-in is on, and keeps every hard denial (loopback,
// link-local/metadata, CGNAT, benchmark, unspecified) enforced regardless.
func TestIsDeniedIP_allowPrivateSinksMatrix(t *testing.T) {
	tests := []struct {
		name       string
		addr       string
		allow      bool
		wantDenied bool
	}{
		// Default deny (fail closed).
		{"rfc1918 off denied", "192.168.1.1", false, true},
		{"10/8 off denied", "10.1.2.3", false, true},
		{"ula off denied", "fd00::1", false, true},
		// Opt-in on: private permitted.
		{"rfc1918 on allowed", "192.168.1.1", true, false},
		{"172.16 on allowed", "172.16.9.9", true, false},
		{"ula on allowed", "fd00::1", true, false},
		// Opt-in on: hard denials remain.
		{"metadata IP stays denied on", "169.254.169.254", true, true},
		// SEC-IMDS6: the AWS IPv6 IMDS literal is IPv6-ULA (IsPrivate) but must
		// stay denied under the opt-in, while generic ULA (fd00::1 above) stays
		// allowed — proving the carve-out is the single literal, not fd00::/8.
		{"ipv6 metadata stays denied on", "fd00:ec2::254", true, true},
		{"link-local stays denied on", "169.254.1.1", true, true},
		{"ipv6 link-local stays denied on", "fe80::1", true, true},
		{"loopback stays denied on", "127.0.0.1", true, true},
		{"ipv6 loopback stays denied on", "::1", true, true},
		{"cgnat stays denied on", "100.64.0.1", true, true},
		{"benchmark stays denied on", "198.18.0.1", true, true},
		{"unspecified stays denied on", "0.0.0.0", true, true},
		// Public always allowed.
		{"public off allowed", "8.8.8.8", false, false},
		{"public on allowed", "93.184.216.34", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withAllowPrivateSinks(t, tt.allow)

			got := isDeniedIP(netip.MustParseAddr(tt.addr))
			if got != tt.wantDenied {
				t.Fatalf("isDeniedIP(%s) with allow=%v = %v, want denied=%v",
					tt.addr, tt.allow, got, tt.wantDenied)
			}
		})
	}
}

// NET-01: admission and dial-time must apply the same private-allow decision.
// This asserts the admission layer's literal-ClusterIP behaviour flips with the
// flag while a literal metadata endpoint stays forbidden either way.
func TestValidateHost_allowPrivateSinksConsistency(t *testing.T) {
	withAllowPrivateSinks(t, true)

	// Literal RFC1918 ClusterIP: admitted under the opt-in.
	if errs := validateURLTarget("https://10.96.0.20:5432", endpointPath()); len(errs) != 0 {
		t.Fatalf("expected literal RFC1918 endpoint admitted under opt-in, got %v", errs)
	}
	// Cloud-metadata IP: still forbidden even under the opt-in.
	if errs := validateURLTarget("https://169.254.169.254", endpointPath()); len(errs) == 0 {
		t.Fatal("expected cloud-metadata endpoint to stay forbidden under the opt-in")
	}
}

// NET-01: the setter defaults closed — a literal ClusterIP is rejected when the
// opt-in was never enabled.
func TestSetAllowPrivateSinks_defaultClosed(t *testing.T) {
	// No withAllowPrivateSinks call: rely on the deny default; guard against a
	// leaked global from another test by asserting under the serializing lock.
	allowPrivateSinksTestMu.Lock()
	t.Cleanup(func() {
		SetAllowPrivateSinks(false)
		allowPrivateSinksTestMu.Unlock()
	})

	if errs := validateURLTarget("https://10.96.0.20:5432", endpointPath()); len(errs) == 0 {
		t.Fatal("expected literal RFC1918 endpoint to be rejected by default (deny)")
	}
}
