// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// allowPrivateSinks is the NET-01 production opt-in (DR-FIND-05 Option A) mirror
// of netguard's dial-time flag: when true, admission permits literal RFC1918 /
// IPv6-ULA sink endpoints (in-cluster ClusterIP services) while every other
// deny-list entry below still applies. Default false (deny by default); set
// process-wide from the manager --allow-private-sinks flag via
// SetAllowPrivateSinks. Atomic so concurrent admission reads stay race-free
// (mirrors maxExportBytesGlobal). This never widens the loopback/link-local/
// metadata/CGNAT/benchmark denials and is not a CRD/tenant-controllable field.
var allowPrivateSinks atomic.Bool

// SetAllowPrivateSinks configures whether admission permits literal private
// (RFC1918/ULA) sink endpoints (NET-01). Keep consistent with
// netguard.SetAllowPrivateSinks — both derive from the one manager flag.
func SetAllowPrivateSinks(allow bool) {
	allowPrivateSinks.Store(allow)
}

// SAFE (SonarCloud go:S1313 "hardcoded IP address"): the literal CIDRs and
// hostnames below are the intentional SSRF deny-list security control, not
// accidental hardcoded configuration. They block sink endpoints from
// resolving to loopback, link-local, carrier-grade-NAT, benchmark-testing,
// or well-known cloud-metadata targets (e.g. the AWS/GCP instance-metadata
// service at 169.254.169.254 and the AWS IPv6 IMDS endpoint fd00:ec2::254),
// which are classic SSRF pivot points.
//
// Do NOT "fix" this finding by externalizing these values into a
// configurable/overridable list (env var, ConfigMap, CRD field, etc.). Doing
// so would let an attacker-influenced input widen or bypass the deny-list,
// which defeats the control this code exists to provide. Changing deny-list
// membership is a security-relevant decision that needs its own review, not
// a side effect of a Sonar cleanup.
//
// Verified by hack/test/sonar_ko_07_deny_cidr_comment_test.sh, which locks in
// both this comment and the known-critical entries below staying verbatim.
var (
	denyCIDRs = []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),       // loopback
		netip.MustParsePrefix("169.254.0.0/16"),    // link-local + cloud metadata
		netip.MustParsePrefix("100.64.0.0/10"),     // carrier-grade NAT
		netip.MustParsePrefix("198.18.0.0/15"),     // benchmark testing
		netip.MustParsePrefix("::1/128"),           // IPv6 loopback
		netip.MustParsePrefix("fe80::/10"),         // IPv6 link-local
		netip.MustParsePrefix("fd00:ec2::254/128"), // IPv6 cloud metadata (AWS IMDS)
	}

	denyHostnames = map[string]struct{}{
		"localhost":                  {},
		"localhost.localdomain":      {},
		"metadata":                   {},
		"metadata.google.internal":   {},
		"instance-data":              {},
		"instance-data.ec2.internal": {},
	}
)

func validateSnapshotSinkEndpointGuards(spec *kollectdevv1alpha1.KollectSnapshotSinkSpec) field.ErrorList {
	switch spec.Type {
	case kollectdevv1alpha1.SnapshotSinkTypeGit, kollectdevv1alpha1.SnapshotSinkTypeGitLab:
		return validateGitRemoteTarget(spec.Endpoint, field.NewPath("spec").Child("endpoint"))
	case kollectdevv1alpha1.SnapshotSinkTypeS3, kollectdevv1alpha1.SnapshotSinkTypeGCS:
		return validateObjectStoreEndpoint(spec.Endpoint, field.NewPath("spec").Child("endpoint"))
	default:
		return nil
	}
}

func validateEventSinkEndpointGuards(spec *kollectdevv1alpha1.KollectEventSinkSpec) field.ErrorList {
	switch spec.Type {
	case kollectdevv1alpha1.EventSinkTypeNats:
		if spec.Nats != nil && strings.TrimSpace(spec.Nats.URL) != "" {
			return validateURLTarget(spec.Nats.URL, field.NewPath("spec").Child("nats").Child("url"))
		}
		return validateURLTarget(spec.Endpoint, field.NewPath("spec").Child("endpoint"))
	case kollectdevv1alpha1.EventSinkTypeKafka:
		if spec.Kafka == nil {
			return nil
		}
		var allErrs field.ErrorList
		for i, broker := range spec.Kafka.Brokers {
			allErrs = append(allErrs,
				validateBrokerHost(broker, field.NewPath("spec").Child("kafka").Child("brokers").Index(i))...)
		}
		return allErrs
	default:
		return nil
	}
}

func validateLegacySinkEndpointGuards(spec *kollectdevv1alpha1.KollectSinkSpec) field.ErrorList {
	switch spec.Type {
	case kollectdevv1alpha1.SinkTypeGit, kollectdevv1alpha1.SinkTypeGitLab:
		return validateGitRemoteTarget(spec.Endpoint, field.NewPath("spec").Child("endpoint"))
	case kollectdevv1alpha1.SinkTypeS3, kollectdevv1alpha1.SinkTypeGCS:
		return validateObjectStoreEndpoint(spec.Endpoint, field.NewPath("spec").Child("endpoint"))
	case kollectdevv1alpha1.SinkTypeNats:
		if spec.Nats != nil && strings.TrimSpace(spec.Nats.URL) != "" {
			return validateURLTarget(spec.Nats.URL, field.NewPath("spec").Child("nats").Child("url"))
		}
		return validateURLTarget(spec.Endpoint, field.NewPath("spec").Child("endpoint"))
	case kollectdevv1alpha1.SinkTypeKafka:
		if spec.Kafka == nil {
			return nil
		}
		var allErrs field.ErrorList
		for i, broker := range spec.Kafka.Brokers {
			allErrs = append(allErrs,
				validateBrokerHost(broker, field.NewPath("spec").Child("kafka").Child("brokers").Index(i))...)
		}
		return allErrs
	default:
		return nil
	}
}

func validateObjectStoreEndpoint(raw string, path *field.Path) field.ErrorList {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		// Plain bucket syntax (bucket/prefix) does not carry a network target.
		return nil
	}
	if strings.EqualFold(u.Scheme, "s3") || strings.EqualFold(u.Scheme, "gs") {
		return nil
	}
	return validateURLTarget(raw, path)
}

func validateGitRemoteTarget(raw string, path *field.Path) field.ErrorList {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.Contains(raw, "://") {
		return validateURLTarget(raw, path)
	}

	host := parseGitSCPHost(raw)
	if host == "" {
		return nil
	}
	return validateHost(host, path, raw)
}

func validateURLTarget(raw string, path *field.Path) field.ErrorList {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return nil
	}
	if strings.EqualFold(u.Scheme, "file") {
		return field.ErrorList{field.Forbidden(path, "file:// endpoints are not allowed")}
	}
	host := u.Hostname()
	if host == "" {
		return nil
	}
	return validateHost(host, path, raw)
}

func validateBrokerHost(raw string, path *field.Path) field.ErrorList {
	host := strings.TrimSpace(raw)
	if host == "" {
		return nil
	}

	if strings.Contains(host, "://") {
		u, err := url.Parse(host)
		if err == nil {
			host = u.Hostname()
		}
	} else if parsedHost, ok := hostFromBroker(host); ok {
		host = parsedHost
	}

	return validateHost(host, path, raw)
}

func validateHost(host string, path *field.Path, raw string) field.ErrorList {
	normalized := strings.Trim(strings.ToLower(strings.TrimSpace(host)), ".")
	if normalized == "" {
		return nil
	}

	if _, deny := denyHostnames[normalized]; deny || strings.HasSuffix(normalized, ".localhost") {
		return field.ErrorList{field.Forbidden(path, fmt.Sprintf("endpoint host %q is not allowed", host))}
	}

	addr, err := netip.ParseAddr(normalized)
	if err != nil {
		return nil
	}
	if isDeniedIP(addr) {
		return field.ErrorList{field.Forbidden(path, fmt.Sprintf("endpoint host %q is not allowed", raw))}
	}
	return nil
}

func isDeniedIP(addr netip.Addr) bool {
	if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() {
		return true
	}
	// RFC1918 / IPv6-ULA are denied unless the NET-01 production opt-in is on
	// (--allow-private-sinks). Every other deny-list entry stays enforced
	// regardless, including the link-local/metadata range above and the
	// denyCIDRs below (169.254.0.0/16 covers 169.254.169.254, and
	// fd00:ec2::254/128 carves the AWS IPv6 IMDS literal back out of the
	// otherwise-permitted IPv6-ULA range).
	if addr.IsPrivate() && !allowPrivateSinks.Load() {
		return true
	}
	// Prefix.Contains returns false for a zoned IPv6 addr, so strip the zone
	// first — a zoned literal (e.g. fd00:ec2::254%eth0) is the same forbidden
	// target and must still match the deny-list.
	unzoned := addr.WithZone("")
	for _, prefix := range denyCIDRs {
		if prefix.Contains(unzoned) {
			return true
		}
	}
	return false
}

func parseGitSCPHost(raw string) string {
	if i := strings.Index(raw, ":"); i > 0 && !strings.Contains(raw[:i], "/") {
		left := raw[:i]
		if at := strings.LastIndex(left, "@"); at >= 0 {
			left = left[at+1:]
		}
		return strings.TrimSpace(left)
	}
	return ""
}

func hostFromBroker(raw string) (string, bool) {
	if strings.HasPrefix(raw, "[") {
		host, _, err := net.SplitHostPort(raw)
		if err == nil {
			return strings.Trim(host, "[]"), true
		}
	}
	host, _, err := net.SplitHostPort(raw)
	if err == nil {
		return strings.Trim(host, "[]"), true
	}
	if addr, err := netip.ParseAddr(raw); err == nil {
		return addr.String(), true
	}
	if strings.Count(raw, ":") == 0 {
		return raw, true
	}
	if i := strings.LastIndex(raw, ":"); i > 0 {
		return raw[:i], true
	}
	return raw, false
}
