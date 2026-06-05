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

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
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

	ns := ref.Namespace
	if ns == "" {
		ns = defaultNamespace
	}

	var secret corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return Credentials{}, fmt.Errorf("secret %q not found in namespace %q", ref.Name, ns)
		}

		return Credentials{}, err
	}

	creds := Credentials{Data: secret.Data}
	if v, ok := secret.Data["username"]; ok {
		creds.Username = string(v)
	}

	for _, key := range []string{"password", "token"} {
		if v, ok := secret.Data[key]; ok {
			if key == "token" {
				creds.Token = string(v)
			} else {
				creds.Password = string(v)
			}
		}
	}

	if creds.Token == "" && creds.Password != "" {
		creds.Token = creds.Password
	}

	return creds, nil
}
