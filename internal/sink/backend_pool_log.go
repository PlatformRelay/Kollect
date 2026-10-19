// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// sinkLog returns the pool's logger; a test seam so redaction tests can capture its lines.
var sinkLog = func() logr.Logger { return logf.Log.WithName("sink") }

func closeBackendLogged(backend Backend, reason string) {
	if err := closeBackend(backend); err != nil {
		sinkLog().Error(err, "closeBackend failed during pool eviction", "reason", reason)
	}
}
