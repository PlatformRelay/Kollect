// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import logf "sigs.k8s.io/controller-runtime/pkg/log"

func closeBackendLogged(backend Backend, reason string) {
	if err := closeBackend(backend); err != nil {
		logf.Log.WithName("sink").Error(err, "closeBackend failed during pool eviction", "reason", reason)
	}
}
