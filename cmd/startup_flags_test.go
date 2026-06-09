// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"flag"
	"testing"
	"time"

	"github.com/konih/kollect/internal/inventory"
	"github.com/konih/kollect/internal/validation"
)

func TestBindStartupFlags_Defaults(t *testing.T) {
	t.Parallel()

	fs := flag.NewFlagSet("defaults", flag.ContinueOnError)
	cfg := startupConfig{}
	bindStartupFlags(fs, &cfg)

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if cfg.metricsAddr != "0" || cfg.probeAddr != ":8081" {
		t.Fatalf("unexpected default addresses: metrics=%q probe=%q", cfg.metricsAddr, cfg.probeAddr)
	}
	if !cfg.secureMetrics || cfg.enableHTTP2 || cfg.inventoryHTTPEnabled {
		t.Fatalf("unexpected default booleans: secure=%t http2=%t invHTTP=%t",
			cfg.secureMetrics, cfg.enableHTTP2, cfg.inventoryHTTPEnabled)
	}
	if cfg.inventoryAuthMode != inventory.AuthModeKubernetes {
		t.Fatalf("inventoryAuthMode = %q, want %q", cfg.inventoryAuthMode, inventory.AuthModeKubernetes)
	}
	if cfg.maxExportBytes != validation.MaxExportBytesGlobal() {
		t.Fatalf("maxExportBytes = %d, want %d", cfg.maxExportBytes, validation.MaxExportBytesGlobal())
	}
	if cfg.collectDispatchWorkers != 4 || cfg.collectDispatchQueueSize != 512 {
		t.Fatalf(
			"unexpected dispatch defaults: workers=%d queue=%d",
			cfg.collectDispatchWorkers,
			cfg.collectDispatchQueueSize,
		)
	}
	if cfg.informerResyncPeriod != 12*time.Hour || cfg.collectMetricsSampleInterval != 30*time.Second {
		t.Fatalf(
			"unexpected duration defaults: resync=%s sample=%s",
			cfg.informerResyncPeriod,
			cfg.collectMetricsSampleInterval,
		)
	}
}

func TestBindStartupFlags_ParsesCustomValues(t *testing.T) {
	t.Parallel()

	fs := flag.NewFlagSet("custom", flag.ContinueOnError)
	cfg := startupConfig{}
	bindStartupFlags(fs, &cfg)

	args := []string{
		"--metrics-bind-address=:8443",
		"--health-probe-bind-address=:18081",
		"--leader-elect=true",
		"--metrics-secure=false",
		"--validating-webhooks-enabled=false",
		"--inventory-http-enabled=true",
		"--inventory-http-port=19090",
		"--inventory-auth-mode=disabled",
		"--inventory-auth-cache-ttl=45s",
		"--max-export-bytes=12345",
		"--max-concurrent-reconciles-target=9",
		"--max-concurrent-reconciles-inventory=8",
		"--max-concurrent-reconciles-cluster-target=7",
		"--max-concurrent-reconciles-cluster-inventory=6",
		"--reconcile-rate-limit=2s",
		"--enable-pprof=true",
		"--pprof-bind-address=:17070",
		"--watch-namespaces=team-a,team-b",
		"--default-included-namespaces=core",
		"--default-excluded-namespaces=kube-system",
		"--scrub-keys=password,token",
		"--collect-dispatch-workers=11",
		"--collect-dispatch-queue-size=99",
		"--informer-resync-period=1h",
		"--collect-metrics-sample-interval=10s",
		"--collect-dispatch-enqueue-wait=100ms",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if cfg.metricsAddr != ":8443" || cfg.probeAddr != ":18081" {
		t.Fatalf("addresses = %q/%q, want :8443/:18081", cfg.metricsAddr, cfg.probeAddr)
	}
	if !cfg.enableLeaderElection || cfg.secureMetrics || cfg.validatingWebhooksEnabled {
		t.Fatalf("boolean parsing mismatch: leader=%t secure=%t webhooks=%t",
			cfg.enableLeaderElection, cfg.secureMetrics, cfg.validatingWebhooksEnabled)
	}
	if !cfg.inventoryHTTPEnabled || cfg.inventoryHTTPPort != 19090 || cfg.inventoryAuthMode != inventory.AuthModeDisabled {
		t.Fatalf("inventory HTTP config mismatch: enabled=%t port=%d mode=%q",
			cfg.inventoryHTTPEnabled, cfg.inventoryHTTPPort, cfg.inventoryAuthMode)
	}
	if cfg.reconcileRateLimit != 2*time.Second || cfg.collectDispatchEnqueueWait != 100*time.Millisecond {
		t.Fatalf("duration parsing mismatch: rate=%s enqueueWait=%s", cfg.reconcileRateLimit, cfg.collectDispatchEnqueueWait)
	}
	if cfg.collectDispatchWorkers != 11 || cfg.collectDispatchQueueSize != 99 {
		t.Fatalf("dispatch tuning mismatch: workers=%d queue=%d", cfg.collectDispatchWorkers, cfg.collectDispatchQueueSize)
	}
}
