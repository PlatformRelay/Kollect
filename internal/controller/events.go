// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	"github.com/platformrelay/kollect/internal/redact"
)

// recordWarning and recordNormal are the single choke-point for every Event
// this package emits (K-23): Event messages are persisted in etcd, so free-
// form error text reaching them is redacted here regardless of caller.
func recordWarning(recorder record.EventRecorder, obj runtime.Object, reason, message string) {
	if recorder == nil {
		return
	}

	recorder.Event(obj, corev1.EventTypeWarning, reason, redact.Text(message))
}

func recordNormal(recorder record.EventRecorder, obj runtime.Object, reason, message string) {
	if recorder == nil {
		return
	}

	recorder.Event(obj, corev1.EventTypeNormal, reason, redact.Text(message))
}
