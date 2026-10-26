// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink"
)

// syncedDebounceSink is one Postgres family sink of a syncedDebounceEnv. Each sink has its own
// table, so the registry selector can route it to its own recording backend, or to a failing
// backend while fail is set.
type syncedDebounceSink struct {
	name     string
	table    string
	interval *metav1.Duration
	recorder *recordingBackend
	fail     atomic.Bool
}

func (s *syncedDebounceSink) exportKey() string {
	return kollectdevv1alpha1.SinkFamilyDatabase + "/" + s.name
}

func (s *syncedDebounceSink) exports() int {
	s.recorder.mu.Lock()
	defer s.recorder.mu.Unlock()

	return len(s.recorder.exported)
}

type syncedDebounceEnv struct {
	ns         string
	store      *collect.Store
	sinks      []*syncedDebounceSink
	events     *record.FakeRecorder
	reconciler *KollectInventoryReconciler
	req        reconcile.Request
}

// newSyncedDebounceEnv creates a namespaced inventory with a 15-minute default interval bound
// to one Postgres sink per entry of refIntervals (nil keeps the inventory default).
func newSyncedDebounceEnv(prefix string, refIntervals ...*metav1.Duration) *syncedDebounceEnv {
	suffix := testNameSuffix()
	env := &syncedDebounceEnv{
		ns:     "default",
		store:  collect.NewStore(),
		events: record.NewFakeRecorder(32),
	}
	env.store.Upsert(collect.Item{
		TargetNamespace: env.ns, TargetName: "demo-target", UID: "uid-" + prefix + "-a",
		Namespace: env.ns, Name: "nginx", Version: "v1", Kind: "Deployment",
	})

	refs := make(kollectdevv1alpha1.InventorySinkRefList, 0, len(refIntervals))
	for i, interval := range refIntervals {
		s := &syncedDebounceSink{
			name:     prefix + "-pg-" + string(rune('a'+i)) + "-" + suffix,
			table:    prefix + "_items_" + string(rune('a'+i)),
			interval: interval,
			recorder: &recordingBackend{},
		}
		sinkObj, secret := createPostgresSinkFixtures(s.name, "pg-"+s.name, env.ns)
		sinkObj.Spec.Postgres.Table = s.table
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, sinkObj) })
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })
		DeferCleanup(func() { sink.EvictBackendPool(env.ns, s.name) })
		refs = append(refs, kollectdevv1alpha1.InventorySinkRef{Name: s.name, ExportMinInterval: interval})
		env.sinks = append(env.sinks, s)
	}

	defaultInterval := metav1.Duration{Duration: 15 * time.Minute}
	invName := prefix + "-inv-" + suffix
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Name: invName, Namespace: env.ns},
		Spec: kollectdevv1alpha1.KollectInventorySpec{
			ExportMinInterval: &defaultInterval,
			DatabaseSinkRefs:  refs,
		},
	}
	Expect(k8sClient.Create(ctx, inv)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, inv) })

	exportErr := kollecterrors.Transient(errors.New("sink unavailable"))
	reg := newPostgresRecordingRegistryWithSelector(func(spec kollectdevv1alpha1.KollectSinkSpec) (sink.Backend, error) {
		for _, s := range env.sinks {
			if spec.Postgres == nil || spec.Postgres.Table != s.table {
				continue
			}
			if s.fail.Load() {
				return &failingBackend{err: exportErr}, nil
			}

			return s.recorder, nil
		}

		return nil, errors.New("no sink for table")
	})
	env.reconciler = &KollectInventoryReconciler{
		Client: k8sClient, Scheme: k8sClient.Scheme(), Store: env.store, Registry: reg, Recorder: env.events,
	}
	env.req = reconcile.Request{NamespacedName: types.NamespacedName{Name: invName, Namespace: env.ns}}

	return env
}

func (env *syncedDebounceEnv) drainEvents() {
	for {
		select {
		case <-env.events.Events:
		default:
			return
		}
	}
}

func (env *syncedDebounceEnv) reconcileAndGet() *kollectdevv1alpha1.KollectInventory {
	env.drainEvents()
	_, err := env.reconciler.Reconcile(context.Background(), env.req)
	Expect(err).NotTo(HaveOccurred())
	got := &kollectdevv1alpha1.KollectInventory{}
	Expect(k8sClient.Get(ctx, env.req.NamespacedName, got)).To(Succeed())

	return got
}

func sinkSyncedCondition(exports []kollectdevv1alpha1.InventorySinkExportStatus, name string) *metav1.Condition {
	for i := range exports {
		if exports[i].Name == name {
			return apimeta.FindStatusCondition(exports[i].Conditions, conditionSinkSynced)
		}
	}

	return nil
}

func expectSinkSynced(
	exports []kollectdevv1alpha1.InventorySinkExportStatus,
	name string,
	status metav1.ConditionStatus,
	reason string,
) {
	GinkgoHelper()

	cond := sinkSyncedCondition(exports, name)
	Expect(cond).NotTo(BeNil(), "per-sink Synced condition of %s", name)
	Expect(cond.Status).To(Equal(status), "per-sink Synced status of %s (%s %q)", name, cond.Reason, cond.Message)
	Expect(cond.Reason).To(Equal(reason), "per-sink Synced reason of %s", name)
}

var _ = Describe("KollectInventory synced debounce (envtest)", func() {
	It("reports debounced sinks as synced (SDC-1, SDC-2)", func() {
		env := newSyncedDebounceEnv("sdc-all", nil, nil)

		env.reconcileAndGet()
		Expect(env.sinks[0].exports()).To(Equal(1))
		Expect(env.sinks[1].exports()).To(Equal(1))

		got := env.reconcileAndGet()
		Expect(env.sinks[0].exports()).To(Equal(1), "second pass must debounce sink A")
		Expect(env.sinks[1].exports()).To(Equal(1), "second pass must debounce sink B")

		for _, s := range env.sinks {
			expectSinkSynced(got.Status.SinkExports, s.exportKey(), metav1.ConditionTrue, kollectdevv1alpha1.ReasonDebounced)
		}
		agg := apimeta.FindStatusCondition(got.Status.Conditions, conditionSynced)
		Expect(agg).NotTo(BeNil())
		Expect(agg.Status).To(Equal(metav1.ConditionTrue), "aggregate Synced %s %q", agg.Reason, agg.Message)
		Expect(agg.Reason).To(Equal(reasonExported))
		Expect(agg.Message).To(Equal("2/2 sink(s) up to date"))
	})

	It("reports one exported and one debounced sink as synced (SDC-2)", func() {
		env := newSyncedDebounceEnv("sdc-mix", &metav1.Duration{Duration: time.Second}, nil)
		sinkA, sinkB := env.sinks[0], env.sinks[1]

		env.reconcileAndGet()
		Expect(sinkA.exports()).To(Equal(1))
		Expect(sinkB.exports()).To(Equal(1))

		time.Sleep(1100 * time.Millisecond)

		got := env.reconcileAndGet()
		Expect(sinkA.exports()).To(Equal(2), "sink A's 1s interval elapsed: re-exported")
		Expect(sinkB.exports()).To(Equal(1), "sink B's 15m interval: debounced")

		expectSinkSynced(got.Status.SinkExports, sinkA.exportKey(), metav1.ConditionTrue, reasonExported)
		expectSinkSynced(got.Status.SinkExports, sinkB.exportKey(), metav1.ConditionTrue, kollectdevv1alpha1.ReasonDebounced)
		agg := apimeta.FindStatusCondition(got.Status.Conditions, conditionSynced)
		Expect(agg).NotTo(BeNil())
		Expect(agg.Status).To(Equal(metav1.ConditionTrue), "aggregate Synced %s %q", agg.Reason, agg.Message)
		Expect(agg.Reason).To(Equal(reasonExported))
		Expect(agg.Message).To(Equal("2/2 sink(s) up to date"))
	})

	It("keeps Synced=False when a debounced sink sits next to a failed one (SDC-3)", func() {
		env := newSyncedDebounceEnv("sdc-fail", nil, nil)
		sinkA, sinkB := env.sinks[0], env.sinks[1]

		env.reconcileAndGet()
		Expect(sinkA.exports()).To(Equal(1))
		Expect(sinkB.exports()).To(Equal(1))

		// Payload C1 -> C2; B fails while A exports C2.
		env.store.Upsert(collect.Item{
			TargetNamespace: env.ns, TargetName: "demo-target", UID: "uid-sdc-fail-b",
			Namespace: env.ns, Name: "redis", Version: "v1", Kind: "Deployment",
		})
		sinkB.fail.Store(true)
		env.reconcileAndGet()
		Expect(sinkA.exports()).To(Equal(2))

		// Unchanged payload: A is debounced, B differs from its baseline C1, is retried and fails.
		got := env.reconcileAndGet()
		Expect(sinkA.exports()).To(Equal(2), "third pass must debounce sink A")

		agg := apimeta.FindStatusCondition(got.Status.Conditions, conditionSynced)
		Expect(agg).NotTo(BeNil())
		Expect(agg.Status).To(Equal(metav1.ConditionFalse), "aggregate Synced (control) %s %q", agg.Reason, agg.Message)
		expectSinkSynced(got.Status.SinkExports, sinkA.exportKey(), metav1.ConditionTrue, kollectdevv1alpha1.ReasonDebounced)
		expectSinkSynced(got.Status.SinkExports, sinkB.exportKey(), metav1.ConditionFalse, reasonExportFailed)
		ready := apimeta.FindStatusCondition(got.Status.Conditions, conditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse), "Ready %s %q", ready.Reason, ready.Message)
	})
})
