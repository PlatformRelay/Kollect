// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"fmt"
	"strings"
)

// Inventory kinds an export request can name (ADR-0422).
const (
	InventoryKindNamespaced = "KollectInventory"
	InventoryKindCluster    = "KollectClusterInventory"

	// clusterInventoryPathNamespace is the namespace component a KollectClusterInventory's object
	// path carries (inventory/cluster/<name>.json). A KollectInventory in a namespace called
	// "cluster" renders the same component, which is why the path cannot tell the two apart.
	clusterInventoryPathNamespace = "cluster"
)

// InventoryIdentity names the inventory an export belongs to: its kind, its namespace (empty for
// KollectClusterInventory) and its name, without any multipart suffix. The object path still drives
// path rendering; the identity decides who owns the files a tree export writes (ADR-0422).
type InventoryIdentity struct {
	Kind      string
	Namespace string
	Name      string
}

// IsZero reports whether no identity was supplied.
func (id InventoryIdentity) IsZero() bool {
	return id == InventoryIdentity{}
}

func (id InventoryIdentity) String() string {
	if id.Kind == InventoryKindCluster {
		return id.Kind + " " + id.Name
	}

	return id.Kind + " " + id.Namespace + "/" + id.Name
}

// validate checks the identity on its own: a known kind, a name, and a namespace exactly when the
// kind is namespaced.
func (id InventoryIdentity) validate() error {
	if strings.TrimSpace(id.Name) == "" || id.Name != strings.TrimSpace(id.Name) {
		return fmt.Errorf("inventory identity %+v has no valid name", id)
	}
	switch id.Kind {
	case InventoryKindNamespaced:
		if strings.TrimSpace(id.Namespace) == "" || id.Namespace != strings.TrimSpace(id.Namespace) {
			return fmt.Errorf("inventory identity %+v: %s needs a namespace", id, id.Kind)
		}
	case InventoryKindCluster:
		if id.Namespace != "" {
			return fmt.Errorf("inventory identity %+v: %s is cluster-scoped and has no namespace", id, id.Kind)
		}
	default:
		return fmt.Errorf("inventory identity %+v: unknown kind %q", id, id.Kind)
	}

	return nil
}

// pathNamespace is the namespace component this identity's object path must carry.
func (id InventoryIdentity) pathNamespace() string {
	if id.Kind == InventoryKindCluster {
		return clusterInventoryPathNamespace
	}

	return id.Namespace
}

// checkObjectPath fails when the identity and the object path disagree: the namespace and the base
// name parsed from the path (multipart suffix removed) must be the identity's, and a
// KollectClusterInventory must come with path namespace "cluster".
func (id InventoryIdentity) checkObjectPath(objectPath, pathNS, pathName string, partIndex, partTotal int) error {
	if err := id.validate(); err != nil {
		return err
	}
	base := baseInventoryName(pathName, partIndex, partTotal)
	if pathNS != id.pathNamespace() || base != id.Name {
		return fmt.Errorf("inventory identity %s does not match object path %q (namespace %q, name %q)",
			id, objectPath, pathNS, base)
	}

	return nil
}
