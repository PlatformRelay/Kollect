// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

import "strings"

const defaultRemoteClusterNamespace = "kollect-system"

// RemoteClusterNamespace returns the namespace for ref, defaulting to kollect-system.
func RemoteClusterNamespace(ref RemoteClusterRef) string {
	ns := strings.TrimSpace(ref.Namespace)
	if ns == "" {
		return defaultRemoteClusterNamespace
	}

	return ns
}

// RemoteClusterKey returns namespace/name for a remote cluster reference.
func RemoteClusterKey(ref RemoteClusterRef) string {
	return RemoteClusterNamespace(ref) + "/" + strings.TrimSpace(ref.Name)
}
