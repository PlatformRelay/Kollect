// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"strings"

	"github.com/konih/kollect/internal/collect"
)

// CleanupCluster removes all merged spoke inventory rows for clusterName from the hub store.
func CleanupCluster(store *collect.Store, clusterName string) {
	if store == nil {
		return
	}

	clusterName = strings.TrimSpace(clusterName)
	if clusterName == "" {
		return
	}

	store.RemoveCluster(clusterName)
}
