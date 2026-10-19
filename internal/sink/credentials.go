// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// Credentials holds key material resolved from a Kubernetes Secret.
type Credentials struct {
	Username string
	Password string
	Token    string
	Data     map[string][]byte
}

// ResolveSecret loads secretRef from the API server.
func ResolveSecret(
	ctx context.Context,
	c client.Reader,
	ref *kollectdevv1alpha1.SecretReference,
	defaultNamespace string,
) (Credentials, error) {
	if ref == nil || ref.Name == "" {
		return Credentials{}, nil
	}

	secret, err := getSecret(ctx, c, secretUse{Namespace: secretRefNamespace(ref, defaultNamespace), Name: ref.Name})
	if err != nil {
		return Credentials{}, err
	}

	creds := Credentials{Data: secret.Data}
	if v, ok := secret.Data["username"]; ok {
		creds.Username = string(v)
	}

	if v, ok := secret.Data[secretKeyPassword]; ok {
		creds.Password = string(v)
	}

	if v, ok := secret.Data[secretKeyToken]; ok {
		creds.Token = string(v)
	}

	if creds.Token == "" && creds.Password != "" {
		creds.Token = creds.Password
	}

	return creds, nil
}

// getSecret reads one Secret through c. A missing Secret is the plain, unclassified
// "not found" error, an empty one EmptySecretError; any other read error is returned as is.
func getSecret(ctx context.Context, c client.Reader, use secretUse) (*corev1.Secret, error) {
	var secret corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Name: use.Name, Namespace: use.Namespace}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("secret %q not found in namespace %q", use.Name, use.Namespace)
		}

		return nil, err
	}
	if SecretDataEmpty(secret.Data) {
		return nil, EmptySecretError(use.Namespace, use.Name)
	}

	return &secret, nil
}

// SecretDataEmpty reports whether data holds no non-empty value (nil, no keys, or only empty
// values). Such a Secret is refused rather than handed to a backend, which could otherwise fall
// back to ambient credentials (SCR-2).
func SecretDataEmpty(data map[string][]byte) bool {
	for _, v := range data {
		if len(v) > 0 {
			return false
		}
	}

	return true
}

// EmptySecretError is the shared, unclassified (retryable) error for a referenced Secret with no
// data; the operator and kollect-pipeline print the same text (SCR-5). An empty namespace (a
// config-dir manifest without one) drops the namespace clause.
func EmptySecretError(namespace, name string) error {
	if namespace == "" {
		return fmt.Errorf("secret %q has no data", name)
	}

	return fmt.Errorf("secret %q in namespace %q has no data", name, namespace)
}
