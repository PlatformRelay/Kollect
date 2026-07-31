// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package netguard provides dial-time SSRF protection for outbound sink clients.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// DefaultDialer is the process-wide deny-private dialer used by sink clients.
// DNS is resolved afresh for every dial. Production builds keep allowPrivate
// false; the integration build tag may flip it for loopback testcontainers only.
var DefaultDialer = NewDialer(nil, nil)

// Resolver is the DNS surface used by Dialer. It is intentionally injectable so
// rebinding and mixed-answer behavior can be verified without live DNS.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// HTTPClient returns a client whose transport applies the resolved-address
// policy to initial requests and every redirect target.
func HTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = DefaultDialer.DialContext
	// A proxy resolves the target outside this process and would bypass the
	// checked-IP invariant. Private proxy support requires a separate explicit
	// operator policy; silently inheriting HTTP_PROXY is not safe here.
	transport.Proxy = nil

	return &http.Client{Transport: transport, Timeout: timeout}
}

// DialContextFunc opens a socket to an already-authorized numeric address.
type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Dialer resolves a sink hostname on every connection attempt, rejects the
// entire answer set if any address is non-public, then dials a numeric address.
// Dialing the checked IP (rather than resolving the hostname a second time)
// closes the validation-to-connect DNS rebinding window.
type Dialer struct {
	resolver Resolver
	dial     DialContextFunc
	// allowPrivate exists only for the integration-tagged test build, whose
	// real backends run in loopback/testcontainer networks. Production
	// constructors never enable it.
	allowPrivate bool
	// allowPrivateSinks is the NET-01 production opt-in (DR-FIND-05 Option A):
	// when true it permits RFC1918 / IPv6-ULA sink targets (e.g. in-cluster
	// ClusterIP services) while STILL denying loopback, link-local (which
	// includes the cloud-metadata IP 169.254.169.254), unspecified, multicast,
	// carrier-grade-NAT, and benchmark ranges. It is strictly narrower than
	// allowPrivate. Default false (deny); set process-wide from the manager
	// --allow-private-sinks flag via SetAllowPrivateSinks.
	allowPrivateSinks bool
}

// SetAllowPrivateSinks configures the process-wide DefaultDialer to permit
// RFC1918/ULA sink dials (NET-01). Call once at manager startup from the
// --allow-private-sinks flag before any sink dials; the unset default stays
// deny-by-default. Keep consistent with validation.SetAllowPrivateSinks so
// admission and dial-time apply the same policy.
func SetAllowPrivateSinks(allow bool) {
	DefaultDialer.allowPrivateSinks = allow
}

// NewDialer constructs a guarded dialer. Nil dependencies select the system
// resolver and net.Dialer respectively.
func NewDialer(resolver Resolver, dial DialContextFunc) *Dialer {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if dial == nil {
		base := &net.Dialer{}
		dial = base.DialContext
	}

	return &Dialer{resolver: resolver, dial: dial}
}

// DialContext implements the common context-aware dialer shape used by sink clients.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	addresses, port, host, err := d.Resolve(ctx, network, address)
	if err != nil {
		return nil, err
	}

	var dialErr error
	for _, resolved := range addresses {
		conn, err := d.dial(ctx, network, net.JoinHostPort(resolved.String(), port))
		if err == nil {
			return conn, nil
		}
		dialErr = errors.Join(dialErr, err)
	}

	return nil, fmt.Errorf("dial sink host %q: %w", host, dialErr)
}

// Resolve returns a fully authorized DNS answer set and the parsed endpoint.
// Callers that must delegate socket creation to an external process can pin
// that process to these numeric addresses instead of permitting a second lookup.
func (d *Dialer) Resolve(
	ctx context.Context,
	network, address string,
) (addresses []netip.Addr, port, host string, err error) {
	host, port, err = net.SplitHostPort(address)
	if err != nil {
		return nil, "", "", fmt.Errorf("guard sink address %q: %w", address, err)
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if hostErr := validateHostname(host); hostErr != nil {
		// allowPrivate (integration build only) permits the localhost hostname
		// family used by testcontainers; metadata hostnames stay denied.
		if !d.allowPrivate || isMetadataHostname(host) {
			return nil, "", "", hostErr
		}
	}

	addresses, err = d.resolve(ctx, network, host)
	if err != nil {
		return nil, "", "", err
	}
	for _, resolved := range addresses {
		if err := d.validateAddress(resolved); err != nil {
			return nil, "", "", fmt.Errorf("sink host %q resolved to a forbidden address: %w", host, err)
		}
	}

	return addresses, port, host, nil
}

func (d *Dialer) validateAddress(address netip.Addr) error {
	if d.allowPrivate {
		return nil
	}

	return validateAddress(address, d.allowPrivateSinks)
}

// Dial implements libraries' context-free custom-dialer interfaces. The
// connection attempt remains bounded even when the caller cannot pass a context.
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return d.DialContext(ctx, network, address)
}

func (d *Dialer) resolve(ctx context.Context, network, host string) ([]netip.Addr, error) {
	if literal, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{literal.Unmap()}, nil
	}

	addresses, err := d.resolver.LookupNetIP(ctx, networkForLookup(network), host)
	if err != nil {
		return nil, fmt.Errorf("resolve sink host %q: %w", host, err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("resolve sink host %q: no addresses", host)
	}

	return addresses, nil
}

func networkForLookup(network string) string {
	switch network {
	case "tcp4", "udp4":
		return "ip4"
	case "tcp6", "udp6":
		return "ip6"
	default:
		return "ip"
	}
}

func validateHostname(host string) error {
	normalized := normalizeHostname(host)
	if isLocalhostHostname(normalized) || isMetadataHostnameNormalized(normalized) {
		return fmt.Errorf("sink host %q is forbidden", host)
	}

	return nil
}

func isMetadataHostname(host string) bool {
	return isMetadataHostnameNormalized(normalizeHostname(host))
}

func normalizeHostname(host string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(host)), ".")
}

func isLocalhostHostname(normalized string) bool {
	switch normalized {
	case "localhost", "localhost.localdomain":
		return true
	}

	return strings.HasSuffix(normalized, ".localhost")
}

func isMetadataHostnameNormalized(normalized string) bool {
	switch normalized {
	case "metadata", "metadata.google.internal",
		"instance-data", "instance-data.ec2.internal":
		return true
	default:
		return false
	}
}

// validateAddress rejects non-globally-routable sink targets. When allowPrivate
// is true (the NET-01 opt-in) RFC1918 / IPv6-ULA addresses are permitted, but
// every other denial — loopback, link-local (incl. cloud metadata), unspecified,
// multicast, carrier-grade-NAT, and benchmark — still applies. allowPrivate
// false is the deny-by-default path (fail closed).
func validateAddress(address netip.Addr, allowPrivate bool) error {
	address = address.Unmap()
	if !address.IsValid() || address.IsUnspecified() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() {
		return fmt.Errorf("IP %s is not globally routable", address)
	}
	if address.IsPrivate() && !allowPrivate {
		return fmt.Errorf("IP %s is not globally routable", address)
	}
	if address.Is4() {
		if netip.MustParsePrefix("100.64.0.0/10").Contains(address) ||
			netip.MustParsePrefix("198.18.0.0/15").Contains(address) {
			return fmt.Errorf("IP %s is not globally routable", address)
		}
	}

	return nil
}
