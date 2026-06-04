// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
			ctx                context.Context
			typeNamespacedName types.NamespacedName
			kollecttarget      *kollectdevv1alpha1.KollectTarget
		)

		BeforeEach(func() {
			ctx = context.Background()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			profile := &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{Name: profileName},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: kollectdevv1alpha1.GroupVersionKind{
						Version: "v1",
						Kind:    "ConfigMap",
					},
				},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())

			kollecttarget = &kollectdevv1alpha1.KollectTarget{}
			err := k8sClient.Get(ctx, typeNamespacedName, kollecttarget)
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
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			target := &kollectdevv1alpha1.KollectTarget{}
			err := k8sClient.Get(ctx, typeNamespacedName, target)
			if err == nil {
				Expect(k8sClient.Delete(ctx, target)).To(Succeed())
			}

			profile := &kollectdevv1alpha1.KollectProfile{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: profileName}, profile)
			if err == nil {
				Expect(k8sClient.Delete(ctx, profile)).To(Succeed())
			}
		})

		It("should mark the target Ready when profileRef resolves", func() {
			controllerReconciler := &KollectTargetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectTarget{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())

			ready := apimeta.FindStatusCondition(updated.Status.Conditions, conditionReady)
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
		})

		It("should mark the target Degraded when profileRef is missing", func() {
			target := &kollectdevv1alpha1.KollectTarget{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, target)).To(Succeed())
			target.Spec.ProfileRef = "missing-profile"
			Expect(k8sClient.Update(ctx, target)).To(Succeed())

			controllerReconciler := &KollectTargetReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectTarget{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())

			degraded := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(degraded).NotTo(BeNil())
			Expect(degraded.Status).To(Equal(metav1.ConditionTrue))
			Expect(degraded.Reason).To(Equal("ProfileNotFound"))
		})
	})
})
