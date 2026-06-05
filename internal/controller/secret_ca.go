// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var caSecretKeys = []string{"tls.crt", "ca.crt", "ca-bundle.crt"}

func resolveCAPEM(ctx context.Context, c client.Reader, tlsSpec *kollectdevv1alpha1.TLSSpec) ([]byte, error) {
	if tlsSpec == nil {
		return nil, nil
	}

	if len(tlsSpec.CABundle) > 0 {
		return tlsSpec.CABundle, nil
	}

	if tlsSpec.CASecretRef == nil {
		return nil, nil
	}

	ref := tlsSpec.CASecretRef
	ns := ref.Namespace
	if ns == "" {
		ns = defaultSecretNamespace
	}

	var secret corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("CA secret %q not found in namespace %q", ref.Name, ns)
		}

		return nil, err
	}

	for _, key := range caSecretKeys {
		if pem, ok := secret.Data[key]; ok && len(pem) > 0 {
			return pem, nil
		}
	}

	return nil, fmt.Errorf(
		"CA secret %q has no PEM data (expected one of: %v)",
		ref.Name,
		caSecretKeys,
	)
}
