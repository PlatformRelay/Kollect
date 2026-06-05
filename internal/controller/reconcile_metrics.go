// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"time"

	"github.com/konih/kollect/internal/metrics"
)

func trackReconcile(controller string) (finish func(err error)) {
	metrics.ReconcileInFlight.WithLabelValues(controller).Inc()

	start := time.Now()

	return func(err error) {
		metrics.ReconcileInFlight.WithLabelValues(controller).Dec()
		metrics.ReconcileDurationSeconds.WithLabelValues(controller).Observe(time.Since(start).Seconds())

		result := metrics.ResultSuccess
		if err != nil {
			result = metrics.ResultFailure
		}

		metrics.ReconcileTotal.WithLabelValues(controller, result).Inc()
	}
}
