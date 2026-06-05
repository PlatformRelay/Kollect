// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

func TestRecordEventsNilRecorder(t *testing.T) {
	t.Parallel()

	obj := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
	}
	recordWarning(nil, obj, "Test", "warn")
	recordNormal(nil, obj, "Test", "ok")
}
