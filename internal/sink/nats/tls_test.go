// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestTLSConfigFromSpec_insecureSkip(t *testing.T) {
	t.Parallel()

	cfg, err := TLSConfigFromSpec(&kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true}, nil)
	if err != nil {
		t.Fatalf("TLSConfigFromSpec: %v", err)
	}

	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify true")
	}

	tlsCfg, err := cfg.ClientConfig()
	if err != nil {
		t.Fatalf("ClientConfig: %v", err)
	}
	if tlsCfg == nil || !tlsCfg.InsecureSkipVerify {
		t.Error("client config should inherit insecure skip verify")
	}
}

func TestTLSConfigFromSpec_offByDefault(t *testing.T) {
	t.Parallel()

	cfg, err := TLSConfigFromSpec(nil, nil)
	if err != nil {
		t.Fatalf("TLSConfigFromSpec: %v", err)
	}
	if cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify false by default")
	}
	if cfg.Enabled() {
		t.Error("expected TLS disabled when no CA and no insecure skip")
	}
}
