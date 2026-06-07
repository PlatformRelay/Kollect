// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"fmt"
	"strings"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// TypeName is the KollectSink.spec.type value for MongoDB sinks.
const TypeName = "mongodb"

const typeName = TypeName

// Config holds resolved MongoDB sink settings.
type Config struct {
	URI              string
	Database         string
	Collection       string
	Cluster          string
	ProvisioningMode string
}

// ConfigFromSpec validates spec and secret data for a mongodb sink.
func ConfigFromSpec(
	spec kollectdevv1alpha1.KollectSinkSpec,
	databaseSecret map[string][]byte,
) (Config, error) {
	if spec.Type != typeName {
		return Config{}, fmt.Errorf("expected mongodb sink, got %q", spec.Type)
	}

	if spec.MongoDB == nil {
		return Config{}, fmt.Errorf("mongodb sink requires spec.mongodb")
	}

	mongo := spec.MongoDB
	if mongo.DatabaseRef == nil || strings.TrimSpace(mongo.DatabaseRef.Name) == "" {
		return Config{}, fmt.Errorf("mongodb sink requires spec.mongodb.databaseRef")
	}

	database := strings.TrimSpace(mongo.Database)
	if database == "" {
		return Config{}, fmt.Errorf("mongodb sink requires spec.mongodb.database")
	}

	collection := strings.TrimSpace(mongo.Collection)
	if collection == "" {
		return Config{}, fmt.Errorf("mongodb sink requires spec.mongodb.collection")
	}

	uri, err := uriFromSecret(databaseSecret)
	if err != nil {
		return Config{}, err
	}

	return Config{
		URI:              uri,
		Database:         database,
		Collection:       collection,
		Cluster:          strings.TrimSpace(spec.Cluster),
		ProvisioningMode: kollectdevv1alpha1.EffectiveProvisioningMode(&spec),
	}, nil
}

func uriFromSecret(data map[string][]byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("mongodb databaseRef secret is empty")
	}

	for _, key := range []string{"uri", "url", "connectionString", "MONGODB_URI"} {
		if v, ok := data[key]; ok && len(strings.TrimSpace(string(v))) > 0 {
			return strings.TrimSpace(string(v)), nil
		}
	}

	return "", fmt.Errorf("mongodb secret must contain uri or connectionString key")
}
