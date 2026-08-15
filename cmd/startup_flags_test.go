// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"flag"
	"testing"
	"time"

	"github.com/platformrelay/kollect/internal/inventory"
	"github.com/platformrelay/kollect/internal/validation"

	ctrl "sigs.k8s.io/controller-runtime"
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
	if cfg.printVersion {
		t.Fatal("printVersion must default to false")
	}
	if cfg.inventoryAuthMode != inventory.AuthModeKubernetes {
		t.Fatalf("inventoryAuthMode = %q, want %q", cfg.inventoryAuthMode, inventory.AuthModeKubernetes)
	}
	if cfg.allowPrivateSinks {
		t.Fatal("allowPrivateSinks must default to false (NET-01 deny by default)")
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
		"--version=true",
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
		"--allow-private-sinks=true",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !cfg.allowPrivateSinks {
		t.Fatal("--allow-private-sinks=true did not set allowPrivateSinks")
	}
	if !cfg.printVersion {
		t.Fatal("--version=true did not set printVersion")
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

// TestLeaderElectionTimingFlags covers PERF-FIX-02.
//
// The defect: cmd/main.go set only LeaderElection and LeaderElectionID, so controller-runtime's
// defaults applied — 15s lease, 10s renew deadline, 2s retry. Roughly ten seconds of API
// unreachability therefore terminated the process. Observed live under a 10k-object load: 18
// restarts, each "leader election lost" -> exit 1, every one of them interrupting in-flight
// exports.
//
// A single-replica operator gains nothing from an aggressive renew deadline: there is no peer
// waiting to take over, so exiting fast just converts a blip into downtime.
func TestLeaderElectionTimingFlags(t *testing.T) {
	t.Run("defaults are more patient than controller-runtime's", func(t *testing.T) {
		var cfg startupConfig
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		bindStartupFlags(fs, &cfg)
		if err := fs.Parse(nil); err != nil {
			t.Fatalf("Parse: %v", err)
		}

		// controller-runtime defaults are 15s/10s/2s. Anything at or below those reintroduces the
		// crash-loop this story exists to fix, so the assertion is a strict inequality.
		if cfg.leaderElectionLeaseDuration <= 15*time.Second {
			t.Errorf("lease duration default = %s, want > 15s (controller-runtime default)",
				cfg.leaderElectionLeaseDuration)
		}
		if cfg.leaderElectionRenewDeadline <= 10*time.Second {
			t.Errorf("renew deadline default = %s, want > 10s (controller-runtime default)",
				cfg.leaderElectionRenewDeadline)
		}
		if cfg.leaderElectionRetryPeriod <= 0 {
			t.Errorf("retry period default = %s, want > 0", cfg.leaderElectionRetryPeriod)
		}
	})

	t.Run("renew deadline stays below lease duration", func(t *testing.T) {
		var cfg startupConfig
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		bindStartupFlags(fs, &cfg)
		if err := fs.Parse(nil); err != nil {
			t.Fatalf("Parse: %v", err)
		}

		// client-go refuses to start with RenewDeadline >= LeaseDuration. Shipping defaults that
		// violate that would turn a resilience fix into a boot failure.
		if cfg.leaderElectionRenewDeadline >= cfg.leaderElectionLeaseDuration {
			t.Fatalf("renew deadline %s must be < lease duration %s",
				cfg.leaderElectionRenewDeadline, cfg.leaderElectionLeaseDuration)
		}
		if cfg.leaderElectionRetryPeriod >= cfg.leaderElectionRenewDeadline {
			t.Fatalf("retry period %s must be < renew deadline %s",
				cfg.leaderElectionRetryPeriod, cfg.leaderElectionRenewDeadline)
		}
	})

	t.Run("all three are overridable", func(t *testing.T) {
		var cfg startupConfig
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		bindStartupFlags(fs, &cfg)
		args := []string{
			"--leader-elect-lease-duration=137s",
			"--leader-elect-renew-deadline=91s",
			"--leader-elect-retry-period=7s",
		}
		if err := fs.Parse(args); err != nil {
			t.Fatalf("Parse: %v", err)
		}

		if cfg.leaderElectionLeaseDuration != 137*time.Second {
			t.Errorf("lease duration = %s, want 137s", cfg.leaderElectionLeaseDuration)
		}
		if cfg.leaderElectionRenewDeadline != 91*time.Second {
			t.Errorf("renew deadline = %s, want 91s", cfg.leaderElectionRenewDeadline)
		}
		if cfg.leaderElectionRetryPeriod != 7*time.Second {
			t.Errorf("retry period = %s, want 7s", cfg.leaderElectionRetryPeriod)
		}
	})
}

// TestApplyLeaderElection covers the wiring itself, not just flag parsing (PERF-FIX-02, review F2).
//
// Review caught that the three ctrl.Options assignments previously sat inline in main() where
// deleting them left every test green: flags parsed, docs existed, the chart rendered them, and
// nothing reached the manager. This test goes red if applyLeaderElection stops assigning any of
// them.
func TestApplyLeaderElection(t *testing.T) {
	var cfg startupConfig
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	bindStartupFlags(fs, &cfg)
	args := []string{
		"--leader-elect=true",
		"--leader-elect-lease-duration=77s",
		"--leader-elect-renew-deadline=55s",
		"--leader-elect-retry-period=11s",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var opts ctrl.Options
	applyLeaderElection(&opts, &cfg)

	if !opts.LeaderElection {
		t.Error("LeaderElection was not set on ctrl.Options")
	}
	if opts.LeaseDuration == nil || *opts.LeaseDuration != 77*time.Second {
		t.Errorf("LeaseDuration = %v, want 77s", opts.LeaseDuration)
	}
	if opts.RenewDeadline == nil || *opts.RenewDeadline != 55*time.Second {
		t.Errorf("RenewDeadline = %v, want 55s", opts.RenewDeadline)
	}
	if opts.RetryPeriod == nil || *opts.RetryPeriod != 11*time.Second {
		t.Errorf("RetryPeriod = %v, want 11s", opts.RetryPeriod)
	}

	// The counterweight to a patient lease: without releasing on cancel, every ordinary restart
	// makes the successor wait out the full LeaseDuration before reconciling anything.
	if !opts.LeaderElectionReleaseOnCancel {
		t.Error("LeaderElectionReleaseOnCancel is false; a clean shutdown would make the next " +
			"leader wait out the whole lease")
	}
}
