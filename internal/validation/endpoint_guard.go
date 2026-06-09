// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var (
	denyCIDRs = []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),    // loopback
		netip.MustParsePrefix("169.254.0.0/16"), // link-local + cloud metadata
		netip.MustParsePrefix("100.64.0.0/10"),  // carrier-grade NAT
		netip.MustParsePrefix("198.18.0.0/15"),  // benchmark testing
		netip.MustParsePrefix("::1/128"),        // IPv6 loopback
		netip.MustParsePrefix("fe80::/10"),      // IPv6 link-local
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
	if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() {
		return true
	}
	for _, prefix := range denyCIDRs {
		if prefix.Contains(addr) {
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
