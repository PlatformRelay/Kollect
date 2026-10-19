//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// privateCAS3 is a path-style S3 endpoint for bucket "b" that accepts PutObject and records
// each object path it received.
type privateCAS3 struct {
	mu   sync.Mutex
	puts []string
}

func (f *privateCAS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	f.mu.Lock()
	f.puts = append(f.puts, r.URL.Path)
	f.mu.Unlock()

	w.Header().Set("ETag", `"etag"`)
	w.WriteHeader(http.StatusOK)
}

func (f *privateCAS3) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.puts...)
}

// C3 (openspec/changes/sink-ca-trust SCT-5): the CLI path (cliBuildContext with the CA from a
// config-dir Secret, withCLIBackend, ExportTargets) exports to an S3 endpoint whose certificate
// only the private CA signs, and refuses one signed by another CA.
func TestCATrustIntegration_cliExportsToPrivateCA(t *testing.T) {
	isolateParityAWSEnv(t)

	caX := sinktlstest.NewCA(t, "X")
	caY := sinktlstest.NewCA(t, "Y")

	store := collect.NewStore()
	store.Upsert(collect.Item{
		TargetNamespace: "default", TargetName: "inv",
		Namespace: "default", Name: "web", Kind: "Deployment", UID: "uid-1",
		Attributes: map[string]any{"image": "nginx:1.25"},
	})

	target := kollectdevv1alpha1.KollectTarget{}
	target.Namespace = "default"
	target.Name = "inv"
	targets := []kollectdevv1alpha1.KollectTarget{target}

	secrets := []corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "object-creds", Namespace: "default"},
			StringData: map[string]string{"accessKeyID": "AKIDTEST", "secretAccessKey": "test-secret"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "private-ca", Namespace: "default"},
			Data:       map[string][]byte{"ca.crt": caX.PEM},
		},
	}

	for _, tc := range []struct {
		name     string
		signer   *sinktlstest.CA
		exported bool
	}{
		{"server signed by the configured CA", caX, true},
		{"server signed by another CA", caY, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &privateCAS3{}
			srv := httptest.NewUnstartedServer(f)
			srv.TLS = &tls.Config{Certificates: []tls.Certificate{tc.signer.Server(t)}, MinVersion: tls.VersionTLS12}
			srv.StartTLS()
			t.Cleanup(srv.Close)

			spec := kollectdevv1alpha1.KollectSinkSpec{
				Type:         "s3",
				Endpoint:     srv.URL + "/b",
				PathTemplate: "inventory/{namespace}/{name}.json",
				SecretRef:    &kollectdevv1alpha1.SecretReference{Name: "object-creds"},
				TLS: &kollectdevv1alpha1.TLSSpec{
					CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "private-ca"},
				},
			}

			ctx := context.Background()

			data, err := ResolveSinkSecretData(spec, secrets)
			if err != nil {
				t.Fatalf("resolve sink secret: %v", err)
			}

			build, err := cliBuildContext(ctx, spec, data, secrets)
			if err != nil {
				t.Fatalf("cliBuildContext: %v", err)
			}

			exported, errs, err := withCLIBackend(ctx, "ctx-a", sink.NewRegistry(), spec, build,
				func(backend sink.Backend) (int, []error) {
					return ExportTargets(ctx, store, targets, backend, spec, "ctx-a", false)
				})
			if err != nil {
				t.Fatalf("withCLIBackend: %v", err)
			}

			if !tc.exported {
				var verr *tls.CertificateVerificationError
				if exported != 0 || len(errs) != 1 || !errors.As(errs[0], &verr) {
					t.Fatalf("exported %d, errors %v; want a certificate verification error and no export", exported, errs)
				}

				if got := f.recorded(); len(got) != 0 {
					t.Fatalf("server signed by another CA received %v", got)
				}

				return
			}

			if exported != 1 || len(errs) != 0 {
				t.Fatalf("exported %d, errors %v; want one export", exported, errs)
			}

			if got := f.recorded(); len(got) != 1 || got[0] != "/b/inventory/default/inv.json" {
				t.Fatalf("server recorded %v, want one PutObject of /b/inventory/default/inv.json", got)
			}
		})
	}
}
