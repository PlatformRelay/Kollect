// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const DefaultSecretNamespace = "kollect-system" //nolint:gosec // namespace name, not a credential

// BuildContext carries resolved material for backend construction.
type BuildContext struct {
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

	out := BuildContext{CAPEM: caPEM}

	creds, err := ResolveSecret(ctx, c, spec.SecretRef, defaultNamespace)
	if err != nil {
		return BuildContext{}, err
	}

	out.SecretData = creds.Data

	if spec.Type == "postgres" && spec.Postgres != nil {
		dbCreds, err := ResolveSecret(ctx, c, spec.Postgres.DatabaseRef, defaultNamespace)
		if err != nil {
			return BuildContext{}, err
		}

		out.DatabaseSecretData = dbCreds.Data
	}

	if spec.Type == "kafka" && spec.Kafka != nil {
		kafkaRef := spec.Kafka.SecretRef
		if kafkaRef == nil {
			kafkaRef = spec.SecretRef
		}

		if kafkaRef != nil {
			kafkaCreds, err := ResolveSecret(ctx, c, kafkaRef, defaultNamespace)
			if err != nil {
				return BuildContext{}, err
			}

			out.SecretData = kafkaCreds.Data
		}
	}

	if gitSecret := gitAuthSecretRef(spec); gitSecret != nil {
		gitCreds, err := ResolveSecret(ctx, c, gitSecret, defaultNamespace)
		if err != nil {
			return BuildContext{}, err
		}

		out.SecretData = gitCreds.Data
	}

	if spec.Type == "nats" && spec.Nats != nil {
		natsRef := spec.Nats.SecretRef
		if natsRef == nil {
			natsRef = spec.SecretRef
		}

		if natsRef != nil {
			natsCreds, err := ResolveSecret(ctx, c, natsRef, defaultNamespace)
			if err != nil {
				return BuildContext{}, err
			}

			out.SecretData = natsCreds.Data
		}
	}

	return out, nil
}

func gitAuthSecretRef(spec kollectdevv1alpha1.KollectSinkSpec) *kollectdevv1alpha1.SecretReference {
	if spec.Type != "git" || spec.Git == nil || spec.Git.Auth == nil {
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
