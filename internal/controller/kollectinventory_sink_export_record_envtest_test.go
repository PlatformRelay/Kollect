// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

// sinkStatusTick is longer than the one-second precision of status timestamps, so a
// timestamp rewritten after it differs from the one written before it.
const sinkStatusTick = 1100 * time.Millisecond

// sinkExportRecord is the part of a sinkExports entry that only a successful export replaces
// (SDC-5), plus the per-sink Synced condition.
type sinkExportRecord struct {
	exportTime metav1.Time
	checksum   string
	synced     metav1.Condition
}

func sinkExportEntry(
	exports []kollectdevv1alpha1.InventorySinkExportStatus,
	name string,
) *kollectdevv1alpha1.InventorySinkExportStatus {
	for i := range exports {
		if exports[i].Name == name {
			return &exports[i]
		}
	}

	return nil
}

// readSinkExportRecord returns the entry's export record; the entry must have exported.
func readSinkExportRecord(exports []kollectdevv1alpha1.InventorySinkExportStatus, name string) sinkExportRecord {
	GinkgoHelper()

	entry := sinkExportEntry(exports, name)
	Expect(entry).NotTo(BeNil(), "sinkExports entry %s", name)
	Expect(entry.LastExportTime).NotTo(BeNil(), "lastExportTime of %s", name)
	Expect(entry.LastChecksum).NotTo(BeEmpty(), "lastChecksum of %s", name)
	synced := apimeta.FindStatusCondition(entry.Conditions, conditionSinkSynced)
	Expect(synced).NotTo(BeNil(), "per-sink Synced condition of %s", name)

	return sinkExportRecord{exportTime: *entry.LastExportTime, checksum: entry.LastChecksum, synced: *synced}
}

// expectSinkExportRecordKept asserts the entry still carries the last successful export's time
// and checksum (SDC-5).
func expectSinkExportRecordKept(
	exports []kollectdevv1alpha1.InventorySinkExportStatus,
	name string,
	want sinkExportRecord,
) {
	GinkgoHelper()

	entry := sinkExportEntry(exports, name)
	Expect(entry).NotTo(BeNil(), "sinkExports entry %s", name)
	Expect(entry.LastExportTime).NotTo(BeNil(), "lastExportTime of %s must keep the last successful export", name)
	Expect(entry.LastExportTime.Time).To(BeTemporally("==", want.exportTime.Time), "lastExportTime of %s", name)
	Expect(entry.LastChecksum).To(Equal(want.checksum), "lastChecksum of %s", name)
}

func (env *syncedDebounceEnv) changePayload(name string) {
	env.store.Upsert(collect.Item{
		TargetNamespace: env.ns, TargetName: "demo-target", UID: "uid-" + name,
		Namespace: env.ns, Name: name, Version: "v1", Kind: "Deployment",
	})
}

func (env *syncedDebounceEnv) requestExport() {
	GinkgoHelper()

	Eventually(func() error {
		inv := &kollectdevv1alpha1.KollectInventory{}
		if err := k8sClient.Get(ctx, env.req.NamespacedName, inv); err != nil {
			return err
		}
		if inv.Annotations == nil {
			inv.Annotations = map[string]string{}
		}
		inv.Annotations[kollectdevv1alpha1.AnnotationRequestedAt] = time.Now().UTC().Format(time.RFC3339Nano)

		return k8sClient.Update(ctx, inv)
	}).WithTimeout(10 * time.Second).Should(Succeed())
}

func aggregateSynced(conditions []metav1.Condition) metav1.Condition {
	GinkgoHelper()

	agg := apimeta.FindStatusCondition(conditions, conditionSynced)
	Expect(agg).NotTo(BeNil(), "aggregate Synced condition")

	return *agg
}

var _ = Describe("KollectInventory per-sink export record (envtest)", func() {
	It("keeps the per-sink export record across a debounced pass (SDC-5)", func() {
		env := newSyncedDebounceEnv("sdc-keep", nil)
		only := env.sinks[0]

		first := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1))
		before := readSinkExportRecord(first.Status.SinkExports, only.exportKey())

		time.Sleep(sinkStatusTick)

		got := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1), "second pass must debounce the sink")
		expectSinkExportRecordKept(got.Status.SinkExports, only.exportKey(), before)
		synced := sinkSyncedCondition(got.Status.SinkExports, only.exportKey())
		Expect(synced).NotTo(BeNil())
		Expect(synced.LastTransitionTime.Time).To(BeTemporally("==", before.synced.LastTransitionTime.Time),
			"per-sink Synced lastTransitionTime across a debounced pass")
	})

	It("keeps the last successful export record across a partly failed pass (SDC-5, SDC-3)", func() {
		env := newSyncedDebounceEnv("sdc-part", nil, nil)
		sinkA, sinkB := env.sinks[0], env.sinks[1]

		first := env.reconcileAndGet()
		Expect(sinkA.exports()).To(Equal(1))
		Expect(sinkB.exports()).To(Equal(1))
		beforeB := readSinkExportRecord(first.Status.SinkExports, sinkB.exportKey())

		time.Sleep(sinkStatusTick)

		env.changePayload("redis")
		sinkB.fail.Store(true)
		got := env.reconcileAndGet()
		Expect(sinkA.exports()).To(Equal(2), "sink A exports the changed payload")

		expectSinkExportRecordKept(got.Status.SinkExports, sinkB.exportKey(), beforeB)
		expectSinkSynced(got.Status.SinkExports, sinkB.exportKey(), metav1.ConditionFalse, reasonExportFailed)
	})

	It("marks the only failing sink as not synced (SDC-3, SDC-5)", func() {
		env := newSyncedDebounceEnv("sdc-only", nil)
		only := env.sinks[0]

		first := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1))
		before := readSinkExportRecord(first.Status.SinkExports, only.exportKey())

		time.Sleep(sinkStatusTick)

		env.changePayload("redis")
		only.fail.Store(true)
		got := env.reconcileAndGet()

		expectSinkSynced(got.Status.SinkExports, only.exportKey(), metav1.ConditionFalse, reasonExportFailed)
		expectSinkExportRecordKept(got.Status.SinkExports, only.exportKey(), before)
		agg := aggregateSynced(got.Status.Conditions)
		Expect(agg.Status).To(Equal(metav1.ConditionFalse), "aggregate Synced %s %q", agg.Reason, agg.Message)
	})

	It("writes no status on a repeated debounced reconcile, database sink (SDC-5, preview path)", func() {
		env := newSyncedDebounceEnv("sdc-quiet", nil)
		only := env.sinks[0]

		env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1))
		debounced := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1), "second pass must debounce the sink")

		time.Sleep(sinkStatusTick)

		got := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1), "third pass must debounce the sink")
		Expect(got.ResourceVersion).To(Equal(debounced.ResourceVersion),
			"a repeated debounced reconcile must not change the status")
		Expect(env.events.Events).To(BeEmpty(), "a repeated debounced reconcile must not record an Event")
	})

	It("writes no status on a repeated debounced reconcile, snapshot sink (SDC-5, full path)", func() {
		suffix := testNameSuffix()
		ns := "default"
		name := "sdc-quiet-snap-" + suffix

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name + "-token", Namespace: ns},
			Data:       map[string][]byte{"token": []byte("sentinel-token")},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), secret) })
		snapshotSink := &kollectdevv1alpha1.KollectSnapshotSink{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: kollectdevv1alpha1.KollectSnapshotSinkSpec{
				Type: "s3",
				SinkCommonFields: kollectdevv1alpha1.SinkCommonFields{
					SecretRef: &kollectdevv1alpha1.SecretReference{Name: secret.Name},
				},
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

		store := collect.NewStore()
		store.Upsert(collect.Item{
			TargetNamespace: ns, TargetName: "demo-target", UID: "uid-" + name,
			Namespace: ns, Name: "nginx", Version: "v1", Kind: "Deployment",
		})
		rr := newSecretRecordingRegistry("s3", "token")
		events := record.NewFakeRecorder(32)
		r := &KollectInventoryReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Store: store, Registry: rr.Registry, Recorder: events,
		}
		req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}}
		reconcileAndGet := func() *kollectdevv1alpha1.KollectInventory {
			GinkgoHelper()

			for len(events.Events) > 0 {
				<-events.Events
			}
			_, err := r.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			got := &kollectdevv1alpha1.KollectInventory{}
			Expect(k8sClient.Get(ctx, req.NamespacedName, got)).To(Succeed())

			return got
		}

		reconcileAndGet()
		Expect(rr.exports.Load()).To(Equal(int32(1)))
		debounced := reconcileAndGet()
		Expect(rr.exports.Load()).To(Equal(int32(1)), "second pass must debounce the sink")
		expectSinkSynced(debounced.Status.SinkExports, kollectdevv1alpha1.SinkFamilySnapshot+"/"+name,
			metav1.ConditionTrue, kollectdevv1alpha1.ReasonDebounced)

		time.Sleep(sinkStatusTick)

		got := reconcileAndGet()
		Expect(rr.exports.Load()).To(Equal(int32(1)), "third pass must debounce the sink")
		Expect(got.ResourceVersion).To(Equal(debounced.ResourceVersion),
			"a repeated debounced reconcile must not change the status")
		Expect(events.Events).To(BeEmpty(), "a repeated debounced reconcile must not record an Event")
	})

	It("export pass and debounce pass give the same aggregate (SDC-5)", func() {
		env := newSyncedDebounceEnv("sdc-same", nil)
		only := env.sinks[0]

		env.reconcileAndGet()
		Expect(only.exports()).To(Equal(1))

		time.Sleep(sinkStatusTick)

		env.requestExport()
		exported := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(2), "requestedAt forces an export of the unchanged payload")
		aggExported := aggregateSynced(exported.Status.Conditions)
		perSinkExported := sinkSyncedCondition(exported.Status.SinkExports, only.exportKey())
		Expect(perSinkExported).NotTo(BeNil())

		time.Sleep(sinkStatusTick)

		debounced := env.reconcileAndGet()
		Expect(only.exports()).To(Equal(2), "the next pass must debounce the sink")
		aggDebounced := aggregateSynced(debounced.Status.Conditions)
		perSinkDebounced := sinkSyncedCondition(debounced.Status.SinkExports, only.exportKey())
		Expect(perSinkDebounced).NotTo(BeNil())

		Expect(aggDebounced.Status).To(Equal(aggExported.Status), "aggregate Synced status")
		Expect(aggDebounced.Reason).To(Equal(aggExported.Reason), "aggregate Synced reason")
		Expect(aggDebounced.Message).To(Equal(aggExported.Message), "aggregate Synced message")
		Expect(aggDebounced.LastTransitionTime.Time).To(BeTemporally("==", aggExported.LastTransitionTime.Time),
			"aggregate Synced lastTransitionTime")
		Expect(perSinkExported.Status).To(Equal(metav1.ConditionTrue), "per-sink Synced after the export pass")
		Expect(perSinkDebounced.Status).To(Equal(metav1.ConditionTrue), "per-sink Synced after the debounce pass")
		Expect(perSinkDebounced.LastTransitionTime.Time).To(BeTemporally("==", perSinkExported.LastTransitionTime.Time),
			"per-sink Synced lastTransitionTime across the export and the debounce pass")
	})

	It("moves lastTransitionTime only on a status change (SDC-3, SDC-5)", func() {
		env := newSyncedDebounceEnv("sdc-ltt", nil)
		only := env.sinks[0]

		type pass struct {
			aggStatus  metav1.ConditionStatus
			aggLTT     time.Time
			sinkStatus metav1.ConditionStatus
			sinkLTT    time.Time
		}
		observe := func(got *kollectdevv1alpha1.KollectInventory) pass {
			GinkgoHelper()

			agg := aggregateSynced(got.Status.Conditions)
			perSink := sinkSyncedCondition(got.Status.SinkExports, only.exportKey())
			Expect(perSink).NotTo(BeNil())

			return pass{agg.Status, agg.LastTransitionTime.Time, perSink.Status, perSink.LastTransitionTime.Time}
		}

		upToDate := observe(env.reconcileAndGet())
		Expect(only.exports()).To(Equal(1))

		time.Sleep(sinkStatusTick)
		env.changePayload("redis")
		only.fail.Store(true)
		failed := observe(env.reconcileAndGet())

		time.Sleep(sinkStatusTick)
		only.fail.Store(false)
		recovered := observe(env.reconcileAndGet())
		Expect(only.exports()).To(Equal(2), "the recovered sink exports the changed payload")

		time.Sleep(sinkStatusTick)
		debounced := observe(env.reconcileAndGet())
		Expect(only.exports()).To(Equal(2), "the next pass must debounce the sink")

		Expect([]metav1.ConditionStatus{upToDate.aggStatus, failed.aggStatus, recovered.aggStatus, debounced.aggStatus}).
			To(Equal([]metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse, metav1.ConditionTrue, metav1.ConditionTrue}),
				"aggregate Synced status per pass")
		// Control on the G1 tree: the aggregate condition persists across passes.
		Expect(map[string]bool{
			"aggregate moved at the failure":       !failed.aggLTT.Equal(upToDate.aggLTT),
			"aggregate moved at the recovery":      !recovered.aggLTT.Equal(failed.aggLTT),
			"aggregate kept at the debounced pass": debounced.aggLTT.Equal(recovered.aggLTT),
		}).To(HaveEach(BeTrue()), "aggregate Synced lastTransitionTime per pass")
		Expect(map[string]bool{
			"per-sink moved at the failure":       !failed.sinkLTT.Equal(upToDate.sinkLTT),
			"per-sink moved at the recovery":      !recovered.sinkLTT.Equal(failed.sinkLTT),
			"per-sink kept at the debounced pass": debounced.sinkLTT.Equal(recovered.sinkLTT),
		}).To(HaveEach(BeTrue()), "per-sink Synced lastTransitionTime per pass")
		Expect([]metav1.ConditionStatus{upToDate.sinkStatus, failed.sinkStatus, recovered.sinkStatus, debounced.sinkStatus}).
			To(Equal([]metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse, metav1.ConditionTrue, metav1.ConditionTrue}),
				"per-sink Synced status per pass")
	})
})
