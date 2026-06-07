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
