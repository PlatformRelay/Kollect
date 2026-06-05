// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestResolveCAPEM(t *testing.T) {
	t.Parallel()

	pem, err := resolveCAPEM(t.Context(), nil, nil)
	if err != nil || pem != nil {
		t.Fatalf("nil tls: pem=%q err=%v", pem, err)
	}

	inline := []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----")
	pem, err = resolveCAPEM(t.Context(), nil, &kollectdevv1alpha1.TLSSpec{CABundle: inline})
	if err != nil || string(pem) != string(inline) {
		t.Fatalf("inline bundle: %v", err)
	}

	scheme := runtime.NewScheme()
	if schemeErr := corev1.AddToScheme(scheme); schemeErr != nil {
		t.Fatal(schemeErr)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: "team-a"},
		Data:       map[string][]byte{"ca.crt": inline},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	pem, err = resolveCAPEM(t.Context(), c, &kollectdevv1alpha1.TLSSpec{
		CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca", Namespace: "team-a"},
	})
	if err != nil || string(pem) != string(inline) {
		t.Fatalf("secret ref: %v pem=%q", err, pem)
	}

	_, err = resolveCAPEM(t.Context(), c, &kollectdevv1alpha1.TLSSpec{
		CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "missing"},
	})
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}
