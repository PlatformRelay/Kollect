// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// ToKollectSinkSpec normalizes a snapshot family spec into the internal registry shape (ADR-0414).
func (s *KollectSnapshotSinkSpec) ToKollectSinkSpec() KollectSinkSpec {
	if s == nil {
		return KollectSinkSpec{}
	}
	return KollectSinkSpec{
		Type:              s.Type,
		Endpoint:          s.Endpoint,
		SecretRef:         s.SecretRef,
		TLS:               s.TLS,
		ConnectionTest:    s.ConnectionTest,
		Cluster:           s.Cluster,
		PathTemplate:      s.PathTemplate,
		ExportMinInterval: s.ExportMinInterval,
		Git:               s.Git,
		GitLab:            s.GitLab,
		ObjectStore:       s.ObjectStore,
	}
}

// ToKollectSinkSpec normalizes a database family spec into the internal registry shape.
func (s *KollectDatabaseSinkSpec) ToKollectSinkSpec() KollectSinkSpec {
	if s == nil {
		return KollectSinkSpec{}
	}
	return KollectSinkSpec{
		Type:              s.Type,
		Endpoint:          s.Endpoint,
		SecretRef:         s.SecretRef,
		TLS:               s.TLS,
		ConnectionTest:    s.ConnectionTest,
		Cluster:           s.Cluster,
		PathTemplate:      s.PathTemplate,
		ExportMinInterval: s.ExportMinInterval,
		Postgres:          s.Postgres,
	}
}

// ToKollectSinkSpec normalizes an event family spec into the internal registry shape.
func (s *KollectEventSinkSpec) ToKollectSinkSpec() KollectSinkSpec {
	if s == nil {
		return KollectSinkSpec{}
	}
	return KollectSinkSpec{
		Type:              s.Type,
		Endpoint:          s.Endpoint,
		SecretRef:         s.SecretRef,
		TLS:               s.TLS,
		ConnectionTest:    s.ConnectionTest,
		Cluster:           s.Cluster,
		PathTemplate:      s.PathTemplate,
		ExportMinInterval: s.ExportMinInterval,
		Nats:              s.Nats,
		Kafka:             s.Kafka,
	}
}
