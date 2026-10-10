// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink"
)

// The envtest half of the family-sink delete watch (BEP-1): a real delete event
// must reach the pool's delete-hook seam through the Watches DeleteFunc the
// generic FamilySinkReconciler wires. The unit tests
// (family_sink_delete_watch_test.go) drive the hook body directly; this spec
// pins the wiring itself — deleting the Watches clause would leave the hook
// unreachable and this spec red.
var _ = Describe("family-sink delete watch (envtest, BEP-1)", func() {
	It("evicts the pooled backend when a family sink is deleted", func() {
		sink.EnableBackendPoolForTest()
		DeferCleanup(func() {
			sink.DisableBackendPoolForTest()
			sink.ResetBackendPoolForTest()
		})

		mgr := newEnvtestManager()
		Expect((&FamilySinkReconciler[kollectdevv1alpha1.KollectSnapshotSink, *kollectdevv1alpha1.KollectSnapshotSink]{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
			Name:   "kollectsnapshotsink-delete-watch",
		}).SetupWithManager(mgr)).To(Succeed())

		watchCtx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		go func() { _ = mgr.Start(watchCtx) }()
		Eventually(func() bool { return mgr.GetCache().WaitForCacheSync(watchCtx) }).
			WithTimeout(30 * time.Second).WithPolling(200 * time.Millisecond).Should(BeTrue())

		sinkObj := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "delete-watch-", Namespace: "default"},
			Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: "s3"},
		}
		Expect(k8sClient.Create(watchCtx, sinkObj)).To(Succeed())

		envelope, envErr := export.MarshalEnvelope([]collect.Item{}, export.Metadata{})
		Expect(envErr).NotTo(HaveOccurred())

		// createPooledSink creates a sink, waits until the manager's cache has
		// it and pools a spy backend for its UID.
		createPooledSink := func() (*kollectdevv1alpha1.KollectSnapshotSink, *deleteWatchSpyBackend) {
			obj := &kollectdevv1alpha1.KollectSnapshotSink{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "delete-watch-", Namespace: "default"},
				Spec:       kollectdevv1alpha1.KollectSnapshotSinkSpec{Type: "s3"},
			}
			Expect(k8sClient.Create(watchCtx, obj)).To(Succeed())
			Eventually(func() error {
				return mgr.GetClient().Get(watchCtx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Name}, &kollectdevv1alpha1.KollectSnapshotSink{})
			}).WithTimeout(30 * time.Second).Should(Succeed())

			spy := &deleteWatchSpyBackend{}
			reg := sink.NewRegistry()
			reg.Register("s3", func(_ kollectdevv1alpha1.KollectSinkSpec, _ sink.BuildContext) (sink.Backend, error) {
				return spy, nil
			})
			_, poolErr := sink.RunExportEnvelope(sink.ExportEnvelopeRequest{
				Ctx:           watchCtx,
				Client:        k8sClient,
				Registry:      reg,
				SinkNamespace: obj.Namespace,
				SinkName:      obj.Name,
				SinkUID:       obj.UID,
				Envelope:      envelope,
				SinkSpec:      obj.FamilySinkSpec(),
			})
			Expect(poolErr).NotTo(HaveOccurred())

			return obj, spy
		}

		// Cache sync does not mean the controller's Watches handler is attached:
		// the manager starts controllers (and their event sources) after the
		// caches sync, asynchronously. A delete issued before the handler is
		// registered is never replayed to it (late handlers only get synthetic
		// Adds for objects still in the store), so the spec would flake with
		// closes=0. Probe with throwaway sinks until one delete reaches the
		// seam: from then on the handler is provably live.
		watchLive := false
		for deadline := time.Now().Add(60 * time.Second); !watchLive && time.Now().Before(deadline); {
			probe, probeSpy := createPooledSink()
			Expect(k8sClient.Delete(watchCtx, probe)).To(Succeed())
			for end := time.Now().Add(2 * time.Second); !watchLive && time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
				watchLive = probeSpy.closes.Load() == 1
			}
		}
		Expect(watchLive).To(BeTrue(), "delete watch handler never became live")

		sinkObj, spy := createPooledSink()

		Expect(k8sClient.Delete(watchCtx, sinkObj)).To(Succeed())

		// The delete event must reach the seam and Close the pooled backend —
		// the wiring this spec exists to pin.
		Eventually(spy.closes.Load).WithTimeout(30 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(int32(1)))
	})
})
