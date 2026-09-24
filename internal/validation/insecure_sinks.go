// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"sync/atomic"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// allowInsecureSinks is the K-14 production opt-in: when true, admission (and
// the sink TLS-config constructors that read it) permit
// `spec.tls.insecureSkipVerify: true`. When false — the deny-by-default shipped
// posture — a sink that requests disabled certificate/host-key verification is
// rejected at admission and refused at construction time. Default false; set
// process-wide from the manager --allow-insecure-sinks flag via
// SetAllowInsecureSinks. Atomic so concurrent admission reads stay race-free
// (mirrors allowPrivateSinks). This is deliberately NOT a CRD field a tenant
// can set: one opt-in for the whole process, chosen by the cluster operator.
var allowInsecureSinks atomic.Bool

// SetAllowInsecureSinks configures whether the process permits sinks to disable
// TLS certificate / SSH host-key verification (K-14). Call once at manager
// startup from --allow-insecure-sinks, before any sink is validated or built.
func SetAllowInsecureSinks(allow bool) {
	allowInsecureSinks.Store(allow)
}

// AllowInsecureSinks reports whether the process-wide insecure-sink opt-in is
// enabled. Sink TLS-config constructors consult it so the gate holds even for
// kinds without an admission webhook (for example the legacy KollectSink).
func AllowInsecureSinks() bool {
	return allowInsecureSinks.Load()
}

// ValidateInsecureSkipVerify rejects a TLS spec that disables verification
// unless the process-wide opt-in is set. tls may be nil.
func ValidateInsecureSkipVerify(tls *kollectdevv1alpha1.TLSSpec, path *field.Path) field.ErrorList {
	if tls == nil || !tls.InsecureSkipVerify {
		return nil
	}
	if AllowInsecureSinks() {
		return nil
	}

	return field.ErrorList{field.Forbidden(
		path.Child("insecureSkipVerify"),
		"disabling TLS certificate/host-key verification requires the operator to start "+
			"the manager with --allow-insecure-sinks",
	)}
}
