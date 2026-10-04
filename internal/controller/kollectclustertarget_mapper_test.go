// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/metrics"
)

// TestClusterTargetMappers_listErrorIsSignalled is the D3 lock: all three
// KollectClusterTargetReconciler watch mappers must log and count a failed List instead of
// silently returning nil (which drops namespace/scope/profile events for every cluster target
// with no signal). The namespaced twin increments WatchMapListErrorsTotal at
// kollectinventory_controller.go:728.
func TestClusterTargetMappers_listErrorIsSignalled(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kollectdevv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	cases := []struct {
		name  string
		watch string
		call  func(r *KollectClusterTargetReconciler) []reconcile.Request
	}{
		{
			name:  "namespace",
			watch: "namespace",
			call: func(r *KollectClusterTargetReconciler) []reconcile.Request {
				return r.mapNamespaceToClusterTargets(context.Background(), nil)
			},
		},
		{
			name:  "clusterScope",
			watch: "clusterScope",
			call: func(r *KollectClusterTargetReconciler) []reconcile.Request {
				return r.mapClusterScopeToClusterTargets(context.Background(), &kollectdevv1alpha1.KollectClusterScope{})
			},
		},
		{
			name:  "profile",
			watch: "profile",
			call: func(r *KollectClusterTargetReconciler) []reconcile.Request {
				return r.mapProfileToClusterTargets(context.Background(), &kollectdevv1alpha1.KollectProfile{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			base := fake.NewClientBuilder().WithScheme(scheme).Build()
			failClient := newListFailClusterTargetClient(base, errors.New("simulated list failure"))
			rec := &KollectClusterTargetReconciler{Client: failClient, Scheme: scheme}

			before := counterValue(metrics.WatchMapListErrorsTotal, "KollectClusterTarget", tc.watch)
			reqs := tc.call(rec)
			after := counterValue(metrics.WatchMapListErrorsTotal, "KollectClusterTarget", tc.watch)

			if len(reqs) != 0 {
				t.Fatalf("List failure must not enqueue; got %v", reqs)
			}
			if after-before != 1 {
				t.Fatalf("WatchMapListErrorsTotal{watch=%q} delta = %v, want 1", tc.watch, after-before)
			}
		})
	}
}
