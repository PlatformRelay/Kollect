// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package validation

import (
	"fmt"
	"strings"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/util/validation/field"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// allowedSecretRefNamespaces is the K-04 production opt-in: the set of
// namespaces a family sink may reference from another namespace via a
// SecretReference. Empty by default (deny cross-namespace). Set process-wide from
// the manager --allow-secret-ref-namespaces flag via SetAllowedSecretRefNamespaces.
//
// This mirrors the allowPrivateSinks pattern (endpoint_guard.go): a
// cluster-admin-only process flag, never a CRD/tenant-controllable field, so a
// tenant sink author can never widen it. Stored behind an atomic pointer so
// concurrent admission reads stay race-free.
var allowedSecretRefNamespaces atomic.Pointer[secretNamespaceSet]

type secretNamespaceSet struct {
	namespaces map[string]struct{}
}

func init() {
	allowedSecretRefNamespaces.Store(&secretNamespaceSet{namespaces: map[string]struct{}{}})
}

// SetAllowedSecretRefNamespaces configures the process-wide allowlist of
// namespaces that may be referenced cross-namespace by a SecretReference (K-04).
// Empty entries and surrounding whitespace are ignored; an empty or nil list
// restores deny-by-default.
func SetAllowedSecretRefNamespaces(namespaces []string) {
	set := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		ns = strings.TrimSpace(ns)
		if ns != "" {
			set[ns] = struct{}{}
		}
	}
	allowedSecretRefNamespaces.Store(&secretNamespaceSet{namespaces: set})
}

// SecretRefNamespaceAllowed reports whether a SecretReference may point at the
// given namespace from a sink in another namespace.
func SecretRefNamespaceAllowed(namespace string) bool {
	current := allowedSecretRefNamespaces.Load()
	if current == nil {
		return false
	}
	_, ok := current.namespaces[namespace]

	return ok
}

// ValidateSecretRefNamespaces rejects every SecretReference on a normalized sink
// spec whose namespace is neither empty (implicit same-namespace), nor the sink's
// own namespace, nor in the operator allowlist (K-04).
//
// It walks the full reference surface a family sink can carry, so no nested
// credential reference (caSecretRef, git auth, databaseRef, per-backend
// secretRef) is left unguarded. sinkNamespace is the namespace of the sink CR
// being admitted.
func ValidateSecretRefNamespaces(spec *kollectdevv1alpha1.KollectSinkSpec, sinkNamespace string) field.ErrorList {
	if spec == nil {
		return nil
	}

	base := field.NewPath("spec")
	var allErrs field.ErrorList

	check := func(path *field.Path, ref *kollectdevv1alpha1.SecretReference) {
		allErrs = append(allErrs, validateSecretRefNamespace(ref, path, sinkNamespace)...)
	}

	check(base.Child("secretRef"), spec.SecretRef)

	if spec.TLS != nil {
		check(base.Child("tls").Child("caSecretRef"), spec.TLS.CASecretRef)
	}
	if spec.Git != nil && spec.Git.Auth != nil {
		check(base.Child("git").Child("auth").Child("secretRef"), spec.Git.Auth.SecretRef)
	}
	if spec.Postgres != nil {
		check(base.Child("postgres").Child("databaseRef"), spec.Postgres.DatabaseRef)
	}
	if spec.MongoDB != nil {
		check(base.Child("mongodb").Child("databaseRef"), spec.MongoDB.DatabaseRef)
	}
	if spec.BigQuery != nil {
		check(base.Child("bigquery").Child("secretRef"), spec.BigQuery.SecretRef)
	}
	if spec.Nats != nil {
		check(base.Child("nats").Child("secretRef"), spec.Nats.SecretRef)
	}
	if spec.Kafka != nil {
		check(base.Child("kafka").Child("secretRef"), spec.Kafka.SecretRef)
	}

	return allErrs
}

func validateSecretRefNamespace(
	ref *kollectdevv1alpha1.SecretReference,
	path *field.Path,
	sinkNamespace string,
) field.ErrorList {
	if ref == nil {
		return nil
	}

	ns := strings.TrimSpace(ref.Namespace)
	if ns == "" || ns == sinkNamespace {
		return nil
	}
	if SecretRefNamespaceAllowed(ns) {
		return nil
	}

	return field.ErrorList{field.Forbidden(path.Child("namespace"), fmt.Sprintf(
		"cross-namespace Secret reference to namespace %q is not permitted: a secretRef must name "+
			"a Secret in the sink's own namespace %q unless the operator allowlists the namespace via "+
			"--allow-secret-ref-namespaces (K-04)",
		ns, sinkNamespace))}
}
