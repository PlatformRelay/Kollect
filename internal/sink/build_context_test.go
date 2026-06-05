// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestBuildContextFromSpec(t *testing.T) {
	t.Parallel()

	caPEM := []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----")

	tests := []struct {
		name          string
		namespace     string
		secrets       []*corev1.Secret
		spec          kollectdevv1alpha1.KollectSinkSpec
		wantSecretKey string
		wantSecretVal string
		wantDBKey     string
		wantDBVal     string
		wantCAPEM     string
	}{
		{
			name:      "git inline TLS and token secret",
			namespace: "team-a",
			secrets: []*corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "git-creds", Namespace: "team-a"},
				Data:       map[string][]byte{"token": []byte("tok")},
			}},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:      "git",
				Endpoint:  "https://example.com/repo.git",
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "git-creds"},
				TLS:       &kollectdevv1alpha1.TLSSpec{CABundle: caPEM},
			},
			wantSecretKey: "token",
			wantSecretVal: "tok",
			wantCAPEM:     string(caPEM),
		},
		{
			name: "postgres database secret",
			secrets: []*corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: "kollect-system"},
				Data:       map[string][]byte{"dsn": []byte("postgres://localhost/db")},
			}},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: kollectdevv1alpha1.SinkTypePostgres,
				Postgres: &kollectdevv1alpha1.PostgresSpec{
					DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
					Table:       "items",
				},
			},
			wantDBKey: "dsn",
			wantDBVal: "postgres://localhost/db",
		},
		{
			name: "kafka secret fallback",
			secrets: []*corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "kafka", Namespace: "kollect-system"},
				Data:       map[string][]byte{"password": []byte("pw")},
			}},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:      "kafka",
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "kafka"},
				Kafka: &kollectdevv1alpha1.KafkaSpec{
					Brokers: []string{"localhost:9092"},
					Topic:   "inventory",
				},
			},
			wantSecretKey: "password",
			wantSecretVal: "pw",
		},
		{
			name:      "nats credentials and CA secret",
			namespace: "team-a",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "nats-creds", Namespace: "team-a"},
					Data:       map[string][]byte{"token": []byte("nats-token")},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: "team-a"},
					Data:       map[string][]byte{"ca.crt": []byte("pem-bytes")},
				},
			},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "nats",
				Nats: &kollectdevv1alpha1.NatsSpec{
					URL:       "nats://localhost:4222",
					Subject:   "inventory.events",
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "nats-creds"},
				},
				TLS: &kollectdevv1alpha1.TLSSpec{
					CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca"},
				},
			},
			wantSecretKey: "token",
			wantSecretVal: "nats-token",
			wantCAPEM:     "pem-bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}

			objects := make([]client.Object, len(tt.secrets))
			for i, secret := range tt.secrets {
				objects[i] = secret
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

			ctx, err := BuildContextFromSpec(context.Background(), cl, tt.spec, tt.namespace)
			if err != nil {
				t.Fatalf("BuildContextFromSpec: %v", err)
			}

			if tt.wantSecretKey != "" {
				if string(ctx.SecretData[tt.wantSecretKey]) != tt.wantSecretVal {
					t.Fatalf("SecretData[%q] = %q, want %q", tt.wantSecretKey, ctx.SecretData[tt.wantSecretKey], tt.wantSecretVal)
				}
			}
			if tt.wantDBKey != "" {
				if string(ctx.DatabaseSecretData[tt.wantDBKey]) != tt.wantDBVal {
					t.Fatalf("DatabaseSecretData[%q] = %q, want %q", tt.wantDBKey, ctx.DatabaseSecretData[tt.wantDBKey], tt.wantDBVal)
				}
			}
			if tt.wantCAPEM != "" {
				if string(ctx.CAPEM) != tt.wantCAPEM {
					t.Fatalf("CAPEM = %q, want %q", ctx.CAPEM, tt.wantCAPEM)
				}
			}
		})
	}
}
