// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
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
		wantNoSecret  bool
		wantNoDB      bool
		wantNoCAPEM   bool
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
			name:      "git auth secret overrides top-level secret",
			namespace: "team-a",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "top-creds", Namespace: "team-a"},
					Data:       map[string][]byte{"token": []byte("top-tok")},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "git-auth", Namespace: "team-a"},
					Data:       map[string][]byte{"token": []byte("auth-tok")},
				},
			},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:      "git",
				Endpoint:  "https://example.com/repo.git",
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "top-creds"},
				Git: &kollectdevv1alpha1.GitSpec{
					Auth: &kollectdevv1alpha1.GitAuthSpec{
						Type:      "token",
						SecretRef: &kollectdevv1alpha1.SecretReference{Name: "git-auth"},
					},
				},
			},
			wantSecretKey: "token",
			wantSecretVal: "auth-tok",
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
			name: "postgres type without postgres block skips database secret",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: kollectdevv1alpha1.SinkTypePostgres,
			},
			wantNoDB: true,
		},
		{
			name: "bigquery database secret",
			secrets: []*corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "bq", Namespace: "kollect-system"},
				Data:       map[string][]byte{"credentials.json": []byte(`{"type":"service_account"}`)},
			}},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "bigquery",
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{
					Project:   "fleet-analytics",
					Dataset:   "inventory",
					Table:     "items",
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "bq"},
				},
			},
			wantDBKey: "credentials.json",
			wantDBVal: `{"type":"service_account"}`,
		},
		{
			name: "bigquery without secretRef skips database secret",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "bigquery",
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{
					Project: "fleet-analytics",
					Dataset: "inventory",
					Table:   "items",
				},
			},
			wantNoDB: true,
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
			name: "kafka typed secret overrides top-level",
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "top", Namespace: "kollect-system"},
					Data:       map[string][]byte{"password": []byte("top-pw")},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "kafka-sasl", Namespace: "kollect-system"},
					Data:       map[string][]byte{"password": []byte("kafka-pw")},
				},
			},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:      "kafka",
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "top"},
				Kafka: &kollectdevv1alpha1.KafkaSpec{
					Brokers:   []string{"localhost:9092"},
					Topic:     "inventory",
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "kafka-sasl"},
				},
			},
			wantSecretKey: "password",
			wantSecretVal: "kafka-pw",
		},
		{
			name: "kafka without any secretRef skips override",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "kafka",
				Kafka: &kollectdevv1alpha1.KafkaSpec{
					Brokers: []string{"localhost:9092"},
					Topic:   "inventory",
				},
			},
			wantNoSecret: true,
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
		{
			name:      "nats falls back to top-level secretRef",
			namespace: "team-a",
			secrets: []*corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "nats-top", Namespace: "team-a"},
				Data:       map[string][]byte{"token": []byte("fallback-tok")},
			}},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:      "nats",
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "nats-top"},
				Nats: &kollectdevv1alpha1.NatsSpec{
					URL:     "nats://localhost:4222",
					Subject: "inventory.events",
				},
			},
			wantSecretKey: "token",
			wantSecretVal: "fallback-tok",
		},
		{
			name: "nats without any secretRef skips override",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "nats",
				Nats: &kollectdevv1alpha1.NatsSpec{
					URL:     "nats://localhost:4222",
					Subject: "inventory.events",
				},
			},
			wantNoSecret: true,
		},
		{
			name: "tls with neither caBundle nor caSecretRef yields empty CAPEM",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "git",
				TLS:  &kollectdevv1alpha1.TLSSpec{InsecureSkipVerify: true},
			},
			wantNoCAPEM: true,
		},
		{
			name: "ca secret with no known PEM keys yields empty CAPEM",
			secrets: []*corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "ca-other", Namespace: "kollect-system"},
				Data:       map[string][]byte{"other.pem": []byte("ignored")},
			}},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "git",
				TLS: &kollectdevv1alpha1.TLSSpec{
					CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca-other"},
				},
			},
			wantNoCAPEM: true,
		},
		{
			name: "ca secret prefers tls.crt key",
			secrets: []*corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "ca-tls", Namespace: "kollect-system"},
				Data: map[string][]byte{
					"tls.crt": []byte("from-tls-crt"),
					"ca.crt":  []byte("from-ca-crt"),
				},
			}},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "git",
				TLS: &kollectdevv1alpha1.TLSSpec{
					CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca-tls"},
				},
			},
			wantCAPEM: "from-tls-crt",
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
			if tt.wantNoSecret && len(ctx.SecretData) != 0 {
				t.Fatalf("SecretData = %#v, want empty", ctx.SecretData)
			}
			if tt.wantDBKey != "" {
				if string(ctx.DatabaseSecretData[tt.wantDBKey]) != tt.wantDBVal {
					t.Fatalf("DatabaseSecretData[%q] = %q, want %q", tt.wantDBKey, ctx.DatabaseSecretData[tt.wantDBKey], tt.wantDBVal)
				}
			}
			if tt.wantNoDB && len(ctx.DatabaseSecretData) != 0 {
				t.Fatalf("DatabaseSecretData = %#v, want empty", ctx.DatabaseSecretData)
			}
			if tt.wantCAPEM != "" {
				if string(ctx.CAPEM) != tt.wantCAPEM {
					t.Fatalf("CAPEM = %q, want %q", ctx.CAPEM, tt.wantCAPEM)
				}
			}
			if tt.wantNoCAPEM && len(ctx.CAPEM) != 0 {
				t.Fatalf("CAPEM = %q, want empty", ctx.CAPEM)
			}
		})
	}
}

func TestBuildContextFromSpec_errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace string
		secrets   []*corev1.Secret
		spec      kollectdevv1alpha1.KollectSinkSpec
		wantSub   string
	}{
		{
			name: "missing top-level secret",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:      "git",
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "missing-top"},
			},
			wantSub: "missing-top",
		},
		{
			name: "missing git auth secret",
			secrets: []*corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "top", Namespace: "kollect-system"},
				Data:       map[string][]byte{"token": []byte("tok")},
			}},
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type:      "git",
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "top"},
				Git: &kollectdevv1alpha1.GitSpec{
					Auth: &kollectdevv1alpha1.GitAuthSpec{
						SecretRef: &kollectdevv1alpha1.SecretReference{Name: "missing-git-auth"},
					},
				},
			},
			wantSub: "missing-git-auth",
		},
		{
			name: "missing postgres database secret",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: kollectdevv1alpha1.SinkTypePostgres,
				Postgres: &kollectdevv1alpha1.PostgresSpec{
					DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "missing-pg"},
					Table:       "items",
				},
			},
			wantSub: "missing-pg",
		},
		{
			name: "missing bigquery secret",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "bigquery",
				BigQuery: &kollectdevv1alpha1.BigQuerySpec{
					Project:   "fleet",
					Dataset:   "inv",
					Table:     "items",
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "missing-bq"},
				},
			},
			wantSub: "missing-bq",
		},
		{
			name: "missing kafka secret",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "kafka",
				Kafka: &kollectdevv1alpha1.KafkaSpec{
					Brokers:   []string{"localhost:9092"},
					Topic:     "inventory",
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "missing-kafka"},
				},
			},
			wantSub: "missing-kafka",
		},
		{
			name: "missing nats secret",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "nats",
				Nats: &kollectdevv1alpha1.NatsSpec{
					URL:       "nats://localhost:4222",
					Subject:   "inventory.events",
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "missing-nats"},
				},
			},
			wantSub: "missing-nats",
		},
		{
			name: "missing ca secret",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "git",
				TLS: &kollectdevv1alpha1.TLSSpec{
					CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "missing-ca"},
				},
			},
			wantSub: "missing-ca",
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

			_, err := BuildContextFromSpec(context.Background(), cl, tt.spec, tt.namespace)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("error %q must contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestGitAuthSecretRef(t *testing.T) {
	t.Parallel()

	ref := &kollectdevv1alpha1.SecretReference{Name: "git-auth"}

	tests := []struct {
		name string
		spec kollectdevv1alpha1.KollectSinkSpec
		want *kollectdevv1alpha1.SecretReference
	}{
		{
			name: "non-git type",
			spec: kollectdevv1alpha1.KollectSinkSpec{Type: "nats"},
		},
		{
			name: "git without git block",
			spec: kollectdevv1alpha1.KollectSinkSpec{Type: "git"},
		},
		{
			name: "git without auth",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "git",
				Git:  &kollectdevv1alpha1.GitSpec{Branch: "main"},
			},
		},
		{
			name: "git auth without secretRef",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "git",
				Git: &kollectdevv1alpha1.GitSpec{
					Auth: &kollectdevv1alpha1.GitAuthSpec{Type: "token"},
				},
			},
		},
		{
			name: "git auth secretRef",
			spec: kollectdevv1alpha1.KollectSinkSpec{
				Type: "git",
				Git: &kollectdevv1alpha1.GitSpec{
					Auth: &kollectdevv1alpha1.GitAuthSpec{SecretRef: ref},
				},
			},
			want: ref,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := gitAuthSecretRef(tt.spec)
			if got != tt.want {
				t.Fatalf("gitAuthSecretRef = %#v, want %#v", got, tt.want)
			}
		})
	}
}
