// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink"
)

// SCR-5: kollect-pipeline rejects an empty referenced Secret with the operator's message, before
// any backend is built, and builds from the current manifest on every run.

// operatorEmptySecretError is what the operator's resolver returns for the same Secret.
func operatorEmptySecretError(t *testing.T, secret corev1.Secret) string {
	t.Helper()

	c := fake.NewClientBuilder().WithScheme(clientgoscheme.Scheme).WithObjects(secret.DeepCopy()).Build()
	_, err := sink.ResolveSecret(context.Background(), c,
		&kollectdevv1alpha1.SecretReference{Name: secret.Name}, secret.Namespace)
	if err == nil {
		t.Fatal("the operator's resolver accepted the empty Secret; the oracle is wrong")
	}

	return err.Error()
}

func TestResolveSinkSecretData_emptySecretFails(t *testing.T) {
	t.Setenv("KOLLECT_TEST_EMPTY_PARITY", "from-the-environment")

	const name = "creds"
	rows := []struct {
		name    string
		secret  corev1.Secret
		wantErr string // "" = accepted (control)
		oracle  bool   // compare with the operator's text
	}{
		{
			name:    "no data and no stringData",
			secret:  corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: sinkNamespace}},
			wantErr: `secret "creds" in namespace "kollect-system" has no data`, oracle: true,
		},
		{
			name: "data keys all empty",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: sinkNamespace},
				Data:       map[string][]byte{"token": {}, "username": []byte("")},
			},
			wantErr: `secret "creds" in namespace "kollect-system" has no data`, oracle: true,
		},
		{
			name: "stringData only, non-empty (control)",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: sinkNamespace},
				StringData: map[string]string{"token": "sentinel-token"},
			},
		},
		{
			name: "env placeholder with the variable set (control)",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: sinkNamespace},
				StringData: map[string]string{"token": "${env:KOLLECT_TEST_EMPTY_PARITY}"}, //nolint:gosec // G101: placeholder, not a credential
			},
		},
		{
			name:    "no namespace in the manifest",
			secret:  corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name}},
			wantErr: `secret "creds" has no data`,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			spec := kollectdevv1alpha1.KollectSinkSpec{
				Type:      "git",
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: name},
			}

			data, err := ResolveSinkSecretData(spec, []corev1.Secret{row.secret})
			if row.wantErr == "" {
				if err != nil {
					t.Fatalf("control row rejected: %v", err)
				}
				if len(data["token"]) == 0 {
					t.Fatal("control row resolved no token")
				}

				return
			}

			if err == nil {
				t.Fatal("the CLI accepted an empty Secret; the backend would be built without credentials")
			}
			if err.Error() != row.wantErr {
				t.Fatalf("error %q, want %q", err, row.wantErr)
			}
			if row.oracle {
				if op := operatorEmptySecretError(t, row.secret); err.Error() != op {
					t.Fatalf("CLI error %q differs from the operator's %q", err, op)
				}
			}
		})
	}
}

func TestResolveSinkSecretData_emptySecretFailsBeforeTheBackend(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		"sink.yaml":  gitSinkYAML("  secretRef:\n    name: creds\n"),
		"creds.yaml": "apiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\n  namespace: " + sinkNamespace + "\n",
	})

	_, called, err := resolveThroughCLI(t, dir)
	if err == nil {
		t.Fatal("an empty Secret manifest was accepted")
	}
	if want := `secret "creds" in namespace "kollect-system" has no data`; err.Error() != want {
		t.Fatalf("error %q, want %q", err, want)
	}
	if called {
		t.Fatal("the backend was built from an empty Secret")
	}
}

func TestRunOneContext_secretManifestChangeNextRun(t *testing.T) {
	run := func(token string) string {
		dir := writeConfigDir(t, map[string]string{
			"sink.yaml":  gitSinkYAML("  secretRef:\n    name: creds\n"),
			"creds.yaml": secretYAML("creds", map[string]string{"token": token}),
		})

		got, called, err := resolveThroughCLI(t, dir)
		if err != nil || !called {
			t.Fatalf("run with %q: called=%v err=%v", token, called, err)
		}

		return string(got.build.SecretData["token"])
	}

	if got := run("sentinel-token-A"); got != "sentinel-token-A" {
		t.Fatalf("first run built from %q", got)
	}
	if got := run("sentinel-token-B"); got != "sentinel-token-B" {
		t.Fatalf("second run built from %q, want the changed manifest's value", got)
	}
}
