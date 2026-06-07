// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
)

var _ = Describe("KollectTarget Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName  = "target-test-resource"
			profileName   = "target-test-profile"
			testNamespace = "target-envtest"
		)

		var (
			reconcileCtx       context.Context
			typeNamespacedName types.NamespacedName
		)

		BeforeEach(func() {
			reconcileCtx = context.Background()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespace,
			}

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
			err := k8sClient.Get(reconcileCtx, types.NamespacedName{Name: testNamespace}, ns)
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(reconcileCtx, ns)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			profile := &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: testNamespace},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: kollectdevv1alpha1.GroupVersionKind{
						Version: "v1",
						Kind:    "ConfigMap",
					},
				},
			}
			Expect(removeKollectTargetWithFinalizer(reconcileCtx, typeNamespacedName)).To(Succeed())
			_ = k8sClient.Delete(reconcileCtx, profile)

			Expect(k8sClient.Create(reconcileCtx, profile)).To(Succeed())

			resource := &kollectdevv1alpha1.KollectTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: testNamespace,
				},
				Spec: kollectdevv1alpha1.KollectTargetSpec{
					ProfileRef: profileName,
				},
			}
			Expect(k8sClient.Create(reconcileCtx, resource)).To(Succeed())
		})

		AfterEach(func() {
			Expect(removeKollectTargetWithFinalizer(reconcileCtx, typeNamespacedName)).To(Succeed())

			profile := &kollectdevv1alpha1.KollectProfile{}
			err := k8sClient.Get(reconcileCtx, types.NamespacedName{Name: profileName, Namespace: testNamespace}, profile)
			if err == nil {
				Expect(k8sClient.Delete(reconcileCtx, profile)).To(Succeed())
			}
		})

		It("should mark the target Ready when profileRef resolves", func() {
			controllerReconciler := &KollectTargetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(reconcileCtx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			updated := &kollectdevv1alpha1.KollectTarget{}
			Expect(k8sClient.Get(reconcileCtx, typeNamespacedName, updated)).To(Succeed())

			ready := apimeta.FindStatusCondition(updated.Status.Conditions, conditionReady)
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
		})

		It("should not resolve a profile from a different namespace", func() {
			Expect(k8sClient.Delete(reconcileCtx, &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: testNamespace},
			})).To(Succeed())

			otherNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}}
			Expect(k8sClient.Create(reconcileCtx, otherNS)).To(Succeed())
			defer func() { _ = k8sClient.Delete(reconcileCtx, otherNS) }()

			otherProfile := &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: "other"},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: kollectdevv1alpha1.GroupVersionKind{
						Version: "v1",
						Kind:    "ConfigMap",
					},
				},
			}
			Expect(k8sClient.Create(reconcileCtx, otherProfile)).To(Succeed())
			defer func() {
				Expect(k8sClient.Delete(reconcileCtx, otherProfile)).To(Succeed())
			}()

			controllerReconciler := &KollectTargetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(reconcileCtx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectTarget{}
			Expect(k8sClient.Get(reconcileCtx, typeNamespacedName, updated)).To(Succeed())

			degraded := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(degraded).NotTo(BeNil())
			Expect(degraded.Reason).To(Equal("ProfileNotFound"))
		})

		It("should mark the target Degraded when profileRef is missing", func() {
			target := &kollectdevv1alpha1.KollectTarget{}
			Expect(k8sClient.Get(reconcileCtx, typeNamespacedName, target)).To(Succeed())
			target.Spec.ProfileRef = "missing-profile"
			Expect(k8sClient.Update(reconcileCtx, target)).To(Succeed())

			controllerReconciler := &KollectTargetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(reconcileCtx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectTarget{}
			Expect(k8sClient.Get(reconcileCtx, typeNamespacedName, updated)).To(Succeed())

			degraded := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(degraded).NotTo(BeNil())
			Expect(degraded.Status).To(Equal(metav1.ConditionTrue))
			Expect(degraded.Reason).To(Equal("ProfileNotFound"))
		})
	})

	Context("with collection engine", func() {
		var (
			engineCtx    context.Context
			engineCancel context.CancelFunc
			engine       *collect.Engine
			testNS       string
		)

		BeforeEach(func() {
			testNS = "target-engine-" + testNameSuffix()

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}}
			Expect(k8sClient.Create(context.Background(), ns)).To(Succeed())

			kubeClient, err := kubernetes.NewForConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
			dyn, err := dynamic.NewForConfig(cfg)
			Expect(err).NotTo(HaveOccurred())

			store := collect.NewStore()
			engine, err = collect.NewEngine(dyn, kubeClient, store, collect.EngineConfig{})
			Expect(err).NotTo(HaveOccurred())

			engineCtx, engineCancel = context.WithCancel(context.Background())
			Expect(engine.Start(engineCtx)).To(Succeed())
		})

		AfterEach(func() {
			if engineCancel != nil {
				engineCancel()
			}
			_ = k8sClient.Delete(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}})
		})

		It("registers target with engine and collects ConfigMap", func() {
			reconcileCtx := context.Background()
			profileName := "engine-profile-" + testNameSuffix()
			targetName := "engine-target-" + testNameSuffix()

			profile := &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: testNS},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
					Attributes: []kollectdevv1alpha1.AttributeSpec{
						{Name: "name", Path: "{.metadata.name}"},
					},
				},
			}
			Expect(k8sClient.Create(reconcileCtx, profile)).To(Succeed())
			defer func() { _ = k8sClient.Delete(reconcileCtx, profile) }()

			kubeClient, err := kubernetes.NewForConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
			for _, name := range []string{"cm-a", "cm-b"} {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
					Data:       map[string]string{"k": name},
				}
				_, err = kubeClient.CoreV1().ConfigMaps(testNS).Create(reconcileCtx, cm, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
			}

			target := &kollectdevv1alpha1.KollectTarget{
				ObjectMeta: metav1.ObjectMeta{Name: targetName, Namespace: testNS},
				Spec: kollectdevv1alpha1.KollectTargetSpec{
					ProfileRef: profileName,
					CollectionFilterSpec: kollectdevv1alpha1.CollectionFilterSpec{
						IncludedNamespaces: []string{testNS},
					},
				},
			}
			Expect(k8sClient.Create(reconcileCtx, target)).To(Succeed())
			defer func() { _ = k8sClient.Delete(reconcileCtx, target) }()

			reconciler := &KollectTargetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Engine: engine,
			}

			_, err = reconciler.Reconcile(reconcileCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: targetName, Namespace: testNS},
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int {
				return engine.ItemCount(testNS, targetName)
			}, 30*time.Second, 200*time.Millisecond).Should(Equal(2))

			_, err = reconciler.Reconcile(reconcileCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: targetName, Namespace: testNS},
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectTarget{}
			Expect(k8sClient.Get(reconcileCtx, types.NamespacedName{Name: targetName, Namespace: testNS}, updated)).To(Succeed())

			ready := apimeta.FindStatusCondition(updated.Status.Conditions, conditionReady)
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		})
	})
})
