// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink"
	s3sink "github.com/platformrelay/kollect/internal/sink/s3"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// These tests cover openspec/changes/sink-ca-trust SCT-5 (C1a): for the same snapshot sink and
// Secrets, the CLI path (cliBuildContext + withCLIBackend) fails with the same shared CA error
// as the operator's export (BuildContextFromSpec + Registry.NewBackend) and probe
// (RunConnectionTest), and the CLI never reaches the export callback.

const garbageCAPEM = "not a certificate"

// isolateParityAWSEnv points the AWS SDK's shared config at an empty file and clears the
// endpoint variables, so the host's AWS setup cannot change which check fires first.
func isolateParityAWSEnv(t *testing.T) {
	t.Helper()

	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("write empty AWS config: %v", err)
	}

	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_ENDPOINT_URL_S3", "")
	t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "")
}

func TestCAParity_C1a_cliAndOperatorShareError(t *testing.T) {
	isolateParityAWSEnv(t)

	caX := sinktlstest.NewCA(t, "X")

	secret := func(name string, data map[string]string) corev1.Secret {
		s := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: sinkNamespace}, Data: map[string][]byte{}}
		for k, v := range data {
			s.Data[k] = []byte(v)
		}

		return s
	}

	secrets := []corev1.Secret{
		secret("object-creds", map[string]string{"accessKeyID": "AKIDTEST", "secretAccessKey": "test-secret"}),
		secret("garbage-ca", map[string]string{"ca.crt": garbageCAPEM}),
		secret("ca-x", map[string]string{"ca.crt": string(caX.PEM)}),
	}

	creds := &kollectdevv1alpha1.SecretReference{Name: "object-creds"}
	bundle := func(pem string) *kollectdevv1alpha1.TLSSpec {
		return &kollectdevv1alpha1.TLSSpec{CABundle: []byte(pem)}
	}
	fromSecret := func(name string) *kollectdevv1alpha1.TLSSpec {
		return &kollectdevv1alpha1.TLSSpec{CASecretRef: &kollectdevv1alpha1.SecretReference{Name: name}}
	}

	const (
		httpsEndpoint = "https://sentinel-host.invalid/b"
		httpEndpoint  = "http://sentinel-host.invalid/b"
	)

	cases := []struct {
		name string
		spec kollectdevv1alpha1.KollectSinkSpec
		want error
	}{
		{"s3 garbage caBundle", kollectdevv1alpha1.KollectSinkSpec{
			Type: "s3", Endpoint: httpsEndpoint, SecretRef: creds, TLS: bundle(garbageCAPEM),
		}, sinktls.ErrNoCACertificate},
		{"s3 garbage caSecretRef", kollectdevv1alpha1.KollectSinkSpec{
			Type: "s3", Endpoint: httpsEndpoint, SecretRef: creds, TLS: fromSecret("garbage-ca"),
		}, sinktls.ErrNoCACertificate},
		{"s3 CA and http endpoint", kollectdevv1alpha1.KollectSinkSpec{
			Type: "s3", Endpoint: httpEndpoint, SecretRef: creds, TLS: fromSecret("ca-x"),
		}, s3sink.ErrCAPlainHTTP},
		{"gcs garbage caBundle", kollectdevv1alpha1.KollectSinkSpec{
			Type: "gcs", Endpoint: httpsEndpoint, SecretRef: creds, TLS: bundle(garbageCAPEM),
		}, sinktls.ErrNoCACertificate},
		{"gcs CA and http endpoint", kollectdevv1alpha1.KollectSinkSpec{
			Type: "gcs", Endpoint: httpEndpoint, SecretRef: creds, TLS: bundle(string(caX.PEM)),
		}, s3sink.ErrCAPlainHTTP},
	}

	objs := make([]corev1.Secret, len(secrets))
	copy(objs, secrets)

	builder := fake.NewClientBuilder().WithScheme(clientgoscheme.Scheme)
	for i := range objs {
		builder = builder.WithObjects(&objs[i])
	}

	operatorClient := builder.Build()
	registry := sink.NewRegistry()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Operator: the reconciler's build context, then the export backend and the probe.
			opBuild, err := sink.BuildContextFromSpec(ctx, operatorClient, tc.spec, sinkNamespace)
			if err != nil {
				t.Fatalf("operator build context: %v (the CA error belongs to the backend constructor)", err)
			}

			_, opExportErr := registry.NewBackend(tc.spec, opBuild)
			_, opProbeErr := sink.RunConnectionTest(ctx, tc.spec, opBuild)

			// CLI: the same steps runOneContext takes after collection.
			exportRan := false

			data, cliErr := ResolveSinkSecretData(tc.spec, secrets)
			if cliErr == nil {
				var cliBuild sink.BuildContext

				cliBuild, cliErr = cliBuildContext(ctx, tc.spec, data, secrets)
				if cliErr == nil {
					_, _, cliErr = withCLIBackend(ctx, "ctx-a", registry, tc.spec, cliBuild,
						func(sink.Backend) (int, []error) {
							exportRan = true

							return 0, nil
						})
				}
			}

			for path, err := range map[string]error{
				"operator export": opExportErr, "operator probe": opProbeErr, "CLI": cliErr,
			} {
				if !errors.Is(err, tc.want) {
					t.Errorf("%s error = %v, want it to wrap %v", path, err, tc.want)
				}

				if !kollecterrors.IsTerminal(err) {
					t.Errorf("%s error class = %s, want terminal: %v", path, kollecterrors.ClassOf(err), err)
				}
			}

			if exportRan {
				t.Error("the CLI export callback ran although the backend could not be built")
			}
		})
	}
}
