// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/operator"
	"github.com/platformrelay/kollect/internal/sink/git"
)

// DefaultSecretNamespace is retained for callers outside operator/webhook packages.
const DefaultSecretNamespace = operator.DefaultSecretNamespace

// BuildContext carries resolved material for backend construction.
type BuildContext struct {
	Ctx                context.Context
	CAPEM              []byte
	SecretData         map[string][]byte
	DatabaseSecretData map[string][]byte
}

// BuildContextFromSpec resolves secrets and TLS material for a KollectSink spec. It reads
// exactly the Secrets referencedSecretRefs lists, through c, so the build and the pool key cannot
// read different Secrets.
func BuildContextFromSpec(
	ctx context.Context,
	c client.Reader,
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
) (BuildContext, error) {
	if defaultNamespace == "" {
		defaultNamespace = DefaultSecretNamespace
	}

	// Before any Secret read, so a both-set spec never fails on a missing Secret first.
	if err := CheckCASources(spec.TLS); err != nil {
		return BuildContext{}, err
	}

	read := func(use secretUse) (*corev1.Secret, error) { return getSecret(ctx, c, use) }

	uses := referencedSecretRefs(spec, defaultNamespace)
	secrets := make(map[secretUse]*corev1.Secret, len(uses))

	// The CA rules come before any credential lookup: a CA Secret that is missing or has no CA
	// key fails first, whatever the credential Secrets look like.
	caUse, caSecret, err := checkCASecret(spec, defaultNamespace, read)
	if err != nil {
		return BuildContext{}, err
	}
	if caSecret != nil {
		secrets[caUse] = caSecret
	}

	for _, use := range uses {
		if _, done := secrets[use]; done {
			continue
		}
		secret, err := read(use)
		if err != nil {
			return BuildContext{}, err
		}
		secrets[use] = secret
	}

	return buildContextFromSecrets(ctx, spec, defaultNamespace, secrets)
}

// buildContextFromSecrets assembles the BuildContext from Secrets already read (keyed by
// referencedSecretRefs' uses), so the pool's rebuild hands over the objects its key came from.
func buildContextFromSecrets(
	ctx context.Context,
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
	secrets map[secretUse]*corev1.Secret,
) (BuildContext, error) {
	if defaultNamespace == "" {
		defaultNamespace = DefaultSecretNamespace
	}

	data := func(ref *kollectdevv1alpha1.SecretReference) (map[string][]byte, error) {
		if ref == nil || ref.Name == "" {
			return nil, nil
		}
		use := secretUse{Namespace: secretRefNamespace(ref, defaultNamespace), Name: ref.Name}
		secret, ok := secrets[use]
		if !ok {
			return nil, fmt.Errorf("secret %q in namespace %q was not read for the build", use.Name, use.Namespace)
		}

		return secret.Data, nil
	}

	caPEM, err := resolveCAPEM(spec.TLS, data)
	if err != nil {
		return BuildContext{}, err
	}

	out := BuildContext{Ctx: ctx, CAPEM: caPEM}

	if out.SecretData, err = data(spec.SecretRef); err != nil {
		return BuildContext{}, err
	}
	if out.DatabaseSecretData, err = data(databaseSecretRef(spec)); err != nil {
		return BuildContext{}, err
	}

	// Kafka, git and NATS credentials replace spec.secretRef's data when they apply.
	for _, override := range []*kollectdevv1alpha1.SecretReference{
		kafkaSecretRef(spec), gitAuthSecretRef(spec), natsSecretRef(spec),
	} {
		if override == nil {
			continue
		}
		if out.SecretData, err = data(override); err != nil {
			return BuildContext{}, err
		}
	}

	return out, nil
}

// checkCASecret applies the CA rules to the tls.caSecretRef Secret (none when caBundle is set):
// it reads the Secret through read and fails when it is missing or has no CA key. It returns the
// Secret it read so the caller need not read it again.
func checkCASecret(
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
	read func(secretUse) (*corev1.Secret, error),
) (secretUse, *corev1.Secret, error) {
	use, ok := caSecretUse(spec, defaultNamespace)
	if !ok {
		return secretUse{}, nil, nil
	}
	if use.Name == "" {
		// A caSecretRef without a name has no CA key; this fails before any credential read,
		// as resolveCAPEM always did.
		_, err := SelectCAKey("", nil)

		return use, nil, err
	}

	secret, err := read(use)
	if err != nil {
		return use, nil, err
	}
	if _, err := SelectCAKey(use.Name, secret.Data); err != nil {
		return use, nil, err
	}

	return use, secret, nil
}

// caSecretUse is the tls.caSecretRef Secret the CA rules read; ok is false when the spec has no
// caSecretRef or a caBundle replaces it.
func caSecretUse(spec kollectdevv1alpha1.KollectSinkSpec, defaultNamespace string) (secretUse, bool) {
	tlsSpec := spec.TLS
	if tlsSpec == nil || len(tlsSpec.CABundle) > 0 || tlsSpec.CASecretRef == nil {
		return secretUse{}, false
	}

	return secretUse{Namespace: secretRefNamespace(tlsSpec.CASecretRef, defaultNamespace), Name: tlsSpec.CASecretRef.Name}, true
}

// databaseSecretRef is the database credential reference of a postgres, bigquery or mongodb sink.
func databaseSecretRef(spec kollectdevv1alpha1.KollectSinkSpec) *kollectdevv1alpha1.SecretReference {
	switch spec.Type {
	case "postgres":
		if spec.Postgres != nil {
			return spec.Postgres.DatabaseRef
		}
	case "bigquery":
		if spec.BigQuery != nil {
			return spec.BigQuery.SecretRef
		}
	case "mongodb":
		if spec.MongoDB != nil {
			return spec.MongoDB.DatabaseRef
		}
	}

	return nil
}

func gitAuthSecretRef(spec kollectdevv1alpha1.KollectSinkSpec) *kollectdevv1alpha1.SecretReference {
	if spec.Type != git.TypeName || spec.Git == nil || spec.Git.Auth == nil {
		return nil
	}

	return spec.Git.Auth.SecretRef
}

func resolveCAPEM(
	tlsSpec *kollectdevv1alpha1.TLSSpec,
	data func(*kollectdevv1alpha1.SecretReference) (map[string][]byte, error),
) ([]byte, error) {
	if err := CheckCASources(tlsSpec); err != nil {
		return nil, err
	}

	if tlsSpec == nil {
		return nil, nil
	}

	if len(tlsSpec.CABundle) > 0 {
		return tlsSpec.CABundle, nil
	}

	if tlsSpec.CASecretRef == nil {
		return nil, nil
	}

	caData, err := data(tlsSpec.CASecretRef)
	if err != nil {
		return nil, err
	}

	return SelectCAKey(tlsSpec.CASecretRef.Name, caData)
}
