// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"errors"
	"fmt"
	"strings"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
)

// ErrCABothSources marks a spec.tls that sets both caBundle and caSecretRef.
var ErrCABothSources = sinktls.ErrCABothSources

// ErrCAKeyMissing marks a spec.tls.caSecretRef Secret that has none of the CA keys.
var ErrCAKeyMissing = errors.New("spec.tls.caSecretRef: the Secret has no CA key")

// caKeys are the Secret keys a CA bundle is read from, in order.
var caKeys = []string{"tls.crt", "ca.crt", "ca.pem"}

// CheckCASources rejects a spec.tls that names two CA sources. Callers run it before
// fetching any Secret, so a both-set spec never fails on a missing Secret first.
func CheckCASources(tlsSpec *kollectdevv1alpha1.TLSSpec) error {
	return sinktls.CheckCASources(tlsSpec)
}

// SelectCAKey returns the first non-empty CA key (tls.crt, ca.crt, ca.pem) of the
// data of the caSecretRef Secret secretName. A Secret with none of them is a terminal
// error naming the Secret and the keys, never the values. The operator and the CLI
// fetch the Secret their own way and both call this.
func SelectCAKey(secretName string, data map[string][]byte) ([]byte, error) {
	for _, key := range caKeys {
		if v := data[key]; len(v) > 0 {
			return v, nil
		}
	}

	return nil, kollecterrors.Terminal(fmt.Errorf("%w: Secret %q has none of the keys %s",
		ErrCAKeyMissing, secretName, strings.Join(caKeys, ", ")))
}
