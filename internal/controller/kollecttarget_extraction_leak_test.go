// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"strings"
	"testing"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
)

// Anyone with get on a KollectTarget reads status.lastExtractionError, the Degraded condition
// and its Warning events. Feed the real extractor's error for each reported probe through the
// status path and assert the selected Secret value reaches none of them.
func TestApplyTargetReadyState_ExtractionErrorDoesNotLeakFieldValues(t *testing.T) {
	t.Parallel()

	const secretValue = "SUPERSECRETVALUE"

	ext, err := collect.NewExtractor()
	if err != nil {
		t.Fatal(err)
	}

	secret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"name": "db-creds", "namespace": "scope-test-ns"},
		"data":       map[string]any{"pw": secretValue},
	}}

	for _, path := range []string{
		"cel:timestamp(object.data.pw)",
		"cel:object.data[object.data.pw]",
		"{.data[?(@.x>1)]}",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			_, extractErr := ext.Extract(secret, []kollectdevv1alpha1.AttributeSpec{{Name: "pw", Path: path}})
			if extractErr == nil {
				t.Fatalf("Extract(%q) error = nil, want an evaluation error", path)
			}

			target := newScopeTestTarget()
			r, recorder := newScopeTestReconciler(t, target)

			if _, err := r.applyTargetReadyState(
				context.Background(), target, 0, false, false, 1, extractErr.Error(),
			); err != nil {
				t.Fatalf("applyTargetReadyState: %v", err)
			}

			if target.Status.LastExtractionError == "" {
				t.Fatal("status.lastExtractionError is empty, want the attribute-scoped error class")
			}
			if strings.Contains(target.Status.LastExtractionError, secretValue) {
				t.Fatalf("status.lastExtractionError %q echoes the Secret value", target.Status.LastExtractionError)
			}

			degraded := apimeta.FindStatusCondition(target.Status.Conditions, conditionDegraded)
			if degraded == nil {
				t.Fatal("Degraded condition missing")
			}
			if strings.Contains(degraded.Message, secretValue) {
				t.Fatalf("Degraded condition message %q echoes the Secret value", degraded.Message)
			}

			close(recorder.Events)
			for ev := range recorder.Events {
				if strings.Contains(ev, secretValue) {
					t.Fatalf("event %q echoes the Secret value", ev)
				}
			}
		})
	}
}
