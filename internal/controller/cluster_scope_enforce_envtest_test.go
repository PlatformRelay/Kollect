// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/sink"
)

// COV-90-01: envtest behavior tests for the scope-enforcement error/degrade paths on
// the cluster-scoped controllers. These exercise real reconcile outcomes (persisted
// status conditions + emitted Warning events), not just "no error".
var _ = Describe("Scope enforcement degrade paths (COV-90-01)", func() {
	var (
		suffix     string
		kubeClient kubernetes.Interface
	)

	BeforeEach(func() {
		suffix = fmt.Sprintf("%x", time.Now().UnixNano())
		var err error
		kubeClient, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())
	})

	// resolveProfileOrDegrade: profile missing -> ProfileNotFound Degraded, no event.
	// Drives the full KollectClusterTarget Reconcile with a nil Engine; the resolve
	// helper runs before any engine work and returns early on degrade.
	Describe("resolveProfileOrDegrade", func() {
		It("degrades KollectClusterTarget with ProfileNotFound when profileRef is missing", func() {
			targetName := "missing-profile-ct-" + suffix
			ct := &kollectdevv1alpha1.KollectClusterTarget{
				ObjectMeta: metav1.ObjectMeta{Name: targetName},
				Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
					ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{
						Name:      "does-not-exist-" + suffix,
						Namespace: sink.DefaultSecretNamespace,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ct)).To(Succeed())
			defer func() {
				Expect(removeKollectClusterTargetWithFinalizer(ctx, targetName, nil)).To(Succeed())
			}()

			recorder := record.NewFakeRecorder(5)
			r := &KollectClusterTargetReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: targetName},
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectClusterTarget{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: targetName}, updated)).To(Succeed())

			deg := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(deg).NotTo(BeNil())
			Expect(deg.Status).To(Equal(metav1.ConditionTrue))
			Expect(deg.Reason).To(Equal(reasonProfileNotFound))
			Expect(deg.Message).To(ContainSubstring("not found"))

			// A plain NotFound is not a forbidden RBAC denial, so no Warning event.
			Consistently(recorder.Events).ShouldNot(Receive())
		})
	})

	// loadClusterScopeBinding: enforced KollectClusterScope whose allowedStaticRefNamespaces
	// excludes the profileRef namespace -> ScopeNamespaceDenied Degraded + Warning event.
	Describe("loadClusterScopeBinding", func() {
		It("degrades KollectClusterTarget when profileRef namespace is outside cluster scope", func() {
			targetName := "ns-denied-ct-" + suffix
			profileName := "scoped-profile-" + suffix
			scopeName := "ct-cluster-scope-" + suffix
			profileNS := sink.DefaultSecretNamespace
			ensureNamespace(ctx, kubeClient, profileNS, nil)

			profile := &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: profileNS},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
				},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, profile) }()

			// allowedStaticRefNamespaces lists only "some-other-ns", so the profile's
			// namespace (kollect-system) is denied.
			clusterScope := &kollectdevv1alpha1.KollectClusterScope{
				ObjectMeta: metav1.ObjectMeta{Name: scopeName},
				Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
					AllowedStaticRefNamespaces: []string{"some-other-ns-" + suffix},
				},
			}
			Expect(k8sClient.Create(ctx, clusterScope)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, clusterScope) }()

			ct := &kollectdevv1alpha1.KollectClusterTarget{
				ObjectMeta: metav1.ObjectMeta{Name: targetName},
				Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
					ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{
						Name:      profileName,
						Namespace: profileNS,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ct)).To(Succeed())
			defer func() {
				Expect(removeKollectClusterTargetWithFinalizer(ctx, targetName, nil)).To(Succeed())
			}()

			recorder := record.NewFakeRecorder(5)
			r := &KollectClusterTargetReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: targetName},
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectClusterTarget{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: targetName}, updated)).To(Succeed())

			deg := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(deg).NotTo(BeNil())
			Expect(deg.Status).To(Equal(metav1.ConditionTrue))
			Expect(deg.Reason).To(Equal(scopeReasonNSDenied))
			Expect(deg.Message).To(ContainSubstring("allowedStaticRefNamespaces"))

			var ev string
			Eventually(recorder.Events).Should(Receive(&ev))
			Expect(ev).To(ContainSubstring(scopeReasonNSDenied))
		})

		It("does not degrade when profileRef namespace is allowed by cluster scope", func() {
			targetName := "ns-allowed-ct-" + suffix
			profileName := "allowed-profile-" + suffix
			scopeName := "ct-allow-scope-" + suffix
			profileNS := sink.DefaultSecretNamespace
			ensureNamespace(ctx, kubeClient, profileNS, nil)

			profile := &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{Name: profileName, Namespace: profileNS},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
				},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, profile) }()

			// The profile namespace IS listed, so the static-ref check passes and the
			// binding is returned without degrade (scopeDegraded == false branch).
			clusterScope := &kollectdevv1alpha1.KollectClusterScope{
				ObjectMeta: metav1.ObjectMeta{Name: scopeName},
				Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
					AllowedStaticRefNamespaces: []string{profileNS},
				},
			}
			Expect(k8sClient.Create(ctx, clusterScope)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, clusterScope) }()

			ct := &kollectdevv1alpha1.KollectClusterTarget{
				ObjectMeta: metav1.ObjectMeta{Name: targetName},
				Spec: kollectdevv1alpha1.KollectClusterTargetSpec{
					ProfileRef: kollectdevv1alpha1.NamespacedObjectReference{
						Name:      profileName,
						Namespace: profileNS,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ct)).To(Succeed())
			defer func() {
				Expect(removeKollectClusterTargetWithFinalizer(ctx, targetName, nil)).To(Succeed())
			}()

			recorder := record.NewFakeRecorder(5)
			r := &KollectClusterTargetReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: targetName},
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectClusterTarget{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: targetName}, updated)).To(Succeed())

			// A ClusterTarget with no engine and an allowed scope reaches setReady.
			deg := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(deg).To(BeNil(), "allowed cluster scope must not degrade the target")
			ready := apimeta.FindStatusCondition(updated.Status.Conditions, conditionReady)
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))

			Consistently(recorder.Events).ShouldNot(Receive())
		})
	})

	// enforceClusterScopePolicy: direct-call is the pragmatic reach here because the
	// full KollectClusterInventory reconcile requires Ready targets + engine rollup
	// before it reaches scope enforcement. The direct call still runs real
	// scope.LoadCluster, real validation, and setDegraded which persists to the API.
	Describe("enforceClusterScopePolicy", func() {
		newInv := func(name, sinkNS string, snapshotRefs kollectdevv1alpha1.InventorySinkRefList) *kollectdevv1alpha1.KollectClusterInventory {
			return &kollectdevv1alpha1.KollectClusterInventory{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Spec: kollectdevv1alpha1.KollectClusterInventorySpec{
					SinkNamespace:    sinkNS,
					SnapshotSinkRefs: snapshotRefs,
				},
			}
		}

		It("allows when no cluster scope is enforced (zero result, no degrade)", func() {
			invName := "ecsp-allow-noscope-" + suffix
			inv := newInv(invName, sink.DefaultSecretNamespace, nil)
			Expect(k8sClient.Create(ctx, inv)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, inv) }()

			recorder := record.NewFakeRecorder(5)
			r := &KollectClusterInventoryReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			result, err := r.enforceClusterScopePolicy(
				ctx, inv, inv.Spec.SinkNamespace, clusterInventorySinkBindings(inv),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.IsZero()).To(BeTrue())
			Consistently(recorder.Events).ShouldNot(Receive())
		})

		It("degrades with SinkNamespaceDenied when a sink namespace is outside cluster scope", func() {
			invName := "ecsp-nsdeny-" + suffix
			sinkNS := "denied-sink-ns-" + suffix
			inv := newInv(invName, sinkNS, kollectdevv1alpha1.InventorySinkRefList{
				{Name: "some-snapshot-sink"},
			})
			Expect(k8sClient.Create(ctx, inv)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, inv) }()

			// Cluster scope allows only an unrelated namespace, so sinkNS is denied by the
			// static-ref namespace check (the first deny branch).
			scopeName := "ecsp-ns-scope-" + suffix
			clusterScope := &kollectdevv1alpha1.KollectClusterScope{
				ObjectMeta: metav1.ObjectMeta{Name: scopeName},
				Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
					AllowedStaticRefNamespaces: []string{"allowed-ns-" + suffix},
				},
			}
			Expect(k8sClient.Create(ctx, clusterScope)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, clusterScope) }()

			recorder := record.NewFakeRecorder(5)
			r := &KollectClusterInventoryReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			result, err := r.enforceClusterScopePolicy(
				ctx, inv, inv.Spec.SinkNamespace, clusterInventorySinkBindings(inv),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.IsZero()).To(BeFalse(), "a degrade returns a non-zero requeue result")

			updated := &kollectdevv1alpha1.KollectClusterInventory{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: invName}, updated)).To(Succeed())
			deg := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(deg).NotTo(BeNil())
			Expect(deg.Status).To(Equal(metav1.ConditionTrue))
			Expect(deg.Reason).To(Equal(reasonSinkNamespaceDenied))
			Expect(deg.Message).To(ContainSubstring("allowedStaticRefNamespaces"))

			var ev string
			Eventually(recorder.Events).Should(Receive(&ev))
			Expect(ev).To(ContainSubstring(reasonSinkNamespaceDenied))
		})

		It("degrades with ScopeSinkDenied when a sink ref is outside the family allowlist", func() {
			invName := "ecsp-sinkdeny-" + suffix
			sinkNS := sink.DefaultSecretNamespace
			inv := newInv(invName, sinkNS, kollectdevv1alpha1.InventorySinkRefList{
				{Name: "unlisted-snapshot-sink-" + suffix, Namespace: sinkNS},
			})
			Expect(k8sClient.Create(ctx, inv)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, inv) }()

			// The sink namespace is allowed (so the first check passes), but the
			// snapshotSinkRefs allowlist does not contain the referenced sink name,
			// hitting the second deny branch (ScopeSinkDenied).
			scopeName := "ecsp-sink-scope-" + suffix
			clusterScope := &kollectdevv1alpha1.KollectClusterScope{
				ObjectMeta: metav1.ObjectMeta{Name: scopeName},
				Spec: kollectdevv1alpha1.KollectClusterScopeSpec{
					AllowedStaticRefNamespaces: []string{sinkNS},
					SnapshotSinkRefs:           []string{"only-this-sink-is-allowed"},
				},
			}
			Expect(k8sClient.Create(ctx, clusterScope)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, clusterScope) }()

			recorder := record.NewFakeRecorder(5)
			r := &KollectClusterInventoryReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			result, err := r.enforceClusterScopePolicy(
				ctx, inv, inv.Spec.SinkNamespace, clusterInventorySinkBindings(inv),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.IsZero()).To(BeFalse())

			updated := &kollectdevv1alpha1.KollectClusterInventory{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: invName}, updated)).To(Succeed())
			deg := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(deg).NotTo(BeNil())
			Expect(deg.Status).To(Equal(metav1.ConditionTrue))
			Expect(deg.Reason).To(Equal(scopeReasonSinkDenied))
			Expect(deg.Message).To(ContainSubstring("snapshotSinkRefs"))

			var ev string
			Eventually(recorder.Events).Should(Receive(&ev))
			Expect(ev).To(ContainSubstring(scopeReasonSinkDenied))
		})
	})

	// enforceTarget: in-scope (allow) and out-of-scope (namespace denied) via the full
	// KollectTarget reconcile with a nil engine (resolveTargetFilterStatus is nil-engine
	// safe). The existing kollecttarget_scope_test.go covers only the GVK-denied branch.
	Describe("enforceTarget", func() {
		It("allows a target whose namespace is within the KollectScope allowlist", func() {
			testNS := "et-allow-ns-" + suffix
			ensureNamespace(ctx, kubeClient, testNS, nil)

			teamScope := &kollectdevv1alpha1.KollectScope{
				ObjectMeta: metav1.ObjectMeta{Name: "et-allow-scope-" + suffix, Namespace: testNS},
				Spec: kollectdevv1alpha1.KollectScopeSpec{
					ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
						AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
							{Version: "v1", Kind: "ConfigMap"},
						},
						AllowedNamespaces: []string{testNS},
					},
				},
			}
			Expect(k8sClient.Create(ctx, teamScope)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, teamScope) }()

			profile := &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{Name: "et-allow-profile-" + suffix, Namespace: testNS},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
				},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, profile) }()

			// Select the target's own namespace so intent + effective namespaces resolve
			// to testNS (which the scope allowlists), keeping this an in-scope target.
			target := &kollectdevv1alpha1.KollectTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "et-allow-target-" + suffix, Namespace: testNS},
				Spec: kollectdevv1alpha1.KollectTargetSpec{
					ProfileRef: profile.Name,
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{corev1.LabelMetadataName: testNS},
					},
				},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, target) }()

			recorder := record.NewFakeRecorder(5)
			r := &KollectTargetReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: target.Name, Namespace: testNS},
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectTarget{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: target.Name, Namespace: testNS}, updated)).To(Succeed())

			// In-scope: enforceTarget returns ok, so the target proceeds to Ready with no
			// scope Degraded condition.
			deg := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(deg).To(BeNil(), "an in-scope target must not carry a scope Degraded condition")
			ready := apimeta.FindStatusCondition(updated.Status.Conditions, conditionReady)
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		})

		It("degrades a target whose intent namespace is outside the KollectScope allowlist", func() {
			testNS := "et-deny-ns-" + suffix
			ensureNamespace(ctx, kubeClient, testNS, nil)

			// GVK is allowed, but allowedNamespaces excludes the target's own namespace,
			// so the intent-namespace check (ValidateTargetIncludedNamespaces) denies it.
			teamScope := &kollectdevv1alpha1.KollectScope{
				ObjectMeta: metav1.ObjectMeta{Name: "et-deny-scope-" + suffix, Namespace: testNS},
				Spec: kollectdevv1alpha1.KollectScopeSpec{
					ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
						AllowedGVKs: []kollectdevv1alpha1.GroupVersionKind{
							{Version: "v1", Kind: "ConfigMap"},
						},
						AllowedNamespaces: []string{"only-other-ns-" + suffix},
					},
				},
			}
			Expect(k8sClient.Create(ctx, teamScope)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, teamScope) }()

			profile := &kollectdevv1alpha1.KollectProfile{
				ObjectMeta: metav1.ObjectMeta{Name: "et-deny-profile-" + suffix, Namespace: testNS},
				Spec: kollectdevv1alpha1.KollectProfileSpec{
					TargetGVK: kollectdevv1alpha1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
				},
			}
			Expect(k8sClient.Create(ctx, profile)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, profile) }()

			target := &kollectdevv1alpha1.KollectTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "et-deny-target-" + suffix, Namespace: testNS},
				Spec: kollectdevv1alpha1.KollectTargetSpec{
					ProfileRef: profile.Name,
				},
			}
			Expect(k8sClient.Create(ctx, target)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, target) }()

			recorder := record.NewFakeRecorder(5)
			r := &KollectTargetReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: target.Name, Namespace: testNS},
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &kollectdevv1alpha1.KollectTarget{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: target.Name, Namespace: testNS}, updated)).To(Succeed())

			deg := apimeta.FindStatusCondition(updated.Status.Conditions, conditionDegraded)
			Expect(deg).NotTo(BeNil())
			Expect(deg.Status).To(Equal(metav1.ConditionTrue))
			Expect(deg.Reason).To(Equal(scopeReasonNSDenied))
			Expect(deg.Message).To(ContainSubstring("allowedNamespaces"))

			var ev string
			Eventually(recorder.Events).Should(Receive(&ev))
			Expect(ev).To(ContainSubstring(scopeReasonNSDenied))
		})
	})

	// scopeFloor / clusterScopeFloor: the enforced branch returns ScopeMinExportInterval;
	// the not-enforced branch returns 0. The returned duration IS the outcome (no status
	// or event side effects), so the return value is the assertion.
	Describe("scopeFloor", func() {
		It("returns the scope minExportInterval when a namespaced KollectScope enforces one", func() {
			floorNS := "floor-ns-" + suffix
			ensureNamespace(ctx, kubeClient, floorNS, nil)

			floor := 42 * time.Second
			kScope := &kollectdevv1alpha1.KollectScope{
				ObjectMeta: metav1.ObjectMeta{Name: "floor-scope-" + suffix, Namespace: floorNS},
				Spec: kollectdevv1alpha1.KollectScopeSpec{
					ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
						MinExportInterval: &metav1.Duration{Duration: floor},
					},
				},
			}
			Expect(k8sClient.Create(ctx, kScope)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, kScope) }()

			r := &KollectInventoryReconciler{Client: k8sClient}
			Expect(r.scopeFloor(ctx, floorNS)).To(Equal(floor))
		})

		It("returns zero when no KollectScope enforces the namespace", func() {
			r := &KollectInventoryReconciler{Client: k8sClient}
			Expect(r.scopeFloor(ctx, "no-scope-ns-"+suffix)).To(Equal(time.Duration(0)))
		})
	})

	Describe("clusterScopeFloor", func() {
		It("returns the scope minExportInterval when the sink namespace is scope-enforced", func() {
			floorNS := "cluster-floor-ns-" + suffix
			ensureNamespace(ctx, kubeClient, floorNS, nil)

			floor := 17 * time.Second
			kScope := &kollectdevv1alpha1.KollectScope{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster-floor-scope-" + suffix, Namespace: floorNS},
				Spec: kollectdevv1alpha1.KollectScopeSpec{
					ScopeCeilingSpec: kollectdevv1alpha1.ScopeCeilingSpec{
						MinExportInterval: &metav1.Duration{Duration: floor},
					},
				},
			}
			Expect(k8sClient.Create(ctx, kScope)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, kScope) }()

			r := &KollectClusterInventoryReconciler{Client: k8sClient}
			Expect(r.clusterScopeFloor(ctx, floorNS)).To(Equal(floor))
		})

		It("returns zero when the sink namespace is not scope-enforced", func() {
			r := &KollectClusterInventoryReconciler{Client: k8sClient}
			Expect(r.clusterScopeFloor(ctx, "no-cluster-scope-ns-"+suffix)).To(Equal(time.Duration(0)))
		})
	})
})
