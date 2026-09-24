// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"reflect"
	"testing"
)

func TestScrubber_redactsSensitiveKeys(t *testing.T) {
	t.Parallel()

	scrubber := NewScrubber(nil)

	tests := []struct {
		name string
		in   map[string]any
		want map[string]any
	}{
		{
			name: "top-level password",
			in: map[string]any{
				"password": "hunter2",
				"replicas": float64(3),
			},
			want: map[string]any{
				"password": redactedValue(),
				"replicas": float64(3),
			},
		},
		{
			name: "nested values map",
			in: map[string]any{
				"database": map[string]any{
					"host":     "postgres",
					"password": "secret",
				},
				"image": map[string]any{
					"tag": "1.2.3",
				},
			},
			want: map[string]any{
				"database": map[string]any{
					"host":     "postgres",
					"password": redactedValue(),
				},
				"image": map[string]any{
					"tag": "1.2.3",
				},
			},
		},
		{
			name: "case-insensitive token",
			in: map[string]any{
				"API_TOKEN": "abc",
			},
			want: map[string]any{
				"API_TOKEN": redactedValue(),
			},
		},
		{
			name: "valueFrom secretKeyRef carrier",
			in: map[string]any{
				"env": map[string]any{
					"valueFrom": map[string]any{
						"secretKeyRef": map[string]any{
							"name": "db-secret",
							"key":  "password",
						},
					},
				},
			},
			want: map[string]any{
				"env": map[string]any{
					"valueFrom": redactedValue(),
				},
			},
		},
		{
			name: "secretRef carrier",
			in: map[string]any{
				"envFrom": []any{
					map[string]any{
						"secretRef": map[string]any{
							"name": "app-secret",
						},
					},
				},
			},
			want: map[string]any{
				"envFrom": []any{
					map[string]any{
						"secretRef": redactedValue(),
					},
				},
			},
		},
		{
			name: "tls key material",
			in: map[string]any{
				"tls": map[string]any{
					"key": "-----BEGIN PRIVATE KEY-----",
					"crt": "-----BEGIN CERTIFICATE-----",
				},
			},
			want: map[string]any{
				"tls": redactedValue(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := scrubber.Scrub(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Scrub() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestScrubber_customKeys(t *testing.T) {
	t.Parallel()

	scrubber := NewScrubber([]string{"internalId"})

	in := map[string]any{
		"internalId": "42",
		"public":     "ok",
	}
	want := map[string]any{
		"internalId": redactedValue(),
		"public":     "ok",
	}

	got := scrubber.Scrub(in)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scrub() = %#v, want %#v", got, want)
	}
}

func TestScrubber_ScrubAttributes(t *testing.T) {
	t.Parallel()

	scrubber := NewScrubber(nil)
	attrs := map[string]any{
		"values": map[string]any{
			"token": "abc",
		},
		"chartVersion": "1.0.0",
	}

	got := scrubber.ScrubAttributes(attrs)
	want := map[string]any{
		"values": map[string]any{
			"token": redactedValue(),
		},
		"chartVersion": "1.0.0",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ScrubAttributes() = %#v, want %#v", got, want)
	}
}

func TestScrubber_bearerCarriers(t *testing.T) {
	t.Parallel()

	scrubber := NewScrubber(nil)

	tests := []struct {
		name string
		in   map[string]any
		want map[string]any
	}{
		{
			name: "authorization bearer header",
			in:   map[string]any{"authorization": "Bearer eyJhbGciOi", "name": "svc"},
			want: map[string]any{"authorization": redactedValue(), "name": "svc"},
		},
		{
			name: "X-Authorization suffix",
			in:   map[string]any{"X-Authorization": "Bearer abc"},
			want: map[string]any{"X-Authorization": redactedValue()},
		},
		{
			name: "proxy-authorization",
			in:   map[string]any{"proxy-authorization": "Basic dXNlcjpwdw=="},
			want: map[string]any{"proxy-authorization": redactedValue()},
		},
		{
			name: "authorizationHeader prefix stem",
			in:   map[string]any{"authorizationHeader": "Bearer abc"},
			want: map[string]any{"authorizationHeader": redactedValue()},
		},
		{
			name: "bearerToken camel and bearer_token snake",
			in: map[string]any{
				"bearerToken":  "abc",
				"bearer_token": "abc",
			},
			want: map[string]any{
				"bearerToken":  redactedValue(),
				"bearer_token": redactedValue(),
			},
		},
		{
			name: "benign keys survive",
			in: map[string]any{
				"authorizer": "rbac",
				"authority":  "cluster.local",
				"replicas":   float64(2),
			},
			want: map[string]any{
				"authorizer": "rbac",
				"authority":  "cluster.local",
				"replicas":   float64(2),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := scrubber.Scrub(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Scrub() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
