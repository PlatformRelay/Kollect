// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"strings"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// allowInsecureSinksTestMu serializes tests that mutate the process-global K-14
// opt-in, mirroring allowPrivateSinksTestMu. Tests using it must not run under
// t.Parallel.
var allowInsecureSinksTestMu sync.Mutex

func withAllowInsecureSinks(t *testing.T, allow bool) {
	t.Helper()

	allowInsecureSinksTestMu.Lock()
	SetAllowInsecureSinks(allow)
	t.Cleanup(func() {
		SetAllowInsecureSinks(false)
		allowInsecureSinksTestMu.Unlock()
	})
}

func tlsPath() *field.Path { return field.NewPath("spec").Child("tls") }

func TestValidateInsecureSkipVerify_deniedByDefault(t *testing.T) {
	withAllowInsecureSinks(t, false)

	errs := ValidateInsecureSkipVerify(&kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true}, tlsPath())
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one forbidden error", errs)
	}
	if !strings.Contains(errs[0].Detail, "--allow-insecure-sinks") {
		t.Fatalf("error detail should name the opt-in flag: %q", errs[0].Detail)
	}
}

func TestValidateInsecureSkipVerify_allowedWithOptIn(t *testing.T) {
	withAllowInsecureSinks(t, true)

	if errs := ValidateInsecureSkipVerify(&kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true}, tlsPath()); len(errs) != 0 {
		t.Fatalf("errs = %v, want none with opt-in", errs)
	}
}

func TestValidateInsecureSkipVerify_secureSpecsAlwaysPass(t *testing.T) {
	withAllowInsecureSinks(t, false)

	if errs := ValidateInsecureSkipVerify(nil, tlsPath()); len(errs) != 0 {
		t.Fatalf("nil tls: errs = %v", errs)
	}
	if errs := ValidateInsecureSkipVerify(&kollectdevv1alpha1.TLSSpec{}, tlsPath()); len(errs) != 0 {
		t.Fatalf("empty tls: errs = %v", errs)
	}
}

// TestValidateEventSinkSpec_rejectsInsecureWithoutOptIn pins the wiring into the
// family-sink admission path.
func TestValidateEventSinkSpec_rejectsInsecureWithoutOptIn(t *testing.T) {
	withAllowInsecureSinks(t, false)

	errs := ValidateEventSinkSpec(&kollectdevv1alpha1.KollectEventSinkSpec{
		Type: kollectdevv1alpha1.EventSinkTypeKafka,
		SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
			TLS: &kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true},
		},
		Kafka: &kollectdevv1alpha1.KafkaSpec{Brokers: []string{"broker:9092"}, Topic: "inventory"},
	})
	if len(errs) == 0 {
		t.Fatal("expected admission to reject insecureSkipVerify without the opt-in")
	}
}
