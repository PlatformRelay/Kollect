// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kafkasink "github.com/platformrelay/kollect/internal/sink/kafka"
	natssink "github.com/platformrelay/kollect/internal/sink/nats"
)

// secretReadTimeout bounds each Secret read on the pool's acquire path.
const secretReadTimeout = 10 * time.Second

// secretUse is one Secret a backend build reads for a sink spec, with its effective namespace.
type secretUse struct {
	Namespace string
	Name      string
}

// secretVersion is the metadata identity of one referenced Secret. It carries no data: the
// pool key is derived from it alone (SCR-4).
type secretVersion struct {
	Namespace       string
	Name            string
	UID             types.UID
	ResourceVersion string
}

// referencedSecretRefs lists, in a fixed order, every Secret BuildContextFromSpec reads for spec,
// each with its effective namespace (ref.Namespace, else defaultNamespace, else
// DefaultSecretNamespace). References with an empty name are skipped, as ResolveSecret skips
// them, and a Secret two references name (Kafka and NATS fall back to spec.secretRef) is listed
// once. Every *SecretReference field of KollectSinkSpec must appear here
// (TestReferencedSecretRefs_coversEverySecretReferenceField).
func referencedSecretRefs(spec kollectdevv1alpha1.KollectSinkSpec, defaultNamespace string) []secretUse {
	if defaultNamespace == "" {
		defaultNamespace = DefaultSecretNamespace
	}

	refs := []*kollectdevv1alpha1.SecretReference{spec.SecretRef}
	switch spec.Type {
	case "postgres":
		if spec.Postgres != nil {
			refs = append(refs, spec.Postgres.DatabaseRef)
		}
	case "bigquery":
		if spec.BigQuery != nil {
			refs = append(refs, spec.BigQuery.SecretRef)
		}
	case "mongodb":
		if spec.MongoDB != nil {
			refs = append(refs, spec.MongoDB.DatabaseRef)
		}
	case kafkasink.TypeName:
		refs = append(refs, kafkaSecretRef(spec))
	case natssink.TypeName:
		refs = append(refs, natsSecretRef(spec))
	}
	refs = append(refs, gitAuthSecretRef(spec))
	if spec.TLS != nil && len(spec.TLS.CABundle) == 0 {
		refs = append(refs, spec.TLS.CASecretRef)
	}

	uses := make([]secretUse, 0, len(refs))
	for _, ref := range refs {
		if ref == nil || ref.Name == "" {
			continue
		}
		use := secretUse{Namespace: secretRefNamespace(ref, defaultNamespace), Name: ref.Name}
		if !containsSecretUse(uses, use) {
			uses = append(uses, use)
		}
	}

	return uses
}

func containsSecretUse(uses []secretUse, use secretUse) bool {
	for _, u := range uses {
		if u == use {
			return true
		}
	}

	return false
}

func secretRefNamespace(ref *kollectdevv1alpha1.SecretReference, defaultNamespace string) string {
	if ref.Namespace != "" {
		return ref.Namespace
	}

	return defaultNamespace
}

// kafkaSecretRef is kafka.secretRef, falling back to spec.secretRef.
func kafkaSecretRef(spec kollectdevv1alpha1.KollectSinkSpec) *kollectdevv1alpha1.SecretReference {
	if spec.Type != "kafka" || spec.Kafka == nil {
		return nil
	}
	if spec.Kafka.SecretRef != nil {
		return spec.Kafka.SecretRef
	}

	return spec.SecretRef
}

// natsSecretRef is nats.secretRef, falling back to spec.secretRef.
func natsSecretRef(spec kollectdevv1alpha1.KollectSinkSpec) *kollectdevv1alpha1.SecretReference {
	if spec.Type != "nats" || spec.Nats == nil {
		return nil
	}
	if spec.Nats.SecretRef != nil {
		return spec.Nats.SecretRef
	}

	return spec.SecretRef
}

// secretMeta reads the metadata of each used Secret, uncached when reader is the API reader.
// Only namespace, name, UID and resourceVersion leave this function; the response (which still
// carries annotations) is dropped.
func secretMeta(ctx context.Context, reader client.Reader, uses []secretUse) ([]secretVersion, error) {
	out := make([]secretVersion, 0, len(uses))
	for _, use := range uses {
		meta := &metav1.PartialObjectMetadata{}
		meta.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
		if err := getSecretObject(ctx, reader, use, meta); err != nil {
			return nil, err
		}
		out = append(out, secretVersion{
			Namespace:       use.Namespace,
			Name:            use.Name,
			UID:             meta.UID,
			ResourceVersion: meta.ResourceVersion,
		})
	}

	return out, nil
}

// fetchSecrets reads each used Secret in full, for a rebuild; an empty Secret is an error.
func fetchSecrets(ctx context.Context, reader client.Reader, uses []secretUse) (map[secretUse]*corev1.Secret, error) {
	out := make(map[secretUse]*corev1.Secret, len(uses))
	for _, use := range uses {
		secret := &corev1.Secret{}
		if err := getSecretObject(ctx, reader, use, secret); err != nil {
			return nil, err
		}
		if SecretDataEmpty(secret.Data) {
			return nil, &secretReadError{use: use, empty: true, err: EmptySecretError(use.Namespace, use.Name)}
		}
		out[use] = secret
	}

	return out, nil
}

// versionsOf returns the metadata identity of fetched Secrets in uses order.
func versionsOf(uses []secretUse, secrets map[secretUse]*corev1.Secret) []secretVersion {
	out := make([]secretVersion, 0, len(uses))
	for _, use := range uses {
		s := secrets[use]
		out = append(out, secretVersion{
			Namespace: use.Namespace, Name: use.Name, UID: s.UID, ResourceVersion: s.ResourceVersion,
		})
	}

	return out
}

// getSecretObject GETs one Secret (full or metadata) with a bounded timeout. A missing Secret
// keeps ResolveSecret's error text.
func getSecretObject(ctx context.Context, reader client.Reader, use secretUse, obj client.Object) error {
	readCtx, cancel := context.WithTimeout(ctx, secretReadTimeout)
	defer cancel()

	if err := reader.Get(readCtx, types.NamespacedName{Namespace: use.Namespace, Name: use.Name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return &secretReadError{
				use: use, notFound: true,
				err: fmt.Errorf("secret %q not found in namespace %q", use.Name, use.Namespace),
			}
		}

		return &secretReadError{use: use, err: fmt.Errorf("read secret %q in namespace %q: %w", use.Name, use.Namespace, err)}
	}

	return nil
}

// secretReadError records which Secret a pool-path read failed on; its text is the inner error's.
type secretReadError struct {
	use secretUse
	// notFound and empty mark a missing or empty Secret: the read failures that retire the pooled
	// backend (any other read failure observed no change).
	notFound bool
	empty    bool
	err      error
}

func (e *secretReadError) Error() string { return e.err.Error() }

func (e *secretReadError) Unwrap() error { return e.err }

// isSecretGone reports whether err is a pool-path read of a missing or empty Secret.
func isSecretGone(err error) bool {
	var failed *secretReadError

	return errors.As(err, &failed) && (failed.notFound || failed.empty)
}

// poolEntryKey derives the pool key from the spec hash and the referenced Secrets' metadata.
func poolEntryKey(specHash string, versions []secretVersion) string {
	h := sha256.New()
	h.Write([]byte(specHash))
	for _, v := range versions {
		for _, part := range []string{v.Namespace, v.Name, string(v.UID), v.ResourceVersion} {
			h.Write([]byte{0})
			h.Write([]byte(part))
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

// changedSecrets names the Secrets whose UID or resourceVersion differs between two snapshots
// of the same spec ("namespace/name").
func changedSecrets(before, after []secretVersion) []string {
	prev := make(map[secretUse]secretVersion, len(before))
	for _, v := range before {
		prev[secretUse{Namespace: v.Namespace, Name: v.Name}] = v
	}

	var out []string
	for _, v := range after {
		if old, ok := prev[secretUse{Namespace: v.Namespace, Name: v.Name}]; !ok || old != v {
			out = append(out, v.Namespace+"/"+v.Name)
		}
	}

	return out
}
