// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestConfigFromSpec(t *testing.T) {
	t.Parallel()

	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: "nats"}, nil)
	if err == nil {
		t.Fatal("expected error without nats spec")
	}

	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:     "nats",
		Endpoint: "nats://broker:4222",
		Cluster:  "prod-a",
		Nats: &kollectdevv1alpha1.NatsSpec{
			Subject: "inventory.events",
			Stream:  "team.inventory",
		},
	}, map[string][]byte{"token": []byte("secret-token")})
	if err != nil {
		t.Fatalf("ConfigFromSpec: %v", err)
	}

	if cfg.URL != "nats://broker:4222" {
		t.Fatalf("url = %q, want nats://broker:4222", cfg.URL)
	}

	if cfg.Subject != "inventory.events" {
		t.Fatalf("subject = %q, want inventory.events", cfg.Subject)
	}

	if cfg.Stream != "team_inventory" {
		t.Fatalf("stream = %q, want team_inventory", cfg.Stream)
	}

	if cfg.Token != "secret-token" {
		t.Fatalf("token not resolved")
	}
}

func TestStreamSubjects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		subject string
		want    []string
	}{
		{subject: "inventory.events", want: []string{"inventory.events"}},
		{subject: "inventory.>", want: []string{"inventory.>"}},
		{subject: "events", want: []string{"events"}},
	}

	for _, tc := range cases {
		got := streamSubjects(tc.subject)
		if len(got) != len(tc.want) {
			t.Fatalf("streamSubjects(%q) = %v, want %v", tc.subject, got, tc.want)
		}

		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Fatalf("streamSubjects(%q)[%d] = %q, want %q", tc.subject, i, got[i], tc.want[i])
			}
		}
	}
}
