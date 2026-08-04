// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// newEnvtestManager builds a manager against the suite envtest apiserver with
// metrics/health probes disabled so parallel specs do not fight over ports.
func newEnvtestManager() manager.Manager {
	GinkgoHelper()

	mgr, err := manager.New(cfg, manager.Options{
		Scheme:                 scheme.Scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		LeaderElection:         false,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(mgr).NotTo(BeNil())

	return mgr
}

var _ = Describe("SetupWithManager wiring (envtest, COV-90-S07)", Ordered, func() {
	// Controller names are unique process-wide (prometheus metric labels). Register
	// once, then assert the duplicate-registration edge on the same manager.
	It("registers all six SetupWithManager wirings and rejects duplicates without panic", func() {
		mgr := newEnvtestManager()
		cl := mgr.GetClient()
		sch := mgr.GetScheme()

		Expect((&FamilySinkReconciler[kollectdevv1alpha1.KollectSnapshotSink, *kollectdevv1alpha1.KollectSnapshotSink]{
			Client: cl, Scheme: sch, Name: "kollectsnapshotsink",
		}).SetupWithManager(mgr)).To(Succeed())

		Expect((&KollectClusterInventoryReconciler{
			Client: cl, Scheme: sch,
		}).SetupWithManager(mgr)).To(Succeed())

		Expect((&KollectClusterTargetReconciler{
			Client: cl, Scheme: sch,
		}).SetupWithManager(mgr)).To(Succeed())

		Expect((&KollectConnectionTestReconciler{
			Client: cl, Scheme: sch,
		}).SetupWithManager(mgr)).To(Succeed())

		invRec := &KollectInventoryReconciler{Client: cl, Scheme: sch}
		Expect(invRec.SetupWithManager(mgr)).To(Succeed())

		Expect((&KollectTargetReconciler{
			Client: cl, Scheme: sch,
		}).SetupWithManager(mgr)).To(Succeed())

		var dupErr error
		Expect(func() {
			dupErr = invRec.SetupWithManager(mgr)
		}).NotTo(Panic())
		Expect(dupErr).To(HaveOccurred())
		// IndexField fails first on KollectInventory (wrapped); name collision is
		// also acceptable if the indexer path changes.
		Expect(dupErr.Error()).To(Or(
			ContainSubstring("index"),
			ContainSubstring("already exists"),
		))
	})

	It("mapClusterEventSinkToInventories enqueues bound cluster inventories only", func() {
		localScheme := runtime.NewScheme()
		Expect(kollectdevv1alpha1.AddToScheme(localScheme)).To(Succeed())

		bound := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "bound-event-cinv"},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				SinkNamespace: "ns-evt",
				EventSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "nats-bus"}},
			},
		}
		wrongNS := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "wrong-ns-event-cinv"},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				SinkNamespace: "other-ns",
				EventSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "nats-bus"}},
			},
		}
		unbound := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "unbound-event-cinv"},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				SinkNamespace: "ns-evt",
				EventSinkRefs: kollectdevv1alpha1.InventorySinkRefList{{Name: "other-bus"}},
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(localScheme).
			WithIndex(
				&kollectdevv1alpha1.KollectClusterInventory{},
				clusterInventorySinkFieldIndex, indexClusterInventorySinkBindings,
			).
			WithObjects(bound, wrongNS, unbound).
			Build()

		r := &KollectClusterInventoryReconciler{Client: fakeClient}
		sinkObj := &kollectdevv1alpha1.KollectEventSink{
			ObjectMeta: metav1.ObjectMeta{Name: "nats-bus", Namespace: "ns-evt"},
		}

		reqs := r.mapClusterEventSinkToInventories(context.Background(), sinkObj)
		names := make([]string, 0, len(reqs))
		for _, req := range reqs {
			names = append(names, req.Name)
		}
		sort.Strings(names)
		Expect(names).To(Equal([]string{"bound-event-cinv"}))
	})
})
