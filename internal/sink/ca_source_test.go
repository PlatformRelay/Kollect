// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// R1: the operator's CA resolution goes through the shared rules: one source only,
// checked before any fetch, and a Secret without a CA key is an error, never "no CA".
func TestResolveCAPEM_sharedRules(t *testing.T) {
	t.Parallel()

	const ns = "team-a"

	secret := func(name string, data map[string]string) *corev1.Secret {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}, Data: map[string][]byte{}}
		for k, v := range data {
			s.Data[k] = []byte(v)
		}

		return s
	}

	cl := fake.NewClientBuilder().WithScheme(clientgoscheme.Scheme).WithObjects(
		secret("ca-crt", map[string]string{"ca.crt": "from-ca-crt"}),
		secret("tls-and-ca", map[string]string{"tls.crt": "from-tls-crt", "ca.crt": "from-ca-crt"}),
		secret("ca-pem", map[string]string{"ca.pem": "from-ca-pem"}),
		secret("other-only", map[string]string{"other": "sentinel-value"}),
	).Build()

	ref := func(name string) *kollectdevv1alpha1.SecretReference {
		return &kollectdevv1alpha1.SecretReference{Name: name}
	}

	cases := []struct {
		name    string
		tls     *kollectdevv1alpha1.TLSSpec
		want    string
		wantErr error
		wantSub []string
	}{
		{name: "no tls"},
		{name: "tls without a CA", tls: &kollectdevv1alpha1.TLSSpec{}},
		{name: "caBundle", tls: &kollectdevv1alpha1.TLSSpec{CABundle: []byte("inline")}, want: "inline"},
		{name: "Secret ca.crt", tls: &kollectdevv1alpha1.TLSSpec{CASecretRef: ref("ca-crt")}, want: "from-ca-crt"},
		{name: "Secret tls.crt wins", tls: &kollectdevv1alpha1.TLSSpec{CASecretRef: ref("tls-and-ca")}, want: "from-tls-crt"},
		{name: "Secret ca.pem", tls: &kollectdevv1alpha1.TLSSpec{CASecretRef: ref("ca-pem")}, want: "from-ca-pem"},
		{
			name:    "Secret without a CA key",
			tls:     &kollectdevv1alpha1.TLSSpec{CASecretRef: ref("other-only")},
			wantErr: ErrCAKeyMissing,
			wantSub: []string{"other-only", "tls.crt", "ca.crt", "ca.pem"},
		},
		{
			name:    "both sources",
			tls:     &kollectdevv1alpha1.TLSSpec{CABundle: []byte("inline"), CASecretRef: ref("ca-crt")},
			wantErr: ErrCABothSources,
			wantSub: []string{"caBundle", "caSecretRef"},
		},
		{
			name:    "both sources, Secret absent",
			tls:     &kollectdevv1alpha1.TLSSpec{CABundle: []byte("inline"), CASecretRef: ref("absent")},
			wantErr: ErrCABothSources,
			wantSub: []string{"caBundle", "caSecretRef"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveCAPEM(tc.tls, func(r *kollectdevv1alpha1.SecretReference) (map[string][]byte, error) {
				creds, rerr := ResolveSecret(t.Context(), cl, r, ns)

				return creds.Data, rerr
			})

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("resolveCAPEM: %v", err)
				}

				if !bytes.Equal(got, []byte(tc.want)) {
					t.Fatalf("CA = %q, want %q", got, tc.want)
				}

				return
			}

			if err == nil {
				t.Fatalf("resolveCAPEM = %q, nil; want %v", got, tc.wantErr)
			}

			if !errors.Is(err, tc.wantErr) {
				t.Errorf("error %q is not %v", err, tc.wantErr)
			}

			if !kollecterrors.IsTerminal(err) {
				t.Errorf("error %q is not terminal", err)
			}

			for _, sub := range tc.wantSub {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("error %q does not name %q", err, sub)
				}
			}

			for _, value := range []string{"sentinel-value", "inline", "from-ca-crt"} {
				if strings.Contains(err.Error(), value) {
					t.Errorf("error %q echoes a value", err)
				}
			}
		})
	}
}
