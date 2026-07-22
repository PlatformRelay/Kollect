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
// It carries no mutable policy state; DNS is resolved afresh for every dial.
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
		return nil, "", "", hostErr
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

	return validateAddress(address)
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
	normalized := strings.Trim(strings.ToLower(strings.TrimSpace(host)), ".")
	switch normalized {
	case "localhost", "localhost.localdomain", "metadata", "metadata.google.internal",
		"instance-data", "instance-data.ec2.internal":
		return fmt.Errorf("sink host %q is forbidden", host)
	}
	if strings.HasSuffix(normalized, ".localhost") {
		return fmt.Errorf("sink host %q is forbidden", host)
	}

	return nil
}

func validateAddress(address netip.Addr) error {
	address = address.Unmap()
	if !address.IsValid() || address.IsUnspecified() || address.IsLoopback() || address.IsPrivate() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() {
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
