// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
	"github.com/platformrelay/kollect/internal/sink/cap"
)

// secretRecordingBackend is a relational fake: cleanup routes it through the empty-export prune.
// exports counts the Export calls of every backend one registry built.
type secretRecordingBackend struct {
	exports *atomic.Int32
}

func (secretRecordingBackend) Type() string                    { return "recording" }
func (secretRecordingBackend) Capabilities() sink.Capabilities { return cap.RelationalStore() }
func (b secretRecordingBackend) Export(context.Context, []byte, string) error {
	b.exports.Add(1)

	return nil
}

// secretRecordingRegistry records the database Secret value each backend build received.
type secretRecordingRegistry struct {
	*sink.Registry

	mu      sync.Mutex
	values  []string
	exports atomic.Int32
}

func newSecretRecordingRegistry(sinkType, key string) *secretRecordingRegistry {
	rr := &secretRecordingRegistry{Registry: sink.NewRegistry()}
	rr.Register(sinkType, func(_ kollectdevv1alpha1.KollectSinkSpec, bc sink.BuildContext) (sink.Backend, error) {
		rr.mu.Lock()
		rr.values = append(rr.values, string(bc.SecretData[key])+string(bc.DatabaseSecretData[key]))
		rr.mu.Unlock()

		return secretRecordingBackend{exports: &rr.exports}, nil
	})

	return rr
}

func (rr *secretRecordingRegistry) recorded() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	return append([]string(nil), rr.values...)
}

// offlineManager builds a manager that is never started and needs no API server: a static
// REST mapper answers the GVK lookups SetupWithManager makes.
func offlineManager(t *testing.T, scheme *runtime.Scheme) manager.Manager {
	t.Helper()

	mapper := meta.NewDefaultRESTMapper(nil)
	for gvk := range scheme.AllKnownTypes() {
		scope := meta.RESTScopeNamespace
		if strings.HasPrefix(gvk.Kind, "KollectCluster") || gvk.Kind == "Namespace" {
			scope = meta.RESTScopeRoot
		}
		mapper.Add(gvk, scope)
	}

	mgr, err := manager.New(&rest.Config{Host: "https://127.0.0.1:1"}, manager.Options{
		Scheme:                 scheme,
		MapperProvider:         func(*rest.Config, *http.Client) (meta.RESTMapper, error) { return mapper, nil },
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		Controller:             config.Controller{SkipNameValidation: ptr.To(true)},
	})
	if err != nil {
		t.Fatalf("manager.New: %v", err)
	}

	return mgr
}

func wiringScheme(t *testing.T) *runtime.Scheme {
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

func TestInventoryReconcilers_defaultSecretReader(t *testing.T) {
	scheme := wiringScheme(t)

	t.Run("SetupWithManager defaults SecretReader to the API reader", func(t *testing.T) {
		mgr := offlineManager(t, scheme)

		inv := &KollectInventoryReconciler{Client: mgr.GetClient(), Scheme: scheme}
		if err := inv.SetupWithManager(mgr); err != nil {
			t.Fatalf("KollectInventoryReconciler.SetupWithManager: %v", err)
		}
		if inv.SecretReader != mgr.GetAPIReader() {
			t.Fatalf("KollectInventoryReconciler.SecretReader = %T, want the manager's API reader", inv.SecretReader)
		}

		cluster := &KollectClusterInventoryReconciler{Client: mgr.GetClient(), Scheme: scheme}
		if err := cluster.SetupWithManager(mgr); err != nil {
			t.Fatalf("KollectClusterInventoryReconciler.SetupWithManager: %v", err)
		}
		if cluster.SecretReader != mgr.GetAPIReader() {
			t.Fatalf("KollectClusterInventoryReconciler.SecretReader = %T, want the manager's API reader", cluster.SecretReader)
		}
	})

	// The cleanup path (cleanupSinkExports) must receive the reconciler's SecretReader: the
	// cached Client holds the stale value A, the reader the current value B.
	newClients := func(t *testing.T, sinkNS string) (client.Client, client.Reader) {
		t.Helper()

		dbSink := &kollectdevv1alpha1.KollectDatabaseSink{
			ObjectMeta: metav1.ObjectMeta{Name: "pg", Namespace: sinkNS, UID: "uid-pg-sink"},
			Spec: kollectdevv1alpha1.KollectDatabaseSinkSpec{
				Type: kollectdevv1alpha1.SinkTypePostgres,
				Postgres: &kollectdevv1alpha1.PostgresSpec{
					DatabaseRef: &kollectdevv1alpha1.SecretReference{Name: "db"},
					Table:       "items",
				},
			},
		}
		secret := func(value string) *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: sinkNS, UID: "uid-db"},
				Data:       map[string][]byte{"token": []byte(value)},
			}
		}
		stale := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dbSink, secret("sentinel-token-A")).Build()
		fresh := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret("sentinel-token-A")).Build()
		current := secret("sentinel-token-A")
		if err := fresh.Get(context.Background(), types.NamespacedName{Namespace: sinkNS, Name: "db"}, current); err != nil {
			t.Fatal(err)
		}
		current.Data = map[string][]byte{"token": []byte("sentinel-token-B")}
		if err := fresh.Update(context.Background(), current); err != nil {
			t.Fatal(err)
		}

		return stale, fresh
	}

	assertUncached := func(t *testing.T, rr *secretRecordingRegistry) {
		t.Helper()

		got := rr.recorded()
		if len(got) == 0 {
			t.Fatal("no backend was built")
		}
		for i, v := range got {
			if v != "sentinel-token-B" {
				t.Fatalf("build %d carries %q, want the SecretReader's value", i, v)
			}
		}
	}

	enablePool := func(t *testing.T) {
		t.Helper()

		sink.EnableBackendPoolForTest()
		sink.ResetBackendPoolForTest()
		t.Cleanup(sink.DisableBackendPoolForTest)
	}

	t.Run("KollectInventory cleanup reads through SecretReader", func(t *testing.T) {
		enablePool(t)
		stale, fresh := newClients(t, "team-a")
		rr := newSecretRecordingRegistry(kollectdevv1alpha1.SinkTypePostgres, "token")
		r := &KollectInventoryReconciler{Client: stale, SecretReader: fresh, Scheme: scheme, Registry: rr.Registry}

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "team-a", Generation: 1},
			Spec:       kollectdevv1alpha1.KollectInventorySpec{DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("pg")},
		}
		if _, err := r.cleanupInventoryDeletion(context.Background(), inv); err != nil {
			t.Fatalf("cleanupInventoryDeletion: %v", err)
		}
		assertUncached(t, rr)
	})

	t.Run("KollectClusterInventory cleanup reads through SecretReader", func(t *testing.T) {
		enablePool(t)
		stale, fresh := newClients(t, "sinks")
		rr := newSecretRecordingRegistry(kollectdevv1alpha1.SinkTypePostgres, "token")
		r := &KollectClusterInventoryReconciler{Client: stale, SecretReader: fresh, Scheme: scheme, Registry: rr.Registry}

		inv := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "cinv", Generation: 1},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				SinkNamespace:    "sinks",
				DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("pg"),
			},
		}
		if _, err := r.cleanupClusterInventorySinks(context.Background(), inv); err != nil {
			t.Fatalf("cleanupClusterInventorySinks: %v", err)
		}
		assertUncached(t, rr)
	})

	t.Run("KollectClusterInventory export reads through SecretReader", func(t *testing.T) {
		enablePool(t)
		stale, fresh := newClients(t, "sinks")
		rr := newSecretRecordingRegistry(kollectdevv1alpha1.SinkTypePostgres, "token")
		r := &KollectClusterInventoryReconciler{Client: stale, SecretReader: fresh, Scheme: scheme, Registry: rr.Registry}

		inv := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "cinv", Generation: 1},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				SinkNamespace:    "sinks",
				DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("pg"),
			},
		}
		items := []collect.Item{{
			TargetNamespace: "apps", TargetName: "web", UID: "uid-item",
			Namespace: "apps", Name: "nginx", Version: "v1", Kind: "Deployment",
		}}
		outcome := r.exportClusterToSinks(context.Background(), logr.Discard(), inv, "cinv", "sinks", items, "checksum")
		if len(outcome.SinkExports) != 1 {
			t.Fatalf("outcome has %d sink exports, want 1", len(outcome.SinkExports))
		}
		assertUncached(t, rr)
	})
}
