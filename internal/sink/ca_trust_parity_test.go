// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/mongodb"
	"github.com/platformrelay/kollect/internal/sink/postgres"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
	"github.com/platformrelay/kollect/internal/sink/sinktls/sinktlstest"
)

// C2 (openspec/changes/sink-ca-trust SCT-5, "Probe and export agree"): for a database sink
// with a CA that cannot take effect, the operator's probe (RunConnectionTest) and its export
// backend (Registry.NewBackend) return the same terminal shared error, before any dial. The
// CLI loads no database sinks, so database parity is this pair.
const parityNamespace = "team-a"

func TestCAParity_C2_probeAndExportShareError(t *testing.T) {
	// pgx reads HOME and PG* variables during the parse; keep the host's out of it.
	t.Setenv("HOME", t.TempDir())

	for _, k := range []string{"PGSSLMODE", "PGSSLROOTCERT", "PGSERVICE", "PGSERVICEFILE", "PGSSLNEGOTIATION"} {
		t.Setenv(k, "")
	}

	caX := sinktlstest.NewCA(t, "X")

	pgSpec := kollectdevv1alpha1.KollectSinkSpec{
		Type: postgres.TypeName,
		Postgres: &kollectdevv1alpha1.PostgresSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "pg"},
			Table:       "inventory",
		},
	}
	mongoSpec := kollectdevv1alpha1.KollectSinkSpec{
		Type: mongodb.TypeName,
		MongoDB: &kollectdevv1alpha1.MongoSpec{
			DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "mongo"},
			Database:    "inventory",
			Collection:  "items",
		},
	}

	const (
		pgDSN    = "host=sentinel-host.invalid user=u dbname=d"
		mongoURI = "mongodb://sentinel-host.invalid/?tls=true"
	)

	cases := []struct {
		name  string
		spec  kollectdevv1alpha1.KollectSinkSpec
		db    map[string][]byte
		caPEM []byte
		want  error
	}{
		{"postgres explicit weak sslmode", pgSpec,
			map[string][]byte{"dsn": []byte(pgDSN + " sslmode=prefer")}, caX.PEM, postgres.ErrCAWeakSSLMode},
		{"postgres sslrootcert conflict", pgSpec,
			map[string][]byte{"dsn": []byte(pgDSN + " sslmode=verify-full sslrootcert=/x")}, caX.PEM, postgres.ErrCARootConflict},
		{"postgres garbage PEM", pgSpec,
			map[string][]byte{"dsn": []byte(pgDSN + " sslmode=verify-full")}, []byte("not a certificate"), sinktls.ErrNoCACertificate},
		{"mongodb tlsInsecure", mongoSpec,
			map[string][]byte{"uri": []byte(mongoURI + "&tlsInsecure=true")}, caX.PEM, mongodb.ErrCAInsecure},
		{"mongodb tlsCAFile", mongoSpec,
			map[string][]byte{"uri": []byte(mongoURI + "&tlsCAFile=/x")}, caX.PEM, mongodb.ErrCAFileConflict},
		{"mongodb garbage PEM", mongoSpec,
			map[string][]byte{"uri": []byte(mongoURI)}, []byte("not a certificate"), sinktls.ErrNoCACertificate},
	}

	registry := NewRegistry()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The build context comes from the operator's resolver, as both controllers build it:
			// the CA from spec.tls, the DSN/URI from the databaseRef Secret.
			spec := tc.spec
			spec.TLS = &kollectdevv1alpha1.TLSSpec{CABundle: tc.caPEM}

			name := "mongo"
			if spec.Postgres != nil {
				name = spec.Postgres.DatabaseRef.Name
			}

			c := fake.NewClientBuilder().WithScheme(clientgoscheme.Scheme).WithObjects(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: parityNamespace}, Data: tc.db,
			}).Build()

			build, err := BuildContextFromSpec(context.Background(), c, spec, parityNamespace)
			if err != nil {
				t.Fatalf("operator build context: %v", err)
			}

			backend, exportErr := registry.NewBackend(spec, build)
			if exportErr == nil {
				_ = CloseBackend(backend)
			}

			_, probeErr := RunConnectionTest(context.Background(), spec, build)

			for path, err := range map[string]error{"export": exportErr, "probe": probeErr} {
				if !errors.Is(err, tc.want) {
					t.Errorf("%s error = %v, want it to wrap %v", path, err, tc.want)
				}

				if !kollecterrors.IsTerminal(err) {
					t.Errorf("%s error class = %s, want terminal: %v", path, kollecterrors.ClassOf(err), err)
				}
			}
		})
	}
}
