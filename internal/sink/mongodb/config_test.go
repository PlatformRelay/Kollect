// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"testing"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestConfigFromSpec_requiresMongoBlock(t *testing.T) {
	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: TypeName}, nil)
	if err == nil {
		t.Fatal("expected error without mongodb block")
	}
}

func TestConfigFromSpec_resolvesURI(t *testing.T) {
	cfg, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "items",
		},
	}, map[string][]byte{"uri": []byte("mongodb://localhost:27017")})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URI != "mongodb://localhost:27017" || cfg.Collection != "items" {
		t.Fatalf("unexpected cfg: %#v", cfg)
	}
}

func TestConfigFromSpec_wrongType(t *testing.T) {
	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{Type: "postgres"}, nil)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestConfigFromSpec_missingDatabaseRef(t *testing.T) {
	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type:    TypeName,
		MongoDB: &kollectdevv1alpha1.MongoSpec{Database: "inv", Collection: "items"},
	}, nil)
	if err == nil {
		t.Fatal("expected error without databaseRef")
	}
}

func TestConfigFromSpec_missingDatabase(t *testing.T) {
	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Collection:  "items",
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error without database name")
	}
}

func TestConfigFromSpec_missingCollection(t *testing.T) {
	_, err := ConfigFromSpec(kollectdevv1alpha1.KollectSinkSpec{
		Type: TypeName,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error without collection name")
	}
}

func TestURIFromSecret_emptySecret(t *testing.T) {
	_, err := uriFromSecret(nil)
	if err == nil {
		t.Fatal("expected error for nil secret")
	}

	_, err = uriFromSecret(map[string][]byte{})
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestURIFromSecret_urlKey(t *testing.T) {
	uri, err := uriFromSecret(map[string][]byte{"url": []byte("mongodb://host:27017")})
	if err != nil {
		t.Fatal(err)
	}
	if uri != "mongodb://host:27017" {
		t.Fatalf("uri = %q", uri)
	}
}

func TestURIFromSecret_connectionStringKey(t *testing.T) {
	uri, err := uriFromSecret(map[string][]byte{"connectionString": []byte("mongodb://cs:27017")})
	if err != nil {
		t.Fatal(err)
	}
	if uri != "mongodb://cs:27017" {
		t.Fatalf("uri = %q", uri)
	}
}

func TestURIFromSecret_mongodbURIKey(t *testing.T) {
	uri, err := uriFromSecret(map[string][]byte{"MONGODB_URI": []byte("mongodb://env:27017")})
	if err != nil {
		t.Fatal(err)
	}
	if uri != "mongodb://env:27017" {
		t.Fatalf("uri = %q", uri)
	}
}

func TestURIFromSecret_unknownKeys(t *testing.T) {
	_, err := uriFromSecret(map[string][]byte{"password": []byte("secret")})
	if err == nil {
		t.Fatal("expected error for secret with no recognized key")
	}
}
