// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/operator"
	"github.com/konih/kollect/internal/sink/git"
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

// BuildContextFromSpec resolves secrets and TLS material for a KollectSink spec.
func BuildContextFromSpec(
	ctx context.Context,
	c client.Reader,
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
) (BuildContext, error) {
	if defaultNamespace == "" {
		defaultNamespace = DefaultSecretNamespace
	}

	caPEM, err := resolveCAPEM(ctx, c, spec.TLS, defaultNamespace)
	if err != nil {
		return BuildContext{}, err
	}

	out := BuildContext{Ctx: ctx, CAPEM: caPEM}

	if err := populateDefaultSecretData(ctx, c, spec, defaultNamespace, &out); err != nil {
		return BuildContext{}, err
	}
	if err := populateDatabaseSecretData(ctx, c, spec, defaultNamespace, &out); err != nil {
		return BuildContext{}, err
	}
	if err := overrideKafkaSecretData(ctx, c, spec, defaultNamespace, &out); err != nil {
		return BuildContext{}, err
	}
	if err := overrideGitSecretData(ctx, c, spec, defaultNamespace, &out); err != nil {
		return BuildContext{}, err
	}
	if err := overrideNatsSecretData(ctx, c, spec, defaultNamespace, &out); err != nil {
		return BuildContext{}, err
	}

	return out, nil
}

func populateDefaultSecretData(
	ctx context.Context,
	c client.Reader,
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
	out *BuildContext,
) error {
	creds, err := ResolveSecret(ctx, c, spec.SecretRef, defaultNamespace)
	if err != nil {
		return err
	}

	out.SecretData = creds.Data

	return nil
}

func populateDatabaseSecretData(
	ctx context.Context,
	c client.Reader,
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
	out *BuildContext,
) error {
	switch spec.Type {
	case "postgres":
		if spec.Postgres == nil {
			return nil
		}

		dbCreds, err := ResolveSecret(ctx, c, spec.Postgres.DatabaseRef, defaultNamespace)
		if err != nil {
			return err
		}

		out.DatabaseSecretData = dbCreds.Data
	case "bigquery":
		if spec.BigQuery == nil || spec.BigQuery.SecretRef == nil {
			return nil
		}

		bqCreds, err := ResolveSecret(ctx, c, spec.BigQuery.SecretRef, defaultNamespace)
		if err != nil {
			return err
		}

		out.DatabaseSecretData = bqCreds.Data
	}

	return nil
}

func overrideKafkaSecretData(
	ctx context.Context,
	c client.Reader,
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
	out *BuildContext,
) error {
	if spec.Type != "kafka" || spec.Kafka == nil {
		return nil
	}

	kafkaRef := spec.Kafka.SecretRef
	if kafkaRef == nil {
		kafkaRef = spec.SecretRef
	}
	if kafkaRef == nil {
		return nil
	}

	kafkaCreds, err := ResolveSecret(ctx, c, kafkaRef, defaultNamespace)
	if err != nil {
		return err
	}

	out.SecretData = kafkaCreds.Data

	return nil
}

func overrideGitSecretData(
	ctx context.Context,
	c client.Reader,
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
	out *BuildContext,
) error {
	gitSecret := gitAuthSecretRef(spec)
	if gitSecret == nil {
		return nil
	}

	gitCreds, err := ResolveSecret(ctx, c, gitSecret, defaultNamespace)
	if err != nil {
		return err
	}

	out.SecretData = gitCreds.Data

	return nil
}

func overrideNatsSecretData(
	ctx context.Context,
	c client.Reader,
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
	out *BuildContext,
) error {
	if spec.Type != "nats" || spec.Nats == nil {
		return nil
	}

	natsRef := spec.Nats.SecretRef
	if natsRef == nil {
		natsRef = spec.SecretRef
	}
	if natsRef == nil {
		return nil
	}

	natsCreds, err := ResolveSecret(ctx, c, natsRef, defaultNamespace)
	if err != nil {
		return err
	}

	out.SecretData = natsCreds.Data

	return nil
}

func gitAuthSecretRef(spec kollectdevv1alpha1.KollectSinkSpec) *kollectdevv1alpha1.SecretReference {
	if spec.Type != git.TypeName || spec.Git == nil || spec.Git.Auth == nil {
		return nil
	}

	return spec.Git.Auth.SecretRef
}

func resolveCAPEM(
	ctx context.Context,
	c client.Reader,
	tlsSpec *kollectdevv1alpha1.TLSSpec,
	defaultNamespace string,
) ([]byte, error) {
	if tlsSpec == nil {
		return nil, nil
	}

	if len(tlsSpec.CABundle) > 0 {
		return tlsSpec.CABundle, nil
	}

	if tlsSpec.CASecretRef == nil {
		return nil, nil
	}

	creds, err := ResolveSecret(ctx, c, tlsSpec.CASecretRef, defaultNamespace)
	if err != nil {
		return nil, err
	}

	for _, key := range []string{"tls.crt", "ca.crt", "ca.pem"} {
		if v, ok := creds.Data[key]; ok && len(v) > 0 {
			return v, nil
		}
	}

	return nil, nil
}
