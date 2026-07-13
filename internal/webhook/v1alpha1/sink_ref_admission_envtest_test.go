// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package webhookv1alpha1

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// These L1 admission specs drive KollectClusterInventory create requests through the
// real API server + validating webhook (envtest), covering the cluster-scope sink-ref
// allowlist admission path (allow vs deny, error names the offending ref) and the
// CRD-level defaulting applied on admission (COV-90-06). The existing scope-ceiling
// envtest specs cover GVK admission only, not sink-ref admission or defaulting.
var _ = Describe("Webhook cluster-scope sink-ref admission (envtest)", func() {
	It("denies KollectClusterInventory whose database sink ref is outside the cluster scope allowlist", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())

		clusterScope := &kollectdevv1alpha1.KollectClusterScope{
			ObjectMeta: metav1.ObjectMeta{Name: "platform-db-" + suffix},
			Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
				DatabaseSinkRefs:           []string{"warehouse"},
				AllowedStaticRefNamespaces: []string{"kollect-system"},
			},
		}
		Expect(webhookClient.Create(webhookCtx, clusterScope)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, clusterScope) }()

		inv := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: "rollup-denied-" + suffix},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"team": "platform"},
				},
				DatabaseSinkRefs: kollectdevv1alpha1.InventorySinkRefList{
					{Name: "shadow-db", Namespace: "kollect-system"},
				},
			},
		}
		err := webhookClient.Create(webhookCtx, inv)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsForbidden(err)).To(BeTrue())
		// The rejection must name the offending ref, not merely fail.
		Expect(err.Error()).To(ContainSubstring("shadow-db"))
		Expect(err.Error()).To(ContainSubstring(clusterScope.Name))
	})

	It("admits KollectClusterInventory whose database sink ref is within the cluster scope allowlist and applies CRD defaults", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())

		clusterScope := &kollectdevv1alpha1.KollectClusterScope{
			ObjectMeta: metav1.ObjectMeta{Name: "platform-ok-" + suffix},
			Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
				DatabaseSinkRefs:           []string{"warehouse"},
				AllowedStaticRefNamespaces: []string{"kollect-system"},
			},
		}
		Expect(webhookClient.Create(webhookCtx, clusterScope)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, clusterScope) }()

		invName := "rollup-ok-" + suffix
		inv := &kollectdevv1alpha1.KollectClusterInventory{
			ObjectMeta: metav1.ObjectMeta{Name: invName},
			Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"team": "platform"},
				},
				DatabaseSinkRefs: kollectdevv1alpha1.InventorySinkRefList{
					{Name: "warehouse", Namespace: "kollect-system"},
				},
				// sinkNamespace, exportMinInterval and dedupe left unset to exercise
				// CRD-level defaulting on admission.
			},
		}
		Expect(webhookClient.Create(webhookCtx, inv)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, inv) }()

		// Read back to observe the values the API server defaulted during admission.
		got := &kollectdevv1alpha1.KollectClusterInventory{}
		Expect(webhookClient.Get(webhookCtx, client.ObjectKey{Name: invName}, got)).To(Succeed())
		Expect(got.Spec.SinkNamespace).To(Equal("kollect-system"))
		Expect(got.Spec.Dedupe).To(Equal(kollectdevv1alpha1.ClusterInventoryDedupeKeepAll))
		Expect(got.Spec.ExportMinInterval).NotTo(BeNil())
		Expect(got.Spec.ExportMinInterval.Duration).To(Equal(30 * time.Second))
	})
})
