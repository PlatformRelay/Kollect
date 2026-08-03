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

// This L1 admission spec drives a KollectInventory UPDATE through the real API server +
// validating webhook (envtest), covering the ValidateUpdate non-deletion re-validation
// path (COV-90-S05). The EDGE intent — "an UPDATE that violates an invariant is DENIED
// with a clear message" — is exercised here by mutating an admitted inventory into an
// invalid state (a cross-namespace family sink ref) on update.
//
// NOTE: the task framed this EDGE as "changing an immutable field". KollectInventory has
// no immutable fields — there is no CEL/x-kubernetes-validations marker on any CRD and
// ValidateUpdate takes the old object as `_`, so it structurally cannot compare old vs
// new. The equivalent update-denial invariant (cross-namespace sink ref) is covered
// instead; see the lane return for the spec/code-mismatch finding.
var _ = Describe("Webhook KollectInventory update admission (envtest)", func() {
	It("admits a valid inventory then DENIES an update that introduces a cross-namespace sink ref", func() {
		suffix := fmt.Sprintf("%x", time.Now().UnixNano())
		name := "inv-update-" + suffix

		inv := &kollectdevv1alpha1.KollectInventory{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: kollectdevv1alpha1.KollectInventorySpec{
				DatabaseSinkRefs: kollectdevv1alpha1.NewSinkRefList("warehouse"),
			},
		}
		Expect(webhookClient.Create(webhookCtx, inv)).To(Succeed())
		defer func() { _ = webhookClient.Delete(webhookCtx, inv) }()

		// Read back the admitted object, then mutate it into an invalid state.
		got := &kollectdevv1alpha1.KollectInventory{}
		Expect(webhookClient.Get(webhookCtx, client.ObjectKey{Name: name, Namespace: "default"}, got)).To(Succeed())
		got.Spec.DatabaseSinkRefs = kollectdevv1alpha1.NewSinkRefList("other-ns/shadow-sink")

		err := webhookClient.Update(webhookCtx, got)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsForbidden(err)).To(BeTrue())
		// The rejection must be actionable: it names the inventory and the offending field.
		Expect(err.Error()).To(ContainSubstring(name))
		Expect(err.Error()).To(ContainSubstring("databaseSinkRefs"))
	})
})
