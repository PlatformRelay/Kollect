// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink/cap"
	"github.com/platformrelay/kollect/internal/sink/git"
)

// Tests for SCR-1 (rotation rebuilds the pooled backend) and the metadata-only key of SCR-4.

const (
	rotationSinkNS   = "team-a"
	rotationSharedNS = "shared-creds"
)

// rotationBackend is the fake backend the recording registry hands out.
type rotationBackend struct {
	sinkType   string
	relational bool
	closes     atomic.Int32
}

func (b *rotationBackend) Type() string { return b.sinkType }

func (b *rotationBackend) Capabilities() Capabilities {
	if b.relational {
		return cap.RelationalStore()
	}

	return cap.SnapshotStore()
}

func (b *rotationBackend) Export(context.Context, []byte, string) error { return nil }

func (b *rotationBackend) ReleaseExport(context.Context, []string, git.ReleaseOptions) ([]string, error) {
	return nil, nil
}

func (b *rotationBackend) Close() error {
	b.closes.Add(1)

	return nil
}

// rotationRegistry records the BuildContext of every factory call.
type rotationRegistry struct {
	*Registry

	mu       sync.Mutex
	contexts []BuildContext
	backends []*rotationBackend
}

func newRotationRegistry(relational bool, sinkTypes ...string) *rotationRegistry {
	rr := &rotationRegistry{Registry: NewRegistry()}
	for _, sinkType := range sinkTypes {
		rr.Register(sinkType, func(_ kollectdevv1alpha1.KollectSinkSpec, bc BuildContext) (Backend, error) {
			b := &rotationBackend{sinkType: sinkType, relational: relational}
			rr.mu.Lock()
			rr.contexts = append(rr.contexts, bc)
			rr.backends = append(rr.backends, b)
			rr.mu.Unlock()

			return b, nil
		})
	}

	return rr
}

func (rr *rotationRegistry) builds() []BuildContext {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	return append([]BuildContext(nil), rr.contexts...)
}

func (rr *rotationRegistry) built() []*rotationBackend {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	return append([]*rotationBackend(nil), rr.backends...)
}

// rotationRef describes one SecretReference field: a spec of the type that reads it and the
// BuildContext field its value lands in.
type rotationRef struct {
	path  string
	key   string
	spec  func(ref *kollectdevv1alpha1.SecretReference) kollectdevv1alpha1.KollectSinkSpec
	value func(bc BuildContext) string
}

func rotationRefs() []rotationRef {
	secretData := func(bc BuildContext) string { return string(bc.SecretData["token"]) }
	databaseData := func(bc BuildContext) string { return string(bc.DatabaseSecretData["token"]) }

	return []rotationRef{
		{
			path: "secretRef", key: "token", value: secretData,
			spec: func(ref *kollectdevv1alpha1.SecretReference) kollectdevv1alpha1.KollectSinkSpec {
				return kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: ref}
			},
		},
		{
			path: "git.auth.secretRef", key: "token", value: secretData,
			spec: func(ref *kollectdevv1alpha1.SecretReference) kollectdevv1alpha1.KollectSinkSpec {
				return kollectdevv1alpha1.KollectSinkSpec{
					Type: "git",
					Git:  &kollectdevv1alpha1.GitSpec{Auth: &kollectdevv1alpha1.GitAuthSpec{SecretRef: ref}},
				}
			},
		},
		{
			path: "kafka.secretRef", key: "token", value: secretData,
			spec: func(ref *kollectdevv1alpha1.SecretReference) kollectdevv1alpha1.KollectSinkSpec {
				return kollectdevv1alpha1.KollectSinkSpec{Type: "kafka", Kafka: &kollectdevv1alpha1.KafkaSpec{SecretRef: ref}}
			},
		},
		{
			path: "nats.secretRef", key: "token", value: secretData,
			spec: func(ref *kollectdevv1alpha1.SecretReference) kollectdevv1alpha1.KollectSinkSpec {
				return kollectdevv1alpha1.KollectSinkSpec{Type: "nats", Nats: &kollectdevv1alpha1.NatsSpec{SecretRef: ref}}
			},
		},
		{
			path: "postgres.databaseRef", key: "token", value: databaseData,
			spec: func(ref *kollectdevv1alpha1.SecretReference) kollectdevv1alpha1.KollectSinkSpec {
				return kollectdevv1alpha1.KollectSinkSpec{
					Type: "postgres", Postgres: &kollectdevv1alpha1.PostgresSpec{DatabaseRef: ref, Table: "items"},
				}
			},
		},
		{
			path: "mongodb.databaseRef", key: "token", value: databaseData,
			spec: func(ref *kollectdevv1alpha1.SecretReference) kollectdevv1alpha1.KollectSinkSpec {
				return kollectdevv1alpha1.KollectSinkSpec{
					Type: "mongodb", MongoDB: &kollectdevv1alpha1.MongoSpec{DatabaseRef: ref, Database: "inv", Collection: "items"},
				}
			},
		},
		{
			path: "bigquery.secretRef", key: "token", value: databaseData,
			spec: func(ref *kollectdevv1alpha1.SecretReference) kollectdevv1alpha1.KollectSinkSpec {
				return kollectdevv1alpha1.KollectSinkSpec{
					Type: "bigquery", BigQuery: &kollectdevv1alpha1.BigQuerySpec{Project: "p", Dataset: "d", Table: "t", SecretRef: ref},
				}
			},
		},
		{
			path: "tls.caSecretRef", key: "ca.crt", value: func(bc BuildContext) string { return string(bc.CAPEM) },
			spec: func(ref *kollectdevv1alpha1.SecretReference) kollectdevv1alpha1.KollectSinkSpec {
				return kollectdevv1alpha1.KollectSinkSpec{Type: "s3", TLS: &kollectdevv1alpha1.TLSSpec{CASecretRef: ref}}
			},
		},
	}
}

func rotationScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	return scheme
}

func rotationSecret(namespace, name string, uid types.UID, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, UID: uid},
		Data:       data,
	}
}

func rotationClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()

	return fake.NewClientBuilder().WithScheme(rotationScheme(t)).WithObjects(objs...).Build()
}

// setSecretData rewrites the Secret's data (and optionally a label) through c.
func setSecretData(t *testing.T, c client.Client, namespace, name string, data map[string][]byte, labels map[string]string) {
	t.Helper()

	var s corev1.Secret
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, &s); err != nil {
		t.Fatalf("get secret %s/%s: %v", namespace, name, err)
	}
	s.Data = data
	if labels != nil {
		s.Labels = labels
	}
	if err := c.Update(context.Background(), &s); err != nil {
		t.Fatalf("update secret %s/%s: %v", namespace, name, err)
	}
}

// enableRotationPool turns the pool on for one test and restores the previous state.
func enableRotationPool(t *testing.T) {
	t.Helper()

	wasDisabled := backendPoolDisabled.Load()
	EnableBackendPoolForTest()
	ResetBackendPoolForTest()
	t.Cleanup(func() {
		ResetBackendPoolForTest()
		backendPoolDisabled.Store(wasDisabled)
	})
}

func mustAcquire(
	t *testing.T, c client.Client, reader client.Reader, reg *Registry, sinkName string, spec kollectdevv1alpha1.KollectSinkSpec,
) (Backend, func()) {
	t.Helper()

	b, release, err := acquireBackendWithReader(context.Background(), c, reader, reg, rotationSinkNS, sinkName,
		types.UID("uid-"+sinkName), spec)
	if err != nil {
		t.Fatalf("acquire %s: %v", sinkName, err)
	}

	return b, release
}

func tokenData(key, value string) map[string][]byte {
	return map[string][]byte{key: []byte(value)}
}

func TestAcquireBackend_rotatedSecretRebuilds(t *testing.T) {
	for _, ref := range rotationRefs() {
		for _, explicit := range []bool{false, true} {
			name := ref.path + "/namespace-omitted"
			secretNS := rotationSinkNS
			if explicit {
				name = ref.path + "/namespace-explicit"
				secretNS = rotationSharedNS
			}

			t.Run(name, func(t *testing.T) {
				enableRotationPool(t)

				secretRef := &kollectdevv1alpha1.SecretReference{Name: "creds"}
				if explicit {
					secretRef.Namespace = secretNS
				}
				spec := ref.spec(secretRef)
				c := rotationClient(t, rotationSecret(secretNS, "creds", "uid-creds", tokenData(ref.key, "sentinel-token-A")))
				reg := newRotationRegistry(false, spec.Type)

				first, release := mustAcquire(t, c, nil, reg.Registry, "rotating", spec)
				release()

				setSecretData(t, c, secretNS, "creds", tokenData(ref.key, "sentinel-token-B"), nil)

				second, release := mustAcquire(t, c, nil, reg.Registry, "rotating", spec)
				release()

				builds := reg.builds()
				if len(builds) != 2 {
					t.Fatalf("factory ran %d times, want 2 (a rotated %s must rebuild the backend)", len(builds), ref.path)
				}
				if got := ref.value(builds[1]); got != "sentinel-token-B" {
					t.Fatalf("second BuildContext carries %q, want the rotated value", got)
				}
				if first == second {
					t.Fatal("acquire after rotation returned the backend built from the old value")
				}
				if closes := reg.built()[0].closes.Load(); closes != 1 {
					t.Fatalf("old backend Close ran %d times, want 1", closes)
				}
			})
		}
	}
}

func TestAcquireBackend_readsSecretsUncached(t *testing.T) {
	pgSpec := kollectdevv1alpha1.KollectSinkSpec{
		Type:     "postgres",
		Postgres: &kollectdevv1alpha1.PostgresSpec{DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "db"}, Table: "items"},
	}
	gitSpec := kollectdevv1alpha1.KollectSinkSpec{
		Type:           "git",
		DeletionPolicy: kollectdevv1alpha1.DeletionPolicyRetain,
		Git: &kollectdevv1alpha1.GitSpec{Auth: &kollectdevv1alpha1.GitAuthSpec{
			SecretRef: &kollectdevv1alpha1.SecretReference{Name: "db"},
		}},
	}
	envelope, err := export.MarshalEnvelope([]collect.Item{}, export.Metadata{})
	if err != nil {
		t.Fatal(err)
	}
	const objectPath = "inventory/apps/demo.json"
	inventory := InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "apps", Name: "demo"}

	rows := []struct {
		name       string
		spec       kollectdevv1alpha1.KollectSinkSpec
		relational bool
		run        func(stale client.Client, fresh client.Reader, reg *Registry, spec kollectdevv1alpha1.KollectSinkSpec) error
	}{
		{
			name: "export", spec: pgSpec, relational: true,
			run: func(stale client.Client, fresh client.Reader, reg *Registry, spec kollectdevv1alpha1.KollectSinkSpec) error {
				_, err := RunExportEnvelope(ExportEnvelopeRequest{
					Ctx: context.Background(), Client: stale, SecretReader: fresh, Registry: reg,
					SinkNamespace: rotationSinkNS, SinkName: "uncached", SinkUID: "uid-uncached",
					ObjectPath: objectPath, Envelope: envelope, SinkSpec: spec,
				})

				return err
			},
		},
		{
			name: "retain cleanup (git release)", spec: gitSpec,
			run: func(stale client.Client, fresh client.Reader, reg *Registry, spec kollectdevv1alpha1.KollectSinkSpec) error {
				_, err := RunCleanupExport(CleanupExportRequest{
					Ctx: context.Background(), Client: stale, SecretReader: fresh, Registry: reg,
					SinkNamespace: rotationSinkNS, SinkName: "uncached", SinkUID: "uid-uncached",
					SinkSpec: spec, ObjectPath: objectPath, Inventory: inventory, Generation: 1,
				})

				return err
			},
		},
		{
			name: "delete cleanup (pruneRelationalExport)", spec: pgSpec, relational: true,
			run: func(stale client.Client, fresh client.Reader, reg *Registry, spec kollectdevv1alpha1.KollectSinkSpec) error {
				_, err := RunCleanupExport(CleanupExportRequest{
					Ctx: context.Background(), Client: stale, SecretReader: fresh, Registry: reg,
					SinkNamespace: rotationSinkNS, SinkName: "uncached", SinkUID: "uid-uncached",
					SinkSpec: spec, ObjectPath: objectPath, Inventory: inventory, Generation: 1,
				})

				return err
			},
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			enableRotationPool(t)

			// The cache still holds A; the API server already has B (a later resourceVersion).
			stale := rotationClient(t, rotationSecret(rotationSinkNS, "db", "uid-db", tokenData("token", "sentinel-token-A")))
			fresh := rotationClient(t, rotationSecret(rotationSinkNS, "db", "uid-db", tokenData("token", "sentinel-token-A")))
			setSecretData(t, fresh, rotationSinkNS, "db", tokenData("token", "sentinel-token-B"), nil)

			reg := newRotationRegistry(row.relational, row.spec.Type)
			if err := row.run(stale, fresh, reg.Registry, row.spec); err != nil {
				t.Fatalf("run: %v", err)
			}

			builds := reg.builds()
			if len(builds) == 0 {
				t.Fatal("no backend was built")
			}
			for i, bc := range builds {
				got := string(bc.SecretData["token"]) + string(bc.DatabaseSecretData["token"])
				if got != "sentinel-token-B" {
					t.Fatalf("build %d carries %q, want the uncached value (read through SecretReader)", i, got)
				}
			}
		})
	}
}

func TestAcquireBackend_rotateBack(t *testing.T) {
	enableRotationPool(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
	c := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-A")))
	reg := newRotationRegistry(false, "s3")

	want := make([]string, 0, 3)
	for _, value := range []string{"sentinel-token-A", "sentinel-token-B", "sentinel-token-A"} {
		if len(want) > 0 {
			setSecretData(t, c, rotationSinkNS, "creds", tokenData("token", value), nil)
		}
		want = append(want, value)
		_, release := mustAcquire(t, c, nil, reg.Registry, "rotate-back", spec)
		release()
	}

	builds := reg.builds()
	if len(builds) != 3 {
		t.Fatalf("factory ran %d times, want 3 (A, B, A)", len(builds))
	}
	for i, bc := range builds {
		if got := string(bc.SecretData["token"]); got != want[i] {
			t.Fatalf("build %d carries %q, want %q", i, got, want[i])
		}
	}
}

func TestAcquireBackend_sameDataNewVersionRebuilds(t *testing.T) {
	enableRotationPool(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
	c := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-A")))
	reg := newRotationRegistry(false, "s3")

	_, release := mustAcquire(t, c, nil, reg.Registry, "relabel", spec)
	release()

	setSecretData(t, c, rotationSinkNS, "creds", tokenData("token", "sentinel-token-A"), map[string]string{"rotated": "yes"})

	_, release = mustAcquire(t, c, nil, reg.Registry, "relabel", spec)
	release()

	if got := len(reg.builds()); got != 2 {
		t.Fatalf("factory ran %d times, want 2: a new resourceVersion with the same data rebuilds once", got)
	}
}

func TestAcquireBackend_unchangedSecretsReuse(t *testing.T) {
	enableRotationPool(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
	c := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-A")))
	reg := newRotationRegistry(false, "s3")

	first, release := mustAcquire(t, c, nil, reg.Registry, "steady", spec)
	release()
	second, release := mustAcquire(t, c, nil, reg.Registry, "steady", spec)
	release()

	if got := len(reg.builds()); got != 1 {
		t.Fatalf("factory ran %d times, want 1", got)
	}
	if first != second {
		t.Fatal("unchanged Secrets must reuse the pooled backend instance")
	}
}

func TestAcquireBackend_unrelatedSecretReuse(t *testing.T) {
	enableRotationPool(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
	c := rotationClient(t,
		rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-A")),
		rotationSecret(rotationSinkNS, "other", "uid-other", tokenData("token", "sentinel-token-A")),
	)
	reg := newRotationRegistry(false, "s3")

	first, release := mustAcquire(t, c, nil, reg.Registry, "unrelated", spec)
	release()
	setSecretData(t, c, rotationSinkNS, "other", tokenData("token", "sentinel-token-B"), nil)
	second, release := mustAcquire(t, c, nil, reg.Registry, "unrelated", spec)
	release()

	if got := len(reg.builds()); got != 1 {
		t.Fatalf("factory ran %d times, want 1 (an unreferenced Secret changed)", got)
	}
	if first != second {
		t.Fatal("an unrelated Secret change must reuse the pooled backend instance")
	}
}

// skewReader reports a stale resourceVersion on the first metadata GET, as if the Secret
// changed between the metadata read and the full read.
type skewReader struct {
	client.Reader
	metaRV string
}

func (r *skewReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := r.Reader.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	if m, ok := obj.(*metav1.PartialObjectMetadata); ok && r.metaRV != "" {
		m.ResourceVersion = r.metaRV
		r.metaRV = ""
	}

	return nil
}

func TestAcquireBackend_keyFromBuildSnapshot(t *testing.T) {
	enableRotationPool(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
	c := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-A")))
	setSecretData(t, c, rotationSinkNS, "creds", tokenData("token", "sentinel-token-B"), nil)

	var current corev1.Secret
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: rotationSinkNS, Name: "creds"}, &current); err != nil {
		t.Fatal(err)
	}
	reader := &skewReader{Reader: c, metaRV: "5"}
	reg := newRotationRegistry(false, "s3")

	_, release := mustAcquire(t, c, reader, reg.Registry, "snapshot", spec)
	release()

	specHash, err := specFingerprint(spec)
	if err != nil {
		t.Fatal(err)
	}
	want := poolEntryKey(specHash, []secretVersion{{
		Namespace: rotationSinkNS, Name: "creds", UID: current.UID, ResourceVersion: current.ResourceVersion,
	}})

	globalBackendPool.mu.Lock()
	entry, ok := globalBackendPool.entries[poolKeyForSink("uid-snapshot", rotationSinkNS, "snapshot")]
	var stored string
	if ok {
		stored = entry.key
	}
	globalBackendPool.mu.Unlock()
	if !ok {
		t.Fatal("no pooled entry after the first acquire")
	}
	if stored != want {
		t.Fatalf("stored key %q, want the key of the full read (resourceVersion %s), not of the metadata read",
			stored, current.ResourceVersion)
	}

	_, release = mustAcquire(t, c, reader, reg.Registry, "snapshot", spec)
	release()
	if got := len(reg.builds()); got != 1 {
		t.Fatalf("factory ran %d times, want 1: the second acquire at the same version is a hit", got)
	}
}

// secretReferenceFields returns the JSON path of every *SecretReference field reachable from t.
func secretReferenceFields(t reflect.Type, prefix string, out *[]string) {
	refType := reflect.TypeFor[*kollectdevv1alpha1.SecretReference]()
	for i := range t.NumField() {
		f := t.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		ft := f.Type
		if ft == refType {
			*out = append(*out, path)

			continue
		}
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct && ft.PkgPath() == refType.Elem().PkgPath() {
			secretReferenceFields(ft, path, out)
		}
	}
}

func TestReferencedSecretRefs_coversEverySecretReferenceField(t *testing.T) {
	var fields []string
	secretReferenceFields(reflect.TypeFor[kollectdevv1alpha1.KollectSinkSpec](), "", &fields)
	if len(fields) == 0 {
		t.Fatal("reflection found no *SecretReference field in KollectSinkSpec")
	}

	rows := map[string]rotationRef{}
	for _, ref := range rotationRefs() {
		rows[ref.path] = ref
	}

	for _, path := range fields {
		row, ok := rows[path]
		if !ok {
			t.Errorf("KollectSinkSpec.%s is a *SecretReference with no coverage row: add it to referencedSecretRefs and rotationRefs", path)

			continue
		}
		for _, namespace := range []string{"", rotationSharedNS} {
			spec := row.spec(&kollectdevv1alpha1.SecretReference{Name: "probe", Namespace: namespace})
			uses := referencedSecretRefs(spec, rotationSinkNS)
			want := secretUse{Namespace: rotationSinkNS, Name: "probe"}
			if namespace != "" {
				want.Namespace = namespace
			}
			found := false
			for _, use := range uses {
				if use == want {
					found = true
				}
			}
			if !found {
				t.Errorf("%s (namespace %q): referencedSecretRefs = %+v, want it to list %+v", path, namespace, uses, want)
			}
		}
	}
}

func TestSecretMeta_metadataOnly(t *testing.T) {
	ret := reflect.TypeOf(secretMeta).Out(0)
	if ret.Kind() != reflect.Slice || ret.Elem().Kind() != reflect.Struct {
		t.Fatalf("secretMeta returns %s, want a slice of a metadata struct", ret)
	}
	fieldNames := make([]string, 0, ret.Elem().NumField())
	for i := range ret.Elem().NumField() {
		f := ret.Elem().Field(i)
		fieldNames = append(fieldNames, f.Name)
		if k := f.Type.Kind(); k == reflect.Map || k == reflect.Slice || k == reflect.Interface || k == reflect.Pointer {
			t.Fatalf("secretMeta's element field %s has kind %s: it could carry Secret data", f.Name, k)
		}
	}
	if got := strings.Join(fieldNames, ","); got != "Namespace,Name,UID,ResourceVersion" {
		t.Fatalf("secretMeta element fields = %s, want Namespace,Name,UID,ResourceVersion", got)
	}

	uses := []secretUse{{Namespace: rotationSinkNS, Name: "creds"}}
	keyOf := func(c client.Reader) string {
		t.Helper()
		versions, err := secretMeta(context.Background(), c, uses)
		if err != nil {
			t.Fatalf("secretMeta: %v", err)
		}

		return poolEntryKey("spec", versions)
	}

	a := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-1", tokenData("token", "sentinel-token-1")))
	b := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-1", tokenData("token", "sentinel-token-2")))
	keyA := keyOf(a)
	if keyB := keyOf(b); keyA != keyB {
		t.Fatalf("equal metadata, different data: keys %q and %q differ (the key reads values)", keyA, keyB)
	}
	if strings.Contains(keyA, "sentinel") {
		t.Fatal("the key contains a Secret value")
	}

	recreated := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-2", tokenData("token", "sentinel-token-1")))
	if keyOf(recreated) == keyA {
		t.Fatal("a changed UID must change the key")
	}

	setSecretData(t, a, rotationSinkNS, "creds", tokenData("token", "sentinel-token-1"), map[string]string{"v": "2"})
	if keyOf(a) == keyA {
		t.Fatal("a changed resourceVersion must change the key")
	}
}

// The pool path keeps BuildContextFromSpec's precedence: CA rules before the credential lookup.
func TestAcquireBackend_caRulesPrecedeCredentialLookup(t *testing.T) {
	enableRotationPool(t)

	creds := &kollectdevv1alpha1.SecretReference{Name: "missing-creds"}
	spec := func(ca string) kollectdevv1alpha1.KollectSinkSpec {
		return kollectdevv1alpha1.KollectSinkSpec{
			Type: "s3", SecretRef: creds,
			TLS: &kollectdevv1alpha1.TLSSpec{CASecretRef: &kollectdevv1alpha1.SecretReference{Name: ca}},
		}
	}
	c := rotationClient(t, rotationSecret(rotationSinkNS, "no-ca-key", "uid-ca", tokenData("other", "x")))
	reg := newRotationRegistry(false, "s3")

	_, _, err := acquireBackendWithReader(context.Background(), c, nil, reg.Registry, rotationSinkNS, "ca-first", "uid-ca-first", spec("no-ca-key"))
	if !errors.Is(err, ErrCAKeyMissing) || !kollecterrors.IsTerminal(err) {
		t.Fatalf("CA Secret without a CA key, credentials missing: err = %v, want terminal %v", err, ErrCAKeyMissing)
	}

	_, _, err = acquireBackendWithReader(context.Background(), c, nil, reg.Registry, rotationSinkNS, "ca-first", "uid-ca-first", spec("missing-ca"))
	if err == nil || !strings.Contains(err.Error(), `"missing-ca"`) {
		t.Fatalf("CA Secret and credentials missing: err = %v, want the CA Secret named first", err)
	}
	if got := len(reg.builds()); got != 0 {
		t.Fatalf("factory ran %d times, want 0", got)
	}
}

// failingReader fails the metadata GET (failMeta) or the full GET (failFull) of one Secret name.
type failingReader struct {
	client.Reader
	failMeta, failFull string
}

var errSecretReadTimeout = errors.New("secret read timed out")

func (r failingReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	_, isMeta := obj.(*metav1.PartialObjectMetadata)
	if (isMeta && key.Name == r.failMeta) || (!isMeta && key.Name == r.failFull) {
		return errSecretReadTimeout
	}

	return r.Reader.Get(ctx, key, obj, opts...)
}

// Edge cases of the CA-first precedence on the pool path and the probe path.
func TestAcquireBackend_caRulesPrecedenceEdges(t *testing.T) {
	enableRotationPool(t)

	withCA := func(secretRef, ca string) kollectdevv1alpha1.KollectSinkSpec {
		return kollectdevv1alpha1.KollectSinkSpec{
			Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: secretRef},
			TLS: &kollectdevv1alpha1.TLSSpec{CASecretRef: &kollectdevv1alpha1.SecretReference{Name: ca}},
		}
	}
	c := rotationClient(t,
		rotationSecret(rotationSinkNS, "no-ca-key", "uid-ca", tokenData("other", "x")),
		rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-A")),
	)
	reg := newRotationRegistry(false, "s3")
	acquire := func(reader client.Reader, spec kollectdevv1alpha1.KollectSinkSpec) error {
		_, release, err := acquireBackendWithReader(context.Background(), c, reader, reg.Registry, rotationSinkNS, "ca-edges", "uid-ca-edges", spec)
		release()

		return err
	}

	// A caSecretRef without a name fails on the CA rule before a missing credential Secret.
	emptyName := withCA("missing-creds", "")
	if err := acquire(nil, emptyName); !errors.Is(err, ErrCAKeyMissing) || !kollecterrors.IsTerminal(err) {
		t.Fatalf("pool, empty caSecretRef name: err = %v, want terminal %v", err, ErrCAKeyMissing)
	}
	if _, err := BuildContextFromSpec(context.Background(), c, emptyName, rotationSinkNS); !errors.Is(err, ErrCAKeyMissing) {
		t.Fatalf("probe path, empty caSecretRef name: err = %v, want %v", err, ErrCAKeyMissing)
	}

	// The full read of the credential Secret fails after its metadata read succeeded.
	if err := acquire(failingReader{Reader: c, failFull: "creds"}, withCA("creds", "no-ca-key")); !errors.Is(err, ErrCAKeyMissing) {
		t.Fatalf("pool, failed full credential read: err = %v, want %v", err, ErrCAKeyMissing)
	}

	// A failed read of the CA Secret itself is returned as is, not replaced by a CA verdict.
	err := acquire(failingReader{Reader: c, failMeta: "no-ca-key"}, withCA("creds", "no-ca-key"))
	if !errors.Is(err, errSecretReadTimeout) || kollecterrors.IsTerminal(err) {
		t.Fatalf("pool, CA metadata read timed out: err = %v, want the read error, not terminal", err)
	}
	if got := len(reg.builds()); got != 0 {
		t.Fatalf("factory ran %d times, want 0", got)
	}
}
