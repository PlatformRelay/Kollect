// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestBuildContextFromSpec_inlineTLSAndSecrets(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	caPEM := []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "git-creds", Namespace: "team-a"},
		Data:       map[string][]byte{"token": []byte("tok")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ctx, err := BuildContextFromSpec(context.Background(), cl, kollectdevv1alpha1.KollectSinkSpec{
		Type:      "git",
		Endpoint:  "https://example.com/repo.git",
		SecretRef: &kollectdevv1alpha1.SecretReference{Name: "git-creds"},
		TLS:       &kollectdevv1alpha1.TLSSpec{CABundle: caPEM},
	}, "team-a")
	if err != nil {
		t.Fatalf("BuildContextFromSpec: %v", err)
	}

	if string(ctx.CAPEM) != string(caPEM) {
		t.Fatalf("CAPEM = %q", ctx.CAPEM)
	}

	if string(ctx.SecretData["token"]) != "tok" {
		t.Fatalf("SecretData = %+v", ctx.SecretData)
	}
}

func TestBuildContextFromSpec_postgresDatabaseSecret(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "kollect-system"},
		Data:       map[string][]byte{"dsn": []byte("postgres://localhost/db")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ctx, err := BuildContextFromSpec(context.Background(), cl, kollectdevv1alpha1.KollectSinkSpec{
		Type: kollectdevv1alpha1.SinkTypePostgres,
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "items",
		},
	}, "")
	if err != nil {
		t.Fatalf("BuildContextFromSpec: %v", err)
	}

	if string(ctx.DatabaseSecretData["dsn"]) != "postgres://localhost/db" {
		t.Fatalf("DatabaseSecretData = %+v", ctx.DatabaseSecretData)
	}
}

func TestBuildContextFromSpec_kafkaSecretFallback(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "kafka", Namespace: "kollect-system"},
		Data:       map[string][]byte{"password": []byte("pw")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ctx, err := BuildContextFromSpec(context.Background(), cl, kollectdevv1alpha1.KollectSinkSpec{
		Type:      "kafka",
		SecretRef: &kollectdevv1alpha1.SecretReference{Name: "kafka"},
		Kafka: &kollectdevv1alpha1.KafkaSpec{
			Brokers: []string{"localhost:9092"},
			Topic:   "inventory",
		},
	}, "")
	if err != nil {
		t.Fatalf("BuildContextFromSpec: %v", err)
	}

	if string(ctx.SecretData["password"]) != "pw" {
		t.Fatalf("SecretData = %+v", ctx.SecretData)
	}
}

func TestBuildContextFromSpec_natsAndCASecret(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	natsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "nats-creds", Namespace: "team-a"},
		Data:       map[string][]byte{"token": []byte("nats-token")},
	}
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: "team-a"},
		Data:       map[string][]byte{"ca.crt": []byte("pem-bytes")},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(natsSecret, caSecret).Build()

	ctx, err := BuildContextFromSpec(context.Background(), cl, kollectdevv1alpha1.KollectSinkSpec{
		Type: "nats",
		Nats: &kollectdevv1alpha1.NatsSpec{
			URL:       "nats://localhost:4222",
			Subject:   "inventory.events",
			SecretRef: &kollectdevv1alpha1.SecretReference{Name: "nats-creds"},
		},
		TLS: &kollectdevv1alpha1.TLSSpec{
			CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca"},
		},
	}, "team-a")
	if err != nil {
		t.Fatalf("BuildContextFromSpec: %v", err)
	}

	if string(ctx.SecretData["token"]) != "nats-token" {
		t.Fatalf("SecretData = %+v", ctx.SecretData)
	}
	if string(ctx.CAPEM) != "pem-bytes" {
		t.Fatalf("CAPEM = %q", ctx.CAPEM)
	}
}
