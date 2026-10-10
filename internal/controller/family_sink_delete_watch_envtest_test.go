// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/metrics"
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

		const controllerLabel = "kollectsnapshotsink-delete-watch"

		reconciles := func() float64 {
			return testutil.ToFloat64(metrics.ReconcileTotal.WithLabelValues(controllerLabel, metrics.ResultSuccess)) +
				testutil.ToFloat64(metrics.ReconcileTotal.WithLabelValues(controllerLabel, metrics.ResultFailure))
		}
		before := reconciles()

		sinkObj, spy := createPooledSink()

		// Cache sync does not mean the controller's Watches handler is attached:
		// the manager starts controllers after the caches sync, asynchronously,
		// and a delete issued before the handler registers is never replayed to
		// it (late handlers only get synthetic Adds for objects still in the
		// store). Workers start only after every source has synced, which is
		// after its handler was added, so an observed reconcile of the sink
		// proves the Watches handler is live before the delete.
		Eventually(reconciles).WithTimeout(30*time.Second).WithPolling(50*time.Millisecond).
			Should(BeNumerically(">", before), "controller never reconciled the sink: manager/controller did not start")

		Expect(k8sClient.Delete(watchCtx, sinkObj)).To(Succeed())

		// The delete event must reach the seam and Close the pooled backend —
		// the wiring this spec exists to pin.
		Eventually(spy.closes.Load).WithTimeout(30 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(int32(1)))
	})
})
