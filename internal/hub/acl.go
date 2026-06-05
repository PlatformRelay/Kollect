// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package hub

import (
	"fmt"
	"strings"
)

// ValidateClusterACL rejects reports from clusters outside the hub registration allowlist.
// When enforced is false and allowlist is empty, any cluster is permitted (dev / open queue).
// When enforced is true, an empty allowlist rejects all clusters (fail-closed production hubs).
func ValidateClusterACL(cluster string, allowlist []string, enforced bool) error {
	cluster = strings.TrimSpace(cluster)
	if cluster == "" {
		return fmt.Errorf("hub acl: cluster is required")
	}

	if !enforced && len(allowlist) == 0 {
		return nil
	}

	for _, allowed := range allowlist {
		if cluster == strings.TrimSpace(allowed) {
			return nil
		}
	}

	return fmt.Errorf("hub acl: cluster %q is not registered", cluster)
}
