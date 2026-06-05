// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package spoke_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/spoke"
)

func TestTryPublishReportNoOpWithoutEnv(t *testing.T) {
	t.Setenv("KOLLECT_SPOKE_CLUSTER", "")

	store := collect.NewStore()
	inv := &kollectdevv1alpha1.KollectInventory{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "inv"},
	}

	if err := spoke.TryPublishReport(context.Background(), store, inv); err != nil {
		t.Fatalf("expected no-op, got %v", err)
	}
}
