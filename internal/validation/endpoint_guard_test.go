// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"net/netip"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestValidateSnapshotSinkSpec_endpointHostGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		spec      kollectdevv1alpha1.KollectSnapshotSinkSpec
		wantError bool
	}{
		{
			name: "allow public https git endpoint",
			spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "https://github.com/konih/kollect.git",
				},
				Git: &kollectdevv1alpha1.GitSpec{},
			},
		},
		{
			name: "allow public git scp endpoint",
			spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "git@github.com:konih/kollect.git",
				},
				Git: &kollectdevv1alpha1.GitSpec{},
			},
		},
		{
			name: "deny localhost git endpoint",
			spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "https://localhost/repo.git",
				},
				Git: &kollectdevv1alpha1.GitSpec{},
			},
			wantError: true,
		},
		{
			name: "deny link local git endpoint",
			spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "https://169.254.169.254/repo.git",
				},
				Git: &kollectdevv1alpha1.GitSpec{},
			},
			wantError: true,
		},
		{
			name: "deny metadata hostname endpoint",
			spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeGitLab,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "https://metadata.google.internal/group/repo.git",
				},
			},
			wantError: true,
		},
		{
			name: "deny file scheme git endpoint",
			spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeGit,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "file:///tmp/repo.git",
				},
				Git: &kollectdevv1alpha1.GitSpec{},
			},
			wantError: true,
		},
		{
			name: "allow bucket style s3 endpoint",
			spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeS3,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "s3://kollect-exports/inventory",
				},
			},
		},
		{
			name: "deny private custom s3 endpoint",
			spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: kollectdevv1alpha1.SnapshotSinkTypeS3,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "https://10.0.0.5:9000/kollect-exports/inventory",
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := ValidateSnapshotSinkSpec(&tt.spec)
			if tt.wantError && len(errs) == 0 {
				t.Fatalf("expected validation error, got none")
			}
			if !tt.wantError && len(errs) > 0 {
				t.Fatalf("expected no validation error, got %v", errs)
			}
		})
	}
}

func TestValidateEventSinkSpec_endpointHostGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		spec      kollectdevv1alpha1.KollectEventSinkSpec
		wantError bool
	}{
		{
			name: "allow public nats url",
			spec: kollectdevv1alpha1.KollectEventSinkSpec{
				Type: kollectdevv1alpha1.EventSinkTypeNats,
				Nats: &kollectdevv1alpha1.NatsSpec{
					URL:     "nats://nats.example.com:4222",
					Subject: "inventory.>",
				},
			},
		},
		{
			name: "deny private nats url",
			spec: kollectdevv1alpha1.KollectEventSinkSpec{
				Type: kollectdevv1alpha1.EventSinkTypeNats,
				Nats: &kollectdevv1alpha1.NatsSpec{
					URL:     "nats://127.0.0.1:4222",
					Subject: "inventory.>",
				},
			},
			wantError: true,
		},
		{
			name: "deny metadata nats endpoint fallback",
			spec: kollectdevv1alpha1.KollectEventSinkSpec{
				Type: kollectdevv1alpha1.EventSinkTypeNats,
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					Endpoint: "nats://metadata.google.internal:4222",
				},
				Nats: &kollectdevv1alpha1.NatsSpec{Subject: "inventory.>"},
			},
			wantError: true,
		},
		{
			name: "allow public kafka brokers",
			spec: kollectdevv1alpha1.KollectEventSinkSpec{
				Type: kollectdevv1alpha1.EventSinkTypeKafka,
				Kafka: &kollectdevv1alpha1.KafkaSpec{
					Brokers: []string{"kafka.example.com:9092"},
					Topic:   "inventory",
				},
			},
		},
		{
			name: "deny localhost kafka broker",
			spec: kollectdevv1alpha1.KollectEventSinkSpec{
				Type: kollectdevv1alpha1.EventSinkTypeKafka,
				Kafka: &kollectdevv1alpha1.KafkaSpec{
					Brokers: []string{"localhost:9092"},
					Topic:   "inventory",
				},
			},
			wantError: true,
		},
		{
			name: "deny ipv6 loopback kafka broker",
			spec: kollectdevv1alpha1.KollectEventSinkSpec{
				Type: kollectdevv1alpha1.EventSinkTypeKafka,
				Kafka: &kollectdevv1alpha1.KafkaSpec{
					Brokers: []string{"[::1]:9092"},
					Topic:   "inventory",
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := ValidateEventSinkSpec(&tt.spec)
			if tt.wantError && len(errs) == 0 {
				t.Fatalf("expected validation error, got none")
			}
			if !tt.wantError && len(errs) > 0 {
				t.Fatalf("expected no validation error, got %v", errs)
			}
		})
	}
}

func TestValidateSinkSpec_legacyEndpointHostGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		spec      kollectdevv1alpha1.KollectSinkSpec
		wantError bool
	}{
		{
			name: "deny legacy git private endpoint",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:     kollectdevv1alpha1.SinkTypeGit,
				Endpoint: "https://10.10.10.10/repo.git",
				Git:      &kollectdevv1alpha1.GitSpec{},
			},
			wantError: true,
		},
		{
			name: "allow legacy kafka public broker",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: kollectdevv1alpha1.SinkTypeKafka,
				Kafka: &kollectdevv1alpha1.KafkaSpec{
					Brokers: []string{"kafka.example.com:9092"},
					Topic:   "inventory",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := ValidateSinkSpec(&tt.spec)
			if tt.wantError && len(errs) == 0 {
				t.Fatalf("expected validation error, got none")
			}
			if !tt.wantError && len(errs) > 0 {
				t.Fatalf("expected no validation error, got %v", errs)
			}
		})
	}
}

func TestIsDeniedIP(t *testing.T) {
	t.Parallel()

	parseAddr := func(s string) netip.Addr {
		a, _ := netip.ParseAddr(s)
		return a
	}

	if !isDeniedIP(parseAddr("192.168.1.1")) {
		t.Error("private RFC1918 address must be denied")
	}
	if !isDeniedIP(parseAddr("100.64.0.1")) {
		t.Error("carrier-grade NAT address must be denied via denyCIDRs")
	}
	if !isDeniedIP(parseAddr("198.18.0.1")) {
		t.Error("benchmark-testing address must be denied via denyCIDRs")
	}
	if isDeniedIP(parseAddr("8.8.8.8")) {
		t.Error("public address must not be denied")
	}
}

func TestValidateBrokerHost_branches(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec").Child("brokers").Index(0)

	// empty host → no error (line 150-152)
	if errs := validateBrokerHost("", path); len(errs) != 0 {
		t.Fatalf("empty: %v", errs)
	}

	// host with URL scheme → url.Parse path (line 154-158)
	if errs := validateBrokerHost("kafka://broker.example.com:9092", path); len(errs) != 0 {
		t.Fatalf("url scheme: %v", errs)
	}

	// plain host:port → hostFromBroker path → valid public host
	if errs := validateBrokerHost("broker.example.com:9092", path); len(errs) != 0 {
		t.Fatalf("host:port: %v", errs)
	}

	// plain host → hostFromBroker: no colon → returns raw, true
	if errs := validateBrokerHost("broker.example.com", path); len(errs) != 0 {
		t.Fatalf("plain host: %v", errs)
	}

	// bare IPv6 address (no port) → hostFromBroker: ParseAddr succeeds; 2001:db8 is documentation range
	_ = validateBrokerHost("2001:db8::1", path) // must not panic

	// IPv6 bracket syntax → [::1]:9092 → SplitHostPort; loopback is denied
	errsIPv6 := validateBrokerHost("[::1]:9092", path)
	if len(errsIPv6) == 0 {
		t.Fatal("loopback must be denied")
	}
}

func TestValidateURLTarget_branches(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec").Child("endpoint")

	// empty → nil (line 131-133)
	if errs := validateURLTarget("", path); len(errs) != 0 {
		t.Fatalf("empty: %v", errs)
	}

	// bad url → url.Parse error or empty scheme → nil (line 135-137)
	if errs := validateURLTarget("not-a-url-no-scheme", path); len(errs) != 0 {
		t.Fatalf("no scheme: %v", errs)
	}

	// file:// → forbidden (line 138-140)
	errs := validateURLTarget("file:///etc/passwd", path)
	if len(errs) != 1 {
		t.Fatalf("file://: expected 1 error, got %v", errs)
	}

	// http url with no host → nil (line 142-144)
	if errs := validateURLTarget("http://", path); len(errs) != 0 {
		t.Fatalf("no host: %v", errs)
	}

	// valid public host → no error (line 145)
	if errs := validateURLTarget("https://api.example.com", path); len(errs) != 0 {
		t.Fatalf("valid: %v", errs)
	}
}

func TestValidateGitRemoteTarget_branches(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec").Child("remote")

	// empty → nil (line 115-117)
	if errs := validateGitRemoteTarget("", path); len(errs) != 0 {
		t.Fatalf("empty: %v", errs)
	}

	// SCP host that parseGitSCPHost returns empty for → nil (line 123-125)
	if errs := validateGitRemoteTarget("justaplainstring", path); len(errs) != 0 {
		t.Fatalf("no SCP match: %v", errs)
	}

	// SCP format with valid host (line 126)
	if errs := validateGitRemoteTarget("git@github.com:org/repo.git", path); len(errs) != 0 {
		t.Fatalf("SCP valid: %v", errs)
	}

	// URL format → validateURLTarget (line 118-120)
	if errs := validateGitRemoteTarget("https://github.com/org/repo.git", path); len(errs) != 0 {
		t.Fatalf("URL: %v", errs)
	}
}

func TestHostFromBroker_paths(t *testing.T) {
	t.Parallel()

	// plain hostname (no colon) → returns raw, true (line 223-225)
	if host, ok := hostFromBroker("broker.example.com"); !ok || host != "broker.example.com" {
		t.Fatalf("plain host: got %q,%v", host, ok)
	}

	// host:port → SplitHostPort (line 216-218)
	if host, ok := hostFromBroker("broker.example.com:9092"); !ok || host != "broker.example.com" {
		t.Fatalf("host:port: got %q,%v", host, ok)
	}

	// bare IPv6 → ParseAddr (line 220-222)
	if host, ok := hostFromBroker("::1"); !ok || host == "" {
		t.Fatalf("bare IPv6: got %q,%v", host, ok)
	}

	// multiple colons but not valid IPv6/host:port → LastIndex (line 226-228)
	if host, ok := hostFromBroker("kafka:broker:9092"); !ok || host != "kafka:broker" {
		t.Fatalf("multi-colon: got %q,%v", host, ok)
	}
}

func TestValidateObjectStoreEndpoint_branches(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec").Child("endpoint")

	// empty → nil
	if errs := validateObjectStoreEndpoint("", path); len(errs) != 0 {
		t.Fatalf("empty: %v", errs)
	}

	// plain bucket syntax (no scheme) → url.Scheme=="" → nil (line 103-106)
	if errs := validateObjectStoreEndpoint("mybucket/prefix", path); len(errs) != 0 {
		t.Fatalf("plain bucket: %v", errs)
	}

	// s3:// → skipped (line 107-109)
	if errs := validateObjectStoreEndpoint("s3://mybucket/key", path); len(errs) != 0 {
		t.Fatalf("s3 scheme: %v", errs)
	}

	// gs:// → skipped
	if errs := validateObjectStoreEndpoint("gs://mybucket/key", path); len(errs) != 0 {
		t.Fatalf("gs scheme: %v", errs)
	}

	// http:// with real host → falls through to validateURLTarget
	if errs := validateObjectStoreEndpoint("http://minio.example.com", path); len(errs) != 0 {
		t.Fatalf("http endpoint: %v", errs)
	}
}

func TestParseGitSCPHost(t *testing.T) {
	t.Parallel()

	// no colon → returns "" (line 206)
	if got := parseGitSCPHost("nocolon"); got != "" {
		t.Fatalf("no colon: got %q, want empty", got)
	}

	// colon with slash before it → returns "" (condition false)
	if got := parseGitSCPHost("path/to:repo"); got != "" {
		t.Fatalf("slash before colon: got %q, want empty", got)
	}

	// SCP format: git@host:path → returns host
	if got := parseGitSCPHost("git@github.com:org/repo"); got != "github.com" {
		t.Fatalf("scp: got %q, want github.com", got)
	}

	// no @ → just hostname before colon
	if got := parseGitSCPHost("github.com:org/repo"); got != "github.com" {
		t.Fatalf("no-at scp: got %q, want github.com", got)
	}
}

func TestValidateHost_denyHostname(t *testing.T) {
	t.Parallel()

	path := field.NewPath("spec").Child("endpoint")

	// empty after normalization → nil (line 168-170)
	if errs := validateHost("   ", path, "   "); len(errs) != 0 {
		t.Fatalf("whitespace-only: %v", errs)
	}

	// public IP → passes validateHost → return nil (line 183)
	if errs := validateHost("8.8.8.8", path, "8.8.8.8"); len(errs) != 0 {
		t.Fatalf("public IP: %v", errs)
	}
}
