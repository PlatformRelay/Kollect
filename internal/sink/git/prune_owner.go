// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"encoding/json"
	"fmt"
)

// pruneOwnerVersion tags the kind-qualified owner encoding (ADR-0422). Owners are otherwise opaque
// to the engine: hashed for the record path and compared as strings.
const pruneOwnerVersion = "v2"

// InventoryPruneOwner encodes the prune owner of one inventory as the JSON array
// ["v2", kind, cluster, namespace, name]. namespace is empty for a cluster-scoped kind. The owner
// carries no UID, so a deleted and recreated inventory with the same kind, namespace and name
// continues its predecessor's record. JSON separates the components without delimiter collisions.
func InventoryPruneOwner(kind, cluster, namespace, name string) (string, error) {
	if kind == "" || cluster == "" || name == "" {
		return "", fmt.Errorf("prune owner needs kind, cluster and name (got %q, %q, %q)", kind, cluster, name)
	}
	owner, err := json.Marshal([5]string{pruneOwnerVersion, kind, cluster, namespace, name})
	if err != nil {
		return "", fmt.Errorf("encode prune owner: %w", err)
	}

	return string(owner), nil
}

// describePruneOwner renders an owner for an error message: the inventory's kind, namespace/name
// and sink cluster when it is a v2 owner, the raw string otherwise.
func describePruneOwner(owner string) string {
	var parts []string
	if err := json.Unmarshal([]byte(owner), &parts); err != nil || len(parts) != 5 || parts[0] != pruneOwnerVersion {
		return fmt.Sprintf("unrecognised owner %q", owner)
	}
	kind, cluster, namespace, name := parts[1], parts[2], parts[3], parts[4]
	if namespace == "" {
		return fmt.Sprintf("%s %s (cluster %q)", kind, name, cluster)
	}

	return fmt.Sprintf("%s %s/%s (cluster %q)", kind, namespace, name, cluster)
}
