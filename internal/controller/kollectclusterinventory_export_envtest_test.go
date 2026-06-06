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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink"
)

var _ = Describe("KollectClusterInventory export (envtest)", func() {
	const tenantLabel = "kollect.dev/tenant"

	var (
		suffix         string
		targetName     string
		profileName    string
		inventoryName  string
		sinkName       string
		nsMatched      string
		tenantLabelVal string
		engineCtx      context.Context
		engineCancel   context.CancelFunc
		engine         *collect.Engine
		store          *collect.Store
		kubeClient     kubernetes.Interface
	)

	BeforeEach(func() {
		suffix = testNameSuffix()
		targetName = "export-target-" + suffix
		profileName = "export-profile-" + suffix
		inventoryName = "export-inv-" + suffix
		sinkName = "export-pg-" + suffix
		nsMatched = "cluster-export-a-" + suffix
		tenantLabelVal = "export-" + suffix

		var err error
		kubeClient, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		dyn, err := dynamic.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		store = collect.NewStore()
		engine, err = collect.NewEngine(dyn, kubeClient, store)
		Expect(err).NotTo(HaveOccurred())

		engineCtx, engineCancel = context.WithCancel(ctx)
		Expect(engine.Start(engineCtx)).To(Succeed())
	})

	AfterEach(func() {
		if engineCancel != nil {
			engineCancel()
		}

		reg := newPostgresRecordingRegistry(&recordingBackend{})
		Expect(removeKollectClusterInventoryWithFinalizer(ctx, inventoryName, store, reg)).To(Succeed())
		Expect(removeKollectClusterTargetWithFinalizer(ctx, targetName, engine)).To(Succeed())
		_ = k8sClient.Delete(ctx, &kollectdevv1alpha1.KollectSink{
			ObjectMeta: metav1.ObjectMeta{Name: sinkName, Namespace: sink.DefaultSecretNamespace},
		})
		_ = k8sClient.Delete(ctx, &kollectdevv1alpha1.KollectProfile{
			ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: sink.DefaultSecretNamespace},
		})
		deleteNamespaceBestEffort(ctx, kubeClient, nsMatched)
	})

	setupCollectFixtures := func() {
		ensureNamespace(ctx, kubeClient, nsMatched, map[string]string{tenantLabel: tenantLabelVal})
		ensureNamespace(ctx, kubeClient, sink.DefaultSecretNamespace, nil)

		for _, name := range []string{"export-cm-1", "export-cm-2"} {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nsMatched},
				Data:       map[string]string{"key": name},
			}
			_, err := kubeClient.CoreV1().ConfigMaps(nsMatched).Create(ctx, cm, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		}

		profile := &kollectdevv1alpha1.KollectProfile{
			ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: sink.DefaultSecretNamespace},
			Spec: kollectdevv1alpha1.KollectProfileSpec{
				TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
				Attributes: []kollectdevv1alpha1.AttributeSpec{
					{Name: "name", Path: "{.metadata.name}"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, profile)).To(Succeed())

		target := &kollectdevv1alpha1.KollectClusterTarget{
			ObjectMeta: metav1.ObjectMeta{Name: targetName},
			Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
				ProfileRef: profileName,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantLabelVal},
				},
			},
		}
		Expect(k8sClient.Create(ctx, target)).To(Succeed())

		targetReconciler := &KollectClusterTargetReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Engine: engine,
		}
		_, err := targetReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: targetName},
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() int {
			return engine.ItemCount(nsMatched, targetName)
		}, 30*time.Second, 200*time.Millisecond).Should(Equal(2))

		_, err = targetReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: targetName},
		})
		Expect(err).NotTo(HaveOccurred())

		sinkObj, pgSecret := createPostgresSinkFixtures(sinkName, "pg-"+suffix, sink.DefaultSecretNamespace)
		Expect(k8sClient.Create(ctx, sinkObj)).To(Succeed())
		Expect(k8sClient.Create(ctx, pgSecret)).To(Succeed())
	}

	It("records sinkExports after cluster rollup export", func() {
		setupCollectFixtures()

		inv := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: inventoryName},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantLabelVal},
				},
				TargetRefs:    []string{targetName},
				SinkRefs:      kollectdevv1alpha1.NewSinkRefList(sinkName),
				SinkNamespace: sink.DefaultSecretNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())

		recorder := &recordingBackend{}
		invReconciler := &KollectClusterInventoryReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Engine:   engine,
			Store:    store,
			Registry: newPostgresRecordingRegistry(recorder),
		}

		_, err := invReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: inventoryName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(1))

		updated := &kollectdevv1alpha1.KollectClusterInventory{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: inventoryName}, updated)).To(Succeed())
		Expect(updated.Status.SinkExports).To(HaveLen(1))
		Expect(updated.Status.SinkExports[0].Name).To(Equal(sinkName))
		Expect(updated.Status.SinkExports[0].LastExportTime).NotTo(BeNil())
		Expect(updated.Status.ItemCount).To(Equal(2))

		synced := apimeta.FindStatusCondition(updated.Status.SinkExports[0].Conditions, conditionSinkSynced)
		Expect(synced).NotTo(BeNil())
		Expect(synced.Status).To(Equal(metav1.ConditionTrue))
	})

	It("exports to two sinks and debounces independently on second reconcile", func() {
		setupCollectFixtures()
		// Freeze the store so async informer updates cannot change the rollup checksum
		// between back-to-back reconciles (full-suite timing otherwise flakes debounce).
		if engineCancel != nil {
			engineCancel()
			engineCancel = nil
		}

		sinkB := "export-pg-b-" + suffix
		sinkObjB, pgSecretB := createPostgresSinkFixtures(sinkB, "pg-b-"+suffix, sink.DefaultSecretNamespace)
		Expect(k8sClient.Create(ctx, sinkObjB)).To(Succeed())
		Expect(k8sClient.Create(ctx, pgSecretB)).To(Succeed())

		longInterval := metav1.Duration{Duration: 15 * time.Minute}
		inv := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: inventoryName},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantLabelVal},
				},
				TargetRefs:        []string{targetName},
				SinkNamespace:     sink.DefaultSecretNamespace,
				ExportMinInterval: &longInterval,
				SinkRefs: kollectdevv1alpha1.InventorySinkRefList{
					{Name: sinkName},
					{Name: sinkB},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())

		recorder := &recordingBackend{}
		invReconciler := &KollectClusterInventoryReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Engine:   engine,
			Store:    store,
			Registry: newPostgresRecordingRegistry(recorder),
		}

		DeferCleanup(func() {
			sink.EvictBackendPool(sink.DefaultSecretNamespace, sinkName)
			sink.EvictBackendPool(sink.DefaultSecretNamespace, sinkB)
		})

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: inventoryName}}
		_, err := invReconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(2))

		_, err = invReconciler.Reconcile(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.exported).To(HaveLen(2), "second reconcile should debounce both sinks")

		updated := &kollectdevv1alpha1.KollectClusterInventory{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: inventoryName}, updated)).To(Succeed())
		for _, exportStatus := range updated.Status.SinkExports {
			debounced := apimeta.FindStatusCondition(exportStatus.Conditions, conditionSinkSynced)
			Expect(debounced).NotTo(BeNil())
			Expect(debounced.Reason).To(Equal(kollectdevv1alpha1.ReasonDebounced))
		}
	})

	It("requeues when no cluster targets match during bootstrap window (EC-P1-14)", func() {
		ensureNamespace(ctx, kubeClient, nsMatched, map[string]string{tenantLabel: tenantLabelVal})

		inv := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: inventoryName},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{tenantLabel: tenantLabelVal},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inv)).To(Succeed())

		invReconciler := &KollectClusterInventoryReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Engine: engine,
			Store:  store,
		}

		result, err := invReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: inventoryName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(30 * time.Second))
	})
})
