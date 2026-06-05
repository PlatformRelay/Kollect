// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"sync"

	"github.com/konih/kollect/internal/metrics"
)

var registerMetricsOnce sync.Once

func ensureMetricsRegistered() {
	registerMetricsOnce.Do(metrics.Register)
}
