// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

var _ = Describe("KollectTarget Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-resource"
			profileName  = "test-profile"
		)

		var (
			reconcileCtx       context.Context
			typeNamespacedName types.NamespacedName
			kollecttarget      *kollectdevv1alpha1.KollectTarget
		)

		BeforeEach(func() {
			reconcileCtx = context.Background()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			profile := &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: "default"},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: kollectdevv1alpha1.GroupVersionKind{
						Version: "v1",
						Kind:    "ConfigMap",
					},
				},
			}
			Expect(k8sClient.Create(reconcileCtx, profile)).To(Succeed())

			kollecttarget = &kollectdevv1alpha1.KollectTarget{}
			err := k8sClient.Get(reconcileCtx, typeNamespacedName, kollecttarget)
			if err != nil && errors.IsNotFound(err) {
				resource := &kollectdevv1alpha1.KollectTarget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: kollectdevv1alpha1.KollectTargetSpec{
						ProfileRef: profileName,
					},
				}
				Expect(k8sClient.Create(reconcileCtx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			target := &kollectdevv1alpha1.KollectTarget{}
			err := k8sClient.Get(reconcileCtx, typeNamespacedName, target)
			if err == nil {
				Expect(k8sClient.Delete(reconcileCtx, target)).To(Succeed())
			}

			profile := &kollectdevv1alpha1.KollectProfile{}
			err = k8sClient.Get(reconcileCtx, types.NamespacedName{Name: profileName, Namespace: "default"}, profile)
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
				ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: "default"},
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
})
