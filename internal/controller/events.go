// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

func recordWarning(recorder record.EventRecorder, obj runtime.Object, reason, message string) {
	if recorder == nil {
		return
	}

	recorder.Event(obj, corev1.EventTypeWarning, reason, message)
}

func recordNormal(recorder record.EventRecorder, obj runtime.Object, reason, message string) {
	if recorder == nil {
		return
	}

	recorder.Event(obj, corev1.EventTypeNormal, reason, message)
}
