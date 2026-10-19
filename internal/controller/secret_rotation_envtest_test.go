// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/operator"
	"github.com/platformrelay/kollect/internal/sink"
	"github.com/platformrelay/kollect/internal/validation"
)

// SCR-1 on the production path: the inventory reconciler wired as cmd/main.go wires it (cached
// Client, SecretReader defaulted by SetupWithManager), the pool on, a recording registry.
var _ = Describe("sink secret rotation (envtest, SCR-1)", func() {
	// startRotationReconciler sets the reconciler up on a fresh manager whose cache is limited by
	// cacheOpts, and starts only that cache, so Reconcile is driven by the spec alone.
	startRotationReconciler := func(cacheOpts cache.Options, store *collect.Store, reg *sink.Registry) *KollectInventoryReconciler {
		GinkgoHelper()

		mgr, err := manager.New(cfg, manager.Options{
			Scheme:                 scheme.Scheme,
			Cache:                  cacheOpts,
			Metrics:                metricsserver.Options{BindAddress: "0"},
			HealthProbeBindAddress: "0",
			Controller:             config.Controller{SkipNameValidation: ptr.To(true)},
		})
		Expect(err).NotTo(HaveOccurred())

		r := &KollectInventoryReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Store: store, Registry: reg}
		Expect(r.SetupWithManager(mgr)).To(Succeed())

		cacheCtx, cancelCache := context.WithCancel(context.Background())
		DeferCleanup(cancelCache)
		go func() { _ = mgr.GetCache().Start(cacheCtx) }()
		Expect(mgr.GetCache().WaitForCacheSync(cacheCtx)).To(BeTrue())

		return r
	}

	enablePool := func() {
		sink.EnableBackendPoolForTest()
		DeferCleanup(func() {
			sink.DisableBackendPoolForTest()
			sink.ResetBackendPoolForTest()
		})
	}

	createNamespace := func(name string) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), ns) })
	}

	storeWithItem := func(ns string) *collect.Store {
		store := collect.NewStore()
		store.Upsert(collect.Item{
			TargetNamespace: ns, TargetName: "demo-target", UID: "uid-rotation",
			Namespace: ns, Name: "nginx", Version: "v1", Kind: "Deployment",
		})

		return store
	}

	createSinkAndInventory := func(ns, name string, secretRef *kollectdevv1alpha1.SecretReference) types.NamespacedName {
		snapshotSink := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type:             "s3",
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{SecretRef: secretRef},
			},
		}
		Expect(k8sClient.Create(ctx, snapshotSink)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), snapshotSink) })

		interval := metav1.Duration{Duration: 15 * time.Minute}
		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				ExportMinInterval: &interval,
				SnapshotSinkRefs:  kollectdevv1alpha1.NewSinkRefList(name),
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), inv) })

		return types.NamespacedName{Namespace: ns, Name: name}
	}

	reconcileUntil := func(r *KollectInventoryReconciler, key types.NamespacedName, rr *secretRecordingRegistry, builds int) []string {
		GinkgoHelper()

		Eventually(func() []string {
			_, _ = r.Reconcile(context.Background(), reconcile.Request{NamespacedName: key})

			return rr.recorded()
		}).WithTimeout(15*time.Second).WithPolling(250*time.Millisecond).Should(HaveLen(builds),
			"the export never built a backend from the current Secret")

		return rr.recorded()
	}

	It("secret rotation takes effect on the next export", func() {
		enablePool()
		ns := "rotation-" + testNameSuffix()
		createNamespace(ns)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "token", Namespace: ns},
			Data:       map[string][]byte{"token": []byte("sentinel-token-1")},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		key := createSinkAndInventory(ns, "rotation", &kollectdevv1alpha1.SecretReference{Name: "token"})

		rr := newSecretRecordingRegistry("s3", "token")
		r := startRotationReconciler(cache.Options{}, storeWithItem(ns), rr.Registry)

		Expect(reconcileUntil(r, key, rr, 1)).To(Equal([]string{"sentinel-token-1"}))

		secret.Data = map[string][]byte{"token": []byte("sentinel-token-2")}
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())

		// requestedAt forces the next export of an otherwise unchanged inventory.
		Eventually(func() error {
			inv := &kollectdevv1alpha1.KollectInventory{}
			if err := k8sClient.Get(ctx, key, inv); err != nil {
				return err
			}
			if inv.Annotations == nil {
				inv.Annotations = map[string]string{}
			}
			inv.Annotations[kollectdevv1alpha1.AnnotationRequestedAt] = time.Now().UTC().Format(time.RFC3339Nano)

			return k8sClient.Update(ctx, inv)
		}).WithTimeout(10 * time.Second).Should(Succeed())

		Expect(reconcileUntil(r, key, rr, 2)).To(Equal([]string{"sentinel-token-1", "sentinel-token-2"}))
		Eventually(rr.exports.Load).WithTimeout(5*time.Second).Should(BeNumerically(">=", 2),
			"the rebuilt backend never exported")
	})

	It("cross-namespace secret under a restricted cache", func() {
		enablePool()
		suffix := testNameSuffix()
		invNS := "rotation-team-" + suffix
		credsNS := "rotation-creds-" + suffix
		createNamespace(invNS)
		createNamespace(credsNS)

		validation.SetAllowedSecretRefNamespaces([]string{credsNS})
		DeferCleanup(func() { validation.SetAllowedSecretRefNamespaces(nil) })

		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "token", Namespace: credsNS},
			Data:       map[string][]byte{"token": []byte("sentinel-token-shared")},
		})).To(Succeed())
		ref := &kollectdevv1alpha1.SecretReference{Name: "token", Namespace: credsNS}
		Expect(validation.ValidateSecretRefNamespaces(&kollectdevv1alpha1.KollectSinkSpec{Type: "s3", SecretRef: ref}, invNS)).
			To(BeEmpty(), "the Secret's namespace must be allowlisted for the scenario")
		key := createSinkAndInventory(invNS, "cross-ns", ref)

		rr := newSecretRecordingRegistry("s3", "token")
		r := startRotationReconciler(operator.CacheOptionsForWatchNamespaces([]string{invNS}), storeWithItem(invNS), rr.Registry)

		Expect(reconcileUntil(r, key, rr, 1)).To(Equal([]string{"sentinel-token-shared"}))
		Eventually(rr.exports.Load).WithTimeout(5*time.Second).Should(BeNumerically(">=", 1),
			"the backend built from the cross-namespace Secret never exported")
	})
})
