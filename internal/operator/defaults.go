// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package operator

// DefaultSecretNamespace is the namespace for cluster-scoped profiles and sink secrets
// when spec.sinkNamespace is unset on cluster inventory / target resources.
const DefaultSecretNamespace = "kollect-system" //nolint:gosec // namespace name, not a credential
