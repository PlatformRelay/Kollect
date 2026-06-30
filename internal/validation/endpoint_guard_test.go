// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"testing"

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

	import_netip := func(s string) netip.Addr {
		a, _ := netip.ParseAddr(s)
		return a
	}

	if !isDeniedIP(import_netip("192.168.1.1")) {
		t.Error("private RFC1918 address must be denied")
	}
	if !isDeniedIP(import_netip("100.64.0.1")) {
		t.Error("carrier-grade NAT address must be denied via denyCIDRs")
	}
	if !isDeniedIP(import_netip("198.18.0.1")) {
		t.Error("benchmark-testing address must be denied via denyCIDRs")
	}
	if isDeniedIP(import_netip("8.8.8.8")) {
		t.Error("public address must not be denied")
	}
}
