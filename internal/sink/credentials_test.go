// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

func TestResolveSecret_nilRef(t *testing.T) {
	t.Parallel()

	creds, err := ResolveSecret(context.Background(), nil, nil, "kollect-system")
	if err != nil || creds.Data != nil {
		t.Fatalf("nil ref: creds=%+v err=%v", creds, err)
	}
}

func TestResolveSecret_loadsCredentials(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "git-creds", Namespace: "kollect-system"},
		Data: map[string][]byte{
			"username": []byte("bot"),
			"password": []byte("secret-token"),
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ref := &kollectdevv1alpha1.SecretReference{Name: "git-creds"}
	creds, err := ResolveSecret(context.Background(), cl, ref, "kollect-system")
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}

	if creds.Username != "bot" || creds.Password != "secret-token" || creds.Token != "secret-token" {
		t.Fatalf("creds = %+v", creds)
	}
}

func TestResolveSecret_missingSecret(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveSecret(context.Background(), fake.NewClientBuilder().WithScheme(scheme).Build(),
		&kollectdevv1alpha1.SecretReference{Name: "missing"}, "kollect-system")
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

// EDGE: an unresolved secretRef must wrap the NotFound into an error that names
// the missing secret and its namespace, so operators can see which ref is wrong
// without guessing. A bare "not found" would be undiagnosable across many refs.
func TestResolveSecret_missingSecretNamesRef(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	// ref.Namespace is empty, so resolution must fall through to defaultNamespace
	// ("team-a") and that resolved namespace is what the error must report.
	_, err := ResolveSecret(
		context.Background(),
		fake.NewClientBuilder().WithScheme(scheme).Build(),
		&kollectdevv1alpha1.SecretReference{Name: "git-creds"},
		"team-a",
	)
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
	if !strings.Contains(err.Error(), "git-creds") {
		t.Fatalf("error must name the missing secret ref: %v", err)
	}
	if !strings.Contains(err.Error(), "team-a") {
		t.Fatalf("error must name the resolved namespace: %v", err)
	}
}

// A non-NotFound API error (e.g. apiserver unavailable) must propagate verbatim
// rather than being masked as a "not found" message, so a transient control-plane
// failure is not misreported as a permanent misconfiguration.
func TestResolveSecret_nonNotFoundPropagates(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("apiserver unavailable")
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption,
			) error {
				return sentinel
			},
		}).
		Build()

	_, err := ResolveSecret(context.Background(), cl,
		&kollectdevv1alpha1.SecretReference{Name: "git-creds"}, "kollect-system")
	if !errors.Is(err, sentinel) {
		t.Fatalf("non-NotFound error must propagate unchanged, got %v", err)
	}
}
