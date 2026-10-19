// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/sink"
	"github.com/platformrelay/kollect/internal/validation"
)

// SCR-2, SCR-4 and SCR-7 on the production path: the inventory reconciler wired as cmd/main.go
// wires it (cached Client, SecretReader defaulted by SetupWithManager), the pool on, a
// recording registry, Reconcile driven by the spec.
var _ = Describe("sink secret deletion and zero-interval retry (envtest, SCR-2, SCR-4, SCR-7)", func() {
	type harness struct {
		r        *KollectInventoryReconciler
		rr       *secretRecordingRegistry
		recorder *record.FakeRecorder
		key      types.NamespacedName
		ns       string
	}

	// setup creates a namespace, a snapshot sink whose secretRef names "token", and an inventory
	// with the given interval, then starts a reconciler on a fresh manager's cache.
	setup := func(interval time.Duration) harness {
		GinkgoHelper()

		sink.EnableBackendPoolForTest()
		DeferCleanup(func() {
			sink.DisableBackendPoolForTest()
			sink.ResetBackendPoolForTest()
		})

		ns := "retire-" + testNameSuffix()
		nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		Expect(k8sClient.Create(ctx, nsObj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), nsObj) })

		snapshotSink := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{Name: "retire", Namespace: ns},
			Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: "s3",
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: "token"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, snapshotSink)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), snapshotSink) })

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "retire", Namespace: ns},
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				ExportMinInterval: &metav1.Duration{Duration: interval},
				SnapshotSinkRefs:  kollectdevv1alpha1.NewSinkRefList("retire"),
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), inv) })

		mgr, err := manager.New(cfg, manager.Options{
			Scheme:                 scheme.Scheme,
			Cache:                  cache.Options{},
			Metrics:                metricsserver.Options{BindAddress: "0"},
			HealthProbeBindAddress: "0",
			Controller:             config.Controller{SkipNameValidation: ptr.To(true)},
		})
		Expect(err).NotTo(HaveOccurred())

		store := collect.NewStore()
		store.Upsert(collect.Item{
			TargetNamespace: ns, TargetName: "demo-target", UID: "uid-retire",
			Namespace: ns, Name: "nginx", Version: "v1", Kind: "Deployment",
		})
		rr := newSecretRecordingRegistry("s3", "token")
		recorder := record.NewFakeRecorder(256)
		r := &KollectInventoryReconciler{
			Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Store: store, Registry: rr.Registry, Recorder: recorder,
		}
		Expect(r.SetupWithManager(mgr)).To(Succeed())

		cacheCtx, cancelCache := context.WithCancel(context.Background())
		DeferCleanup(cancelCache)
		go func() { _ = mgr.GetCache().Start(cacheCtx) }()
		Expect(mgr.GetCache().WaitForCacheSync(cacheCtx)).To(BeTrue())

		return harness{r: r, rr: rr, recorder: recorder, key: types.NamespacedName{Namespace: ns, Name: "retire"}, ns: ns}
	}

	createSecret := func(ns, value string) {
		GinkgoHelper()

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "token", Namespace: ns,
				Annotations: map[string]string{"last-applied": `{"data":{"token":"sentinel-key"}}`},
			},
			Data: map[string][]byte{"token": []byte(value)},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	}

	reconcileOnce := func(h harness) (ctrl.Result, error) {
		return h.r.Reconcile(context.Background(), reconcile.Request{NamespacedName: h.key})
	}

	degradedMessage := func(h harness) string {
		inv := &kollectdevv1alpha1.KollectInventory{}
		if err := k8sClient.Get(ctx, h.key, inv); err != nil {
			return ""
		}
		if cond := apimeta.FindStatusCondition(inv.Status.Conditions, conditionDegraded); cond != nil {
			return cond.Message
		}

		return ""
	}

	// reconcileUntilNotFound reconciles until the status names the missing Secret, and returns
	// the result of the reconcile that wrote it (not a status-conflict requeue).
	reconcileUntilNotFound := func(h harness) ctrl.Result {
		GinkgoHelper()

		notFound := `secret "token" not found in namespace "` + h.ns + `"`
		var last ctrl.Result
		Eventually(func(g Gomega) {
			res, err := reconcileOnce(h)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(res.RequeueAfter).NotTo(Equal(conflictRequeueAfter))
			g.Expect(degradedMessage(h)).To(ContainSubstring(notFound))
			last = res
		}).WithTimeout(15 * time.Second).WithPolling(250 * time.Millisecond).Should(Succeed())

		return last
	}

	reconcileUntilExports := func(h harness, exports int32) {
		GinkgoHelper()

		Eventually(func() int32 {
			_, _ = reconcileOnce(h)

			return h.rr.exports.Load()
		}).WithTimeout(15*time.Second).WithPolling(250*time.Millisecond).Should(BeNumerically(">=", exports),
			"the export never ran")
	}

	drainEvents := func(recorder *record.FakeRecorder) []string {
		var events []string
		for {
			select {
			case e := <-recorder.Events:
				events = append(events, e)
			default:
				return events
			}
		}
	}

	It("deleted secret fails the export with a redacted status", func() {
		h := setup(15 * time.Minute)
		createSecret(h.ns, "sentinel-token-1")

		reconcileUntilExports(h, 1)
		Expect(h.rr.recorded()).To(Equal([]string{"sentinel-token-1"}))

		Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "token", Namespace: h.ns}})).To(Succeed())

		// requestedAt forces the next export of an otherwise unchanged inventory.
		Eventually(func() error {
			inv := &kollectdevv1alpha1.KollectInventory{}
			if err := k8sClient.Get(ctx, h.key, inv); err != nil {
				return err
			}
			if inv.Annotations == nil {
				inv.Annotations = map[string]string{}
			}
			inv.Annotations[kollectdevv1alpha1.AnnotationRequestedAt] = time.Now().UTC().Format(time.RFC3339Nano)

			return k8sClient.Update(ctx, inv)
		}).WithTimeout(10 * time.Second).Should(Succeed())

		reconcileUntilNotFound(h)

		inv := &kollectdevv1alpha1.KollectInventory{}
		Expect(k8sClient.Get(ctx, h.key, inv)).To(Succeed())
		status, err := json.Marshal(inv.Status)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(status)).NotTo(ContainSubstring("sentinel-"), "a Secret value reached the inventory status")
		events := drainEvents(h.recorder)
		Expect(strings.Join(events, "\n")).To(ContainSubstring(`secret "token" not found`),
			"no Event names the missing Secret")
		for _, e := range events {
			Expect(e).NotTo(ContainSubstring("sentinel-"), "a Secret value reached an Event")
		}

		// Restore: the next retry exports from a new build, with no change to the sink.
		createSecret(h.ns, "sentinel-token-2")
		reconcileUntilExports(h, 2)
		Expect(h.rr.recorded()).To(Equal([]string{"sentinel-token-1", "sentinel-token-2"}))
	})

	It("zero-interval inventory retries a deleted secret", func() {
		h := setup(0)

		res := reconcileUntilNotFound(h)
		Expect(res.RequeueAfter).To(Equal(validation.ZeroIntervalWatchdog),
			"a 0s inventory whose export failed transiently must be retried after the watchdog")

		createSecret(h.ns, "sentinel-token-1")
		reconcileUntilExports(h, 1)
		Expect(h.rr.recorded()).To(Equal([]string{"sentinel-token-1"}))
	})

	It("unchanged zero-interval inventory is not re-exported", func() {
		h := setup(0)
		createSecret(h.ns, "sentinel-token-1")

		reconcileUntilExports(h, 1)
		Expect(h.rr.exports.Load()).To(BeEquivalentTo(1))

		for range 2 {
			_, err := reconcileOnce(h)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(h.rr.exports.Load()).To(BeEquivalentTo(1),
			"an unchanged, successfully exported 0s inventory was exported again")
	})
})
