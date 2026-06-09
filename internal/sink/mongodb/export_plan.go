// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"go.mongodb.org/mongo-driver/bson"

	"github.com/konih/kollect/internal/collect"
)

type exportScope struct {
	inventoryNamespace string
	inventoryName      string
	cluster            string
}

func newExportScope(objectPath, cluster string) exportScope {
	invNS, invName := inventoryFromObjectPath(objectPath)
	return exportScope{
		inventoryNamespace: invNS,
		inventoryName:      invName,
		cluster:            cluster,
	}
}

func (s exportScope) filter() bson.M {
	return bson.M{
		"inventory_namespace": s.inventoryNamespace,
		"inventory_name":      s.inventoryName,
		"cluster":             s.cluster,
	}
}

func upsertFilter(scope exportScope, item collect.Item) bson.M {
	filter := scope.filter()
	filter["target_name"] = item.TargetName
	filter["source_uid"] = item.UID

	return filter
}
