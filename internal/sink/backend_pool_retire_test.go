// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/bigquery"
	"github.com/platformrelay/kollect/internal/sink/gcs"
	"github.com/platformrelay/kollect/internal/sink/git"
	"github.com/platformrelay/kollect/internal/sink/gitlab"
	kafkasink "github.com/platformrelay/kollect/internal/sink/kafka"
	"github.com/platformrelay/kollect/internal/sink/local"
	"github.com/platformrelay/kollect/internal/sink/mongodb"
	natssink "github.com/platformrelay/kollect/internal/sink/nats"
	"github.com/platformrelay/kollect/internal/sink/postgres"
	"github.com/platformrelay/kollect/internal/sink/s3"
)

// Tests for SCR-2 (a deleted or emptied referenced Secret fails, and the retire rules) and the
// SCR-4 sentinel checks on what this change adds.

func rotationRefByPath(t *testing.T, path string) rotationRef {
	t.Helper()

	for _, ref := range rotationRefs() {
		if ref.path == path {
			return ref
		}
	}
	t.Fatalf("no rotation reference %q", path)

	return rotationRef{}
}

// pooledEntryFor returns the pooled entry for the sink mustAcquire names, or nil.
func pooledEntryFor(sinkName string) *pooledEntry {
	globalBackendPool.mu.Lock()
	defer globalBackendPool.mu.Unlock()

	return globalBackendPool.entries[poolKeyForSink(types.UID("uid-"+sinkName), rotationSinkNS, sinkName)]
}

func acquireErr(
	c client.Client, reader client.Reader, reg *Registry, sinkName string, spec kollectdevv1alpha1.KollectSinkSpec,
) error {
	_, release, err := acquireBackendWithReader(context.Background(), c, reader, reg, rotationSinkNS, sinkName,
		types.UID("uid-"+sinkName), spec)
	release()

	return err
}

func deleteSecret(t *testing.T, c client.Client, namespace, name string) {
	t.Helper()

	if err := c.Delete(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}); err != nil {
		t.Fatalf("delete secret %s/%s: %v", namespace, name, err)
	}
}

func TestSecretMeta_notFoundIsTransient(t *testing.T) {
	c := rotationClient(t)

	_, err := secretMeta(context.Background(), c, []secretUse{{Namespace: rotationSinkNS, Name: "gone"}})
	if err == nil {
		t.Fatal("secretMeta of a missing Secret returned no error")
	}
	if want := `secret "gone" not found in namespace "team-a"`; err.Error() != want {
		t.Fatalf("error %q, want the plain %q", err, want)
	}
	if apierrors.IsNotFound(err) {
		t.Fatal("secretMeta returned the API NotFound as is; ClassOf classes it terminal")
	}
	wrapped := fmt.Errorf("acquire backend for %q: %w", "s", err)
	if class := kollecterrors.ClassOf(wrapped); class != kollecterrors.ClassTransient {
		t.Fatalf("ClassOf = %q, want %q: a deleted Secret must be retried", class, kollecterrors.ClassTransient)
	}
}

func TestAcquireBackend_deletedSecretFails(t *testing.T) {
	for _, path := range []string{"secretRef", "postgres.databaseRef", "tls.caSecretRef", "git.auth.secretRef"} {
		t.Run(path, func(t *testing.T) {
			enableRotationPool(t)

			ref := rotationRefByPath(t, path)
			spec := ref.spec(&kollectdevv1alpha1.SecretReference{Name: "creds"})
			c := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData(ref.key, "sentinel-token-A")))
			reg := newRotationRegistry(false, spec.Type)

			_, release := mustAcquire(t, c, nil, reg.Registry, "deleted", spec)
			release()

			deleteSecret(t, c, rotationSinkNS, "creds")

			err := acquireErr(c, nil, reg.Registry, "deleted", spec)
			if err == nil {
				t.Fatal("acquire after the Secret was deleted succeeded on the cached backend")
			}
			if want := `secret "creds" not found in namespace "team-a"`; !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not name the Secret and namespace (%s)", err, want)
			}
			if class := kollecterrors.ClassOf(err); class != kollecterrors.ClassTransient {
				t.Fatalf("ClassOf = %q, want transient", class)
			}
			if pooledEntryFor("deleted") != nil {
				t.Fatal("the pooled entry is still in place after its Secret was deleted")
			}
			if closes := reg.built()[0].closes.Load(); closes != 1 {
				t.Fatalf("retired backend Close ran %d times, want 1", closes)
			}
			if builds := len(reg.builds()); builds != 1 {
				t.Fatalf("factory ran %d times, want 1 (no build without the Secret)", builds)
			}
		})
	}
}

func TestAcquireBackend_emptiedSecretFails(t *testing.T) {
	rows := map[string]map[string][]byte{
		"data nil":          nil,
		"data empty map":    {},
		"every value empty": {"token": {}, "other": []byte("")},
	}
	for name, data := range rows {
		t.Run(name, func(t *testing.T) {
			enableRotationPool(t)

			spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
			c := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-A")))
			reg := newRotationRegistry(false, "s3")

			_, release := mustAcquire(t, c, nil, reg.Registry, "emptied", spec)
			release()

			setSecretData(t, c, rotationSinkNS, "creds", data, nil)

			err := acquireErr(c, nil, reg.Registry, "emptied", spec)
			if err == nil {
				t.Fatal("acquire accepted an empty Secret")
			}
			if want := `secret "creds" in namespace "team-a" has no data`; !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q, want it to contain %q", err, want)
			}
			if class := kollecterrors.ClassOf(err); class != kollecterrors.ClassTransient {
				t.Fatalf("ClassOf = %q, want transient", class)
			}
			if pooledEntryFor("emptied") != nil {
				t.Fatal("the pooled entry is still in place after its Secret was emptied")
			}
			if closes := reg.built()[0].closes.Load(); closes != 1 {
				t.Fatalf("retired backend Close ran %d times, want 1", closes)
			}
			if builds := len(reg.builds()); builds != 1 {
				t.Fatalf("factory ran %d times, want 1 (no build from an empty Secret)", builds)
			}
		})
	}
}

func TestAcquireBackend_restoredSecretRecovers(t *testing.T) {
	enableRotationPool(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
	c := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds-1", tokenData("token", "sentinel-token-A")))
	reg := newRotationRegistry(false, "s3")

	_, release := mustAcquire(t, c, nil, reg.Registry, "restored", spec)
	release()

	deleteSecret(t, c, rotationSinkNS, "creds")
	if err := acquireErr(c, nil, reg.Registry, "restored", spec); err == nil {
		t.Fatal("acquire succeeded with the Secret deleted")
	}

	if err := c.Create(context.Background(),
		rotationSecret(rotationSinkNS, "creds", "uid-creds-2", tokenData("token", "sentinel-token-B"))); err != nil {
		t.Fatal(err)
	}

	_, release = mustAcquire(t, c, nil, reg.Registry, "restored", spec)
	release()

	builds := reg.builds()
	if len(builds) != 2 {
		t.Fatalf("factory ran %d times, want 2 (a new build after the restore)", len(builds))
	}
	if got := string(builds[1].SecretData["token"]); got != "sentinel-token-B" {
		t.Fatalf("restored build carries %q, want the restored value", got)
	}
}

// failingMetaReader fails reads of one kind while fail is set: the metadata GET
// (*PartialObjectMetadata) or the full GET (*corev1.Secret); name limits it to one Secret.
type failingMetaReader struct {
	client.Reader
	fail atomic.Bool
	full bool
	name string
	err  error
}

func (r *failingMetaReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if r.fail.Load() && (r.name == "" || r.name == key.Name) {
		_, isMeta := obj.(*metav1.PartialObjectMetadata)
		_, isFull := obj.(*corev1.Secret)
		if (isMeta && !r.full) || (isFull && r.full) {
			return r.err
		}
	}

	return r.Reader.Get(ctx, key, obj, opts...)
}

func TestAcquireBackend_cancelledReadKeepsEntry(t *testing.T) {
	enableRotationPool(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
	c := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-A")))
	reader := &failingMetaReader{Reader: c, err: context.Canceled}
	reg := newRotationRegistry(false, "s3")

	first, release := mustAcquire(t, c, reader, reg.Registry, "shared", spec)
	release()

	reader.fail.Store(true)
	if err := acquireErr(c, reader, reg.Registry, "shared", spec); err == nil {
		t.Fatal("acquire succeeded although its metadata read was cancelled")
	}
	reader.fail.Store(false)

	if pooledEntryFor("shared") == nil {
		t.Fatal("a cancelled read retired the shared pooled backend")
	}
	if closes := reg.built()[0].closes.Load(); closes != 0 {
		t.Fatalf("pooled backend Close ran %d times after a cancelled read, want 0", closes)
	}

	second, release := mustAcquire(t, c, reader, reg.Registry, "shared", spec)
	release()
	if first != second {
		t.Fatal("the next acquire did not get the pooled backend")
	}
	if builds := len(reg.builds()); builds != 1 {
		t.Fatalf("factory ran %d times, want 1", builds)
	}
}

func TestAcquireBackend_observedChangeRetiresOnFailedRead(t *testing.T) {
	enableRotationPool(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
	c := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-A")))
	reader := &failingMetaReader{Reader: c, full: true, err: context.DeadlineExceeded}
	reg := newRotationRegistry(false, "s3")

	_, release := mustAcquire(t, c, reader, reg.Registry, "observed", spec)
	release()

	setSecretData(t, c, rotationSinkNS, "creds", tokenData("token", "sentinel-token-B"), nil)
	reader.fail.Store(true)

	if err := acquireErr(c, reader, reg.Registry, "observed", spec); err == nil {
		t.Fatal("acquire succeeded although the rebuild's full read timed out")
	}
	if pooledEntryFor("observed") != nil {
		t.Fatal("an observed Secret change left the old backend pooled after the rebuild read failed")
	}
	if closes := reg.built()[0].closes.Load(); closes != 1 {
		t.Fatalf("retired backend Close ran %d times, want 1", closes)
	}
}

func TestResolveSecret_emptyIsError(t *testing.T) {
	c := rotationClient(t, rotationSecret(rotationSinkNS, "creds", "uid-creds", map[string][]byte{"token": {}}))
	ref := &kollectdevv1alpha1.SecretReference{Name: "creds"}
	const want = `secret "creds" in namespace "team-a" has no data`

	_, err := ResolveSecret(context.Background(), c, ref, rotationSinkNS)
	if err == nil || err.Error() != want {
		t.Fatalf("ResolveSecret error = %v, want %q", err, want)
	}

	// The pool-disabled acquire builds through BuildContextFromSpec and must refuse it too.
	wasDisabled := backendPoolDisabled.Load()
	backendPoolDisabled.Store(true)
	t.Cleanup(func() { backendPoolDisabled.Store(wasDisabled) })

	reg := newRotationRegistry(false, "s3")
	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: ref}
	err = acquireErr(c, nil, reg.Registry, "disabled", spec)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("pool-disabled acquire error = %v, want %q", err, want)
	}
	if builds := len(reg.builds()); builds != 0 {
		t.Fatalf("factory ran %d times, want 0", builds)
	}
}

func TestS3_emptiedSecretNoAmbientCredentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDAMBIENTPROBE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "sentinel-key-ambient")

	for _, pooled := range []bool{true, false} {
		t.Run(fmt.Sprintf("pooled=%v", pooled), func(t *testing.T) {
			enableRotationPool(t)
			if !pooled {
				backendPoolDisabled.Store(true)
			}

			var calls atomic.Int32
			reg := NewRegistry()
			reg.Register("s3", func(spec kollectdevv1alpha1.KollectSinkSpec, bc BuildContext) (Backend, error) {
				calls.Add(1)

				return newS3Backend(spec, bc)
			})

			spec := kollectdevv1alpha1.KollectSinkSpec{
				Type: "s3", Endpoint: "s3://inventory-bucket/prefix",
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "s3-creds"},
			}
			c := rotationClient(t, rotationSecret(rotationSinkNS, "s3-creds", "uid-s3", map[string][]byte{
				"accessKeyID": []byte("AKIDSINK"), "secretAccessKey": []byte("sentinel-key"),
			}))
			setSecretData(t, c, rotationSinkNS, "s3-creds", map[string][]byte{}, nil)

			err := acquireErr(c, nil, reg, "s3-ambient", spec)
			if err == nil {
				t.Fatal("an emptied S3 Secret built a backend (it would use the ambient AWS credentials)")
			}
			if !strings.Contains(err.Error(), `secret "s3-creds" in namespace "team-a" has no data`) {
				t.Fatalf("error %q does not name the emptied Secret", err)
			}
			if n := calls.Load(); n != 0 {
				t.Fatalf("S3 factory ran %d times, want 0", n)
			}
		})
	}
}

// countingClient counts every Secret GET (full or metadata) made through it.
type countingClient struct {
	client.Client
	secretGets atomic.Int32
}

func (c *countingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	switch o := obj.(type) {
	case *corev1.Secret:
		c.secretGets.Add(1)
	case *metav1.PartialObjectMetadata:
		if o.Kind == "Secret" {
			c.secretGets.Add(1)
		}
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

func TestCleanup_streamSinkDeletedSecretRetained(t *testing.T) {
	specs := map[string]kollectdevv1alpha1.KollectSinkSpec{
		"kafka": {Type: "kafka", Kafka: &kollectdevv1alpha1.KafkaSpec{SecretRef: &kollectdevv1alpha1.SecretReference{Name: "gone"}}},
		"nats":  {Type: "nats", Nats: &kollectdevv1alpha1.NatsSpec{SecretRef: &kollectdevv1alpha1.SecretReference{Name: "gone"}}},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			enableRotationPool(t)

			c := &countingClient{Client: rotationClient(t)}
			reg := newRotationRegistry(false, spec.Type)

			outcome, err := RunCleanupExport(CleanupExportRequest{
				Ctx: context.Background(), Client: c, SecretReader: c, Registry: reg.Registry,
				SinkNamespace: rotationSinkNS, SinkName: "stream", SinkUID: "uid-stream", SinkSpec: spec,
				ObjectPath: "inventory/apps/demo.json", Generation: 1,
				Inventory: InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "apps", Name: "demo"},
			})
			if err != nil {
				t.Fatalf("cleanup of a stream sink with a deleted Secret failed: %v", err)
			}
			if outcome != CleanupRetained {
				t.Fatalf("outcome = %v, want CleanupRetained", outcome)
			}
			if n := c.secretGets.Load(); n != 0 {
				t.Fatalf("cleanup read %d Secrets, want 0", n)
			}
			if builds := len(reg.builds()); builds != 0 {
				t.Fatalf("factory ran %d times, want 0", builds)
			}
		})
	}
}

func TestCleanup_deletedGitAuthSecretFails(t *testing.T) {
	enableRotationPool(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{
		Type:           "git",
		DeletionPolicy: kollectdevv1alpha1.DeletionPolicyRetain,
		Git: &kollectdevv1alpha1.GitSpec{Auth: &kollectdevv1alpha1.GitAuthSpec{
			SecretRef: &kollectdevv1alpha1.SecretReference{Name: "git-auth"},
		}},
	}
	c := rotationClient(t)
	reg := newRotationRegistry(false, "git")

	_, err := RunCleanupExport(CleanupExportRequest{
		Ctx: context.Background(), Client: c, Registry: reg.Registry,
		SinkNamespace: rotationSinkNS, SinkName: "git", SinkUID: "uid-git", SinkSpec: spec,
		ObjectPath: "inventory/apps/demo.json", Generation: 1,
		Inventory: InventoryIdentity{Kind: InventoryKindNamespaced, Namespace: "apps", Name: "demo"},
	})
	if err == nil {
		t.Fatal("Git release with a deleted auth Secret reported success")
	}
	if want := `secret "git-auth" not found in namespace "team-a"`; !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not name the Secret (%s)", err, want)
	}
}

func TestStreamSinkTypes_matchRetainingBackends(t *testing.T) {
	// Typed nils: every backend's Capabilities returns a constant, and ExportCleaner is a type
	// assertion, so no backend is built and no Secret is read.
	zero := map[string]Backend{
		"git": (*git.Backend)(nil), "gitlab": (*gitlab.Backend)(nil), "s3": (*s3.Backend)(nil),
		"gcs": (*gcs.Backend)(nil), "postgres": (*postgres.Backend)(nil), "bigquery": (*bigquery.Backend)(nil),
		"mongodb": (*mongodb.Backend)(nil), "kafka": (*kafkasink.Backend)(nil), "nats": (*natssink.Backend)(nil),
		"local": (*local.Backend)(nil),
	}

	reg := NewRegistry()
	reg.mu.RLock()
	registered := make([]string, 0, len(reg.factories))
	for name := range reg.factories {
		registered = append(registered, name)
	}
	reg.mu.RUnlock()
	sort.Strings(registered)

	var retaining []string
	for _, name := range registered {
		b, ok := zero[name]
		if !ok {
			t.Fatalf("registered sink type %q has no row in this table", name)
		}
		_, cleaner := b.(ExportCleaner)
		if !b.Capabilities().SupportsDelete && !cleaner {
			retaining = append(retaining, name)
		}
	}
	if len(zero) != len(registered) {
		t.Fatalf("table has %d types, the registry %d", len(zero), len(registered))
	}

	set := make([]string, 0, len(streamSinkTypes))
	for name := range streamSinkTypes {
		set = append(set, name)
	}
	sort.Strings(set)

	if strings.Join(set, ",") != strings.Join(retaining, ",") {
		t.Fatalf("streamSinkTypes = %v, want the types cleanup retains without contact: %v", set, retaining)
	}
}

// captureSinkLog routes the pool's logger into a buffer for one test.
func captureSinkLog(t *testing.T) func() string {
	t.Helper()

	var mu sync.Mutex
	var buf bytes.Buffer
	logger := funcr.New(func(prefix, args string) {
		mu.Lock()
		defer mu.Unlock()
		buf.WriteString(prefix + " " + args + "\n")
	}, funcr.Options{Verbosity: 10})

	previous := sinkLog
	sinkLog = func() logr.Logger { return logger.WithName("sink") }
	t.Cleanup(func() { sinkLog = previous })

	return func() string {
		mu.Lock()
		defer mu.Unlock()

		return buf.String()
	}
}

// closeFailingBackend fails Close with an error, so the pool's close log line is captured too.
type closeFailingBackend struct{ rotationBackend }

func (b *closeFailingBackend) Close() error {
	b.closes.Add(1)

	return errors.New("close failed")
}

func TestRotationErrors_noSentinel(t *testing.T) {
	enableRotationPool(t)
	logs := captureSinkLog(t)

	spec := kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"}}
	secret := rotationSecret(rotationSinkNS, "creds", "uid-creds", map[string][]byte{
		"password": []byte("sentinel-pass"), "token": []byte("sentinel-token"),
	})
	secret.Annotations = map[string]string{"last-applied": `{"data":{"key":"sentinel-key"}}`}
	c := rotationClient(t, secret)

	reg := NewRegistry()
	reg.Register("s3", func(kollectdevv1alpha1.KollectSinkSpec, BuildContext) (Backend, error) {
		return &closeFailingBackend{rotationBackend{sinkType: "s3"}}, nil
	})

	var errs []error
	_, release := mustAcquire(t, c, nil, reg, "redacted", spec)
	release()

	// Rotate: same keys, new values, the annotation keeps its sentinel.
	setSecretData(t, c, rotationSinkNS, "creds", map[string][]byte{
		"password": []byte("sentinel-pass-2"), "token": []byte("sentinel-token-2"),
	}, nil)
	_, release = mustAcquire(t, c, nil, reg, "redacted", spec)
	release()

	poolDump := func() string {
		globalBackendPool.mu.Lock()
		defer globalBackendPool.mu.Unlock()

		var b strings.Builder
		for k, e := range globalBackendPool.entries {
			fmt.Fprintf(&b, "%s %+v\n", k, *e)
		}

		return b.String()
	}
	dumps := make([]string, 0, 3)
	dumps = append(dumps, poolDump())

	// Empty, then delete; each acquire must fail and its error is checked.
	setSecretData(t, c, rotationSinkNS, "creds", map[string][]byte{"password": {}}, nil)
	if err := acquireErr(c, nil, reg, "redacted", spec); err == nil {
		t.Fatal("acquire accepted the emptied Secret")
	} else {
		errs = append(errs, err)
	}
	dumps = append(dumps, poolDump())

	deleteSecret(t, c, rotationSinkNS, "creds")
	if err := acquireErr(c, nil, reg, "redacted", spec); err == nil {
		t.Fatal("acquire succeeded with the Secret deleted")
	} else {
		errs = append(errs, err)
	}
	dumps = append(dumps, poolDump())

	captured := logs()
	if !strings.Contains(captured, "rebuilt sink backend") || !strings.Contains(captured, "closeBackend failed") {
		t.Fatalf("log capture saw neither the rebuild nor the close line; captured:\n%s", captured)
	}

	texts := make([]string, 0, len(errs)+1+len(dumps))
	for _, err := range errs {
		texts = append(texts, err.Error())
	}
	texts = append(texts, captured)
	texts = append(texts, dumps...)
	for _, text := range texts {
		if strings.Contains(text, "sentinel-") {
			t.Fatalf("a sentinel value leaked: %s", text)
		}
	}
}

func TestAcquireBackend_caGoneBehindFailedCredentialReadRetires(t *testing.T) {
	for _, row := range []string{"deleted", "emptied"} {
		t.Run(row, func(t *testing.T) {
			enableRotationPool(t)

			spec := kollectdevv1alpha1.KollectSinkSpec{
				Type:      "s3",
				SecretRef: &kollectdevv1alpha1.SecretReference{Name: "creds"},
				TLS:       &kollectdevv1alpha1.TLSSpec{CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca"}},
			}
			c := rotationClient(t,
				rotationSecret(rotationSinkNS, "creds", "uid-creds", tokenData("token", "sentinel-token-A")),
				rotationSecret(rotationSinkNS, "ca", "uid-ca", tokenData("ca.crt", "sentinel-key-ca")),
			)
			// The credential metadata read times out, so only the CA-rules read sees the CA Secret.
			reader := &failingMetaReader{Reader: c, name: "creds", err: context.DeadlineExceeded}
			reg := newRotationRegistry(false, "s3")

			_, release := mustAcquire(t, c, reader, reg.Registry, "ca-gone", spec)
			release()

			want := `secret "ca" not found in namespace "team-a"`
			if row == "deleted" {
				deleteSecret(t, c, rotationSinkNS, "ca")
			} else {
				setSecretData(t, c, rotationSinkNS, "ca", map[string][]byte{}, nil)
				want = `secret "ca" in namespace "team-a" has no data`
			}
			reader.fail.Store(true)

			err := acquireErr(c, reader, reg.Registry, "ca-gone", spec)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("acquire error = %v, want the CA rule's %q", err, want)
			}
			if pooledEntryFor("ca-gone") != nil {
				t.Fatal("a CA Secret seen missing or empty by the CA-rules read left the pooled backend in place")
			}
			if closes := reg.built()[0].closes.Load(); closes != 1 {
				t.Fatalf("retired backend Close ran %d times, want 1", closes)
			}
		})
	}
}

// The design's compatibility table: an entirely empty CA Secret is the retryable "has no data"
// error; a CA Secret with other keys but no CA key keeps the terminal ErrCAKeyMissing.
func TestAcquireBackend_emptiedCASecretIsRetryable(t *testing.T) {
	rows := []struct {
		name     string
		data     map[string][]byte
		terminal bool
	}{
		{name: "entirely empty", data: map[string][]byte{}},
		{name: "other keys, no CA key", data: tokenData("unrelated", "x"), terminal: true},
	}
	for _, row := range rows {
		for _, pooled := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/pooled=%v", row.name, pooled), func(t *testing.T) {
				enableRotationPool(t)
				if !pooled {
					backendPoolDisabled.Store(true)
				}

				spec := kollectdevv1alpha1.KollectSinkSpec{
					Type: "s3",
					TLS:  &kollectdevv1alpha1.TLSSpec{CASecretRef: &kollectdevv1alpha1.SecretReference{Name: "ca"}},
				}
				c := rotationClient(t, rotationSecret(rotationSinkNS, "ca", "uid-ca", row.data))
				reg := newRotationRegistry(false, "s3")

				err := acquireErr(c, nil, reg.Registry, "ca-empty", spec)
				if err == nil {
					t.Fatal("acquire accepted a CA Secret without a CA key")
				}
				if row.terminal {
					if !errors.Is(err, ErrCAKeyMissing) || !kollecterrors.IsTerminal(err) {
						t.Fatalf("err = %v, want terminal %v", err, ErrCAKeyMissing)
					}

					return
				}
				if want := `secret "ca" in namespace "team-a" has no data`; !strings.Contains(err.Error(), want) {
					t.Fatalf("err = %v, want %q", err, want)
				}
				if errors.Is(err, ErrCAKeyMissing) || kollecterrors.ClassOf(err) != kollecterrors.ClassTransient {
					t.Fatalf("err = %v (class %s), want the transient has-no-data error", err, kollecterrors.ClassOf(err))
				}
			})
		}
	}
}
