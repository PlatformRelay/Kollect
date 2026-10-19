//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink/postgres"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// PI4: a DSN Secret that conflicts with the sink CA fails the acquisition; once the
// Secret is corrected, the next acquisition builds and exports with the sink spec
// unchanged, so the failure was not cached.
func TestPostgresCATrust_PI4_recoversAfterSecretFix(t *testing.T) {
	caX := sinktlstest.NewCA(t, "X")
	srv := sinktlstest.StartTLSPostgres(t, caX.Server(t))

	ResetBackendPoolForTest()
	t.Cleanup(ResetBackendPoolForTest)

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "team-a"},
		Data:       map[string][]byte{"dsn": []byte(srv.DSN("sslmode=verify-full sslrootcert=/x"))},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:    "postgres",
		Cluster: "test-cluster",
		TLS:     &kollectdevv1alpha1.TLSSpec{CABundle: caX.PEM},
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "ca_trust_recovery",
			Schema:      "public",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	reg := NewRegistry()

	_, release, err := acquireBackend(ctx, cl, reg, "team-a", "pg-sink", "uid-pi4", spec)
	release()
	if !errors.Is(err, postgres.ErrCARootConflict) {
		t.Fatalf("first acquisition: err %v, want ErrCARootConflict", err)
	}

	secret.Data["dsn"] = []byte(srv.DSN("sslmode=verify-full"))
	if err = cl.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}

	backend, release, err := acquireBackend(ctx, cl, reg, "team-a", "pg-sink", "uid-pi4", spec)
	if err != nil {
		t.Fatalf("acquisition after the Secret fix: %v", err)
	}
	defer release()

	payload, err := json.Marshal([]collect.Item{{
		TargetNamespace: "apps", TargetName: "web", Namespace: "apps", Name: "demo",
		Version: "v1", Kind: "Deployment", UID: "uid-pi4",
	}})
	if err != nil {
		t.Fatal(err)
	}

	if err := backend.Export(ctx, payload, "inventory/apps/demo.json"); err != nil {
		t.Fatalf("export after the Secret fix: %v", err)
	}
}
