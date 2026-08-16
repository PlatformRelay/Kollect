// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"crypto/tls"
	"flag"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/platformrelay/kollect/internal/controller"
	"github.com/platformrelay/kollect/internal/inventory"
	"github.com/platformrelay/kollect/internal/validation"
)

type startupConfig struct {
	printVersion                  bool
	metricsAddr                   string
	metricsCertPath               string
	metricsCertName               string
	metricsCertKey                string
	webhookCertPath               string
	webhookCertName               string
	webhookCertKey                string
	enableLeaderElection          bool
	leaderElectionLeaseDuration   time.Duration
	leaderElectionRenewDeadline   time.Duration
	leaderElectionRetryPeriod     time.Duration
	probeAddr                     string
	secureMetrics                 bool
	enableHTTP2                   bool
	inventoryHTTPEnabled          bool
	inventoryHTTPPort             int
	inventoryAuthMode             string
	inventoryAuthCacheTTL         time.Duration
	maxExportBytes                int64
	maxConcurrentTarget           int
	maxConcurrentInventory        int
	maxConcurrentClusterTarget    int
	maxConcurrentClusterInventory int
	reconcileRateLimit            time.Duration
	targetCountResync             time.Duration
	enablePprof                   bool
	pprofAddr                     string
	watchNamespacesRaw            string
	defaultIncludedNamespacesRaw  string
	defaultExcludedNamespacesRaw  string
	scrubKeysRaw                  string
	validatingWebhooksEnabled     bool
	tenantMode                    bool
	allowPrivateSinks             bool
	collectDispatchWorkers        int
	collectDispatchQueueSize      int
	informerResyncPeriod          time.Duration
	collectMetricsSampleInterval  time.Duration
	collectDispatchEnqueueWait    time.Duration
	tlsOpts                       []func(*tls.Config)
}

func bindStartupFlags(fs *flag.FlagSet, cfg *startupConfig) {
	fs.BoolVar(&cfg.printVersion, "version", false, "Print version information and exit (VERSION-01).")
	fs.StringVar(&cfg.metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	fs.StringVar(&cfg.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.BoolVar(&cfg.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	// Leader-election timings (PERF-FIX-02). These were previously left at controller-runtime's
	// defaults — 15s lease / 10s renew / 2s retry — which means ~10 seconds of API-server
	// unreachability terminates the process with "leader election lost". Under a 10k-object load
	// on a real cluster that produced 18 restarts, each one interrupting in-flight exports and
	// leaving the inventory looking short.
	//
	// The defaults below are deliberately MORE PATIENT than controller-runtime's, because the
	// shipped topology is a single replica: there is no standby waiting to take over, so a fast
	// exit buys no failover speed and only converts a transient blip into downtime. Operators
	// running multiple replicas and wanting quicker failover can tighten all three.
	//
	// Invariant, enforced by client-go at startup: retryPeriod < renewDeadline < leaseDuration.
	fs.DurationVar(&cfg.leaderElectionLeaseDuration, "leader-elect-lease-duration", 60*time.Second,
		"Duration non-leaders wait before attempting to acquire leadership. Must be greater than "+
			"--leader-elect-renew-deadline.")
	fs.DurationVar(&cfg.leaderElectionRenewDeadline, "leader-elect-renew-deadline", 40*time.Second,
		"How long the leader retries refreshing leadership before giving up and exiting. This is "+
			"the API-server outage the operator can ride out; raise it for flaky control planes.")
	fs.DurationVar(&cfg.leaderElectionRetryPeriod, "leader-elect-retry-period", 5*time.Second,
		"Interval between leadership acquisition/renewal attempts. Must be less than "+
			"--leader-elect-renew-deadline.")
	fs.BoolVar(&cfg.secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	fs.BoolVar(&cfg.validatingWebhooksEnabled, "validating-webhooks-enabled", true,
		"Register in-process validating webhooks and start the webhook TLS server.")
	fs.BoolVar(&cfg.tenantMode, "tenant-mode", false,
		"Operator runs with namespaced RBAC only (Helm tenantMode). Cluster-scoped kinds "+
			"(KollectClusterTarget/KollectClusterInventory) are rejected at admission.")
	fs.BoolVar(&cfg.allowPrivateSinks, "allow-private-sinks", false,
		"Permit sink endpoints that resolve to RFC1918 / IPv6-ULA (in-cluster ClusterIP) addresses "+
			"(NET-01). Default false (deny). Cluster-admin only via Helm allowPrivateSinks; loopback, "+
			"link-local, cloud-metadata, and file:// stay denied even when enabled.")
	fs.StringVar(&cfg.webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	fs.StringVar(&cfg.webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	fs.StringVar(&cfg.webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	fs.StringVar(&cfg.metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	fs.StringVar(&cfg.metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	fs.StringVar(&cfg.metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	fs.BoolVar(&cfg.enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	fs.BoolVar(&cfg.inventoryHTTPEnabled, "inventory-http-enabled", false,
		"Expose GET /v1alpha1/inventory with aggregated summary JSON (debug only).")
	fs.IntVar(&cfg.inventoryHTTPPort, "inventory-http-port", 8082,
		"Port for the inventory HTTP server when --inventory-http-enabled is set.")
	fs.StringVar(&cfg.inventoryAuthMode, "inventory-auth-mode", inventory.AuthModeKubernetes,
		"Inventory HTTP auth mode: kubernetes (TokenReview+SAR) or disabled (dev/CI only).")
	fs.DurationVar(&cfg.inventoryAuthCacheTTL, "inventory-auth-cache-ttl", 30*time.Second,
		"TTL for in-memory TokenReview/SAR cache (0 disables cache).")
	fs.Int64Var(&cfg.maxExportBytes, "max-export-bytes", validation.MaxExportBytesGlobal(),
		"Global cap for KollectInventory.spec.maxExportBytes and export payload size.")
	fs.IntVar(&cfg.maxConcurrentTarget, "max-concurrent-reconciles-target", 5,
		"Max concurrent KollectTarget reconciles.")
	fs.IntVar(&cfg.maxConcurrentInventory, "max-concurrent-reconciles-inventory", 3,
		"Max concurrent KollectInventory reconciles.")
	fs.IntVar(&cfg.maxConcurrentClusterTarget, "max-concurrent-reconciles-cluster-target", 2,
		"Max concurrent KollectClusterTarget reconciles.")
	fs.IntVar(&cfg.maxConcurrentClusterInventory, "max-concurrent-reconciles-cluster-inventory", 2,
		"Max concurrent KollectClusterInventory reconciles.")
	fs.DurationVar(&cfg.reconcileRateLimit, "reconcile-rate-limit", 0,
		"Base delay for per-item exponential reconcile failure rate limiting (0 = controller-runtime default 5ms).")
	fs.DurationVar(&cfg.targetCountResync, "target-count-resync", controller.DefaultTargetCountResync,
		"How often a Ready KollectTarget is requeued to refresh status.collectedCount (0 = default 60s).")
	fs.BoolVar(&cfg.enablePprof, "enable-pprof", false,
		"Expose Go pprof on --pprof-bind-address (separate from metrics).")
	fs.StringVar(&cfg.pprofAddr, "pprof-bind-address", ":6060",
		"Bind address for pprof when --enable-pprof is set.")
	fs.StringVar(&cfg.watchNamespacesRaw, "watch-namespaces", "",
		"Comma-separated namespaces to watch (empty = all namespaces).")
	fs.StringVar(&cfg.defaultIncludedNamespacesRaw, "default-included-namespaces", "",
		"Comma-separated default Target includedNamespaces when unset on the CRD (Helm defaultIncludedNamespaces).")
	fs.StringVar(&cfg.defaultExcludedNamespacesRaw, "default-excluded-namespaces", "",
		"Comma-separated default Target excludedNamespaces when unset on the CRD (Helm defaultExcludedNamespaces).")
	fs.StringVar(&cfg.scrubKeysRaw, "scrub-keys", "",
		"Comma-separated extra attribute keys to redact before store insert (built-in denylist always applies).")
	fs.IntVar(&cfg.collectDispatchWorkers, "collect-dispatch-workers", 4,
		"Worker goroutines draining the collection informer dispatch queue (PERF-03).")
	fs.IntVar(&cfg.collectDispatchQueueSize, "collect-dispatch-queue-size", 512,
		"Bounded queue depth for collection informer dispatch jobs.")
	fs.DurationVar(&cfg.informerResyncPeriod, "informer-resync-period", 12*time.Hour,
		"Dynamic informer resync period as a correctness backstop (PERF-15).")
	fs.DurationVar(&cfg.collectMetricsSampleInterval, "collect-metrics-sample-interval", 30*time.Second,
		"Minimum interval between domain snapshot metric refreshes per target (PERF-08).")
	fs.DurationVar(&cfg.collectDispatchEnqueueWait, "collect-dispatch-enqueue-wait", 25*time.Millisecond,
		"Brief wait before synchronous dispatch fallback when the queue is full.")
}

// applyLeaderElection copies the leader-election settings from cfg onto opts (PERF-FIX-02).
//
// This exists as a named function purely so the wiring is TESTABLE. Review caught that the three
// assignments previously sat inline in main()'s ctrl.Options literal, where deleting them left
// every test green: the flags would still parse, still be documented, still appear in the chart —
// and reach nothing. Flags that reach nothing is exactly the silent failure PERF-FIX-02 exists to
// prevent, so it gets a test that goes red when the wiring is removed.
//
// Pointers, because ctrl.Options declares these as *time.Duration so that "unset" is
// distinguishable from "zero". They point at cfg's fields, which is safe: cfg outlives the
// manager construction it is passed to.
func applyLeaderElection(opts *ctrl.Options, cfg *startupConfig) {
	opts.LeaderElection = cfg.enableLeaderElection
	opts.LeaseDuration = &cfg.leaderElectionLeaseDuration
	opts.RenewDeadline = &cfg.leaderElectionRenewDeadline
	opts.RetryPeriod = &cfg.leaderElectionRetryPeriod

	// Step down voluntarily when the manager stops, instead of making the successor wait out the
	// full lease. Safe here because main() performs no work after mgr.Start returns — it logs and
	// exits — which is the precondition the controller-runtime scaffold names for this option.
	//
	// This is the counterweight to the patient lease above. Raising LeaseDuration 15s -> 60s would
	// otherwise quadruple how long a NEW leader waits after an ordinary restart (rolling upgrade,
	// image bump, OOM), turning a resilience fix into a 60s reconciliation gap on every deploy of
	// the shipped single replica. Releasing on cancel makes a clean shutdown hand over
	// immediately, so the long lease costs nothing except after an UNCLEAN exit — which is the
	// only case where waiting is the correct behaviour anyway.
	opts.LeaderElectionReleaseOnCancel = true
}
