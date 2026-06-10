// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"crypto/tls"
	"flag"
	"time"

	"github.com/konih/kollect/internal/inventory"
	"github.com/konih/kollect/internal/validation"
)

type startupConfig struct {
	metricsAddr                   string
	metricsCertPath               string
	metricsCertName               string
	metricsCertKey                string
	webhookCertPath               string
	webhookCertName               string
	webhookCertKey                string
	enableLeaderElection          bool
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
	enablePprof                   bool
	pprofAddr                     string
	watchNamespacesRaw            string
	defaultIncludedNamespacesRaw  string
	defaultExcludedNamespacesRaw  string
	scrubKeysRaw                  string
	validatingWebhooksEnabled     bool
	tenantMode                    bool
	collectDispatchWorkers        int
	collectDispatchQueueSize      int
	informerResyncPeriod          time.Duration
	collectMetricsSampleInterval  time.Duration
	collectDispatchEnqueueWait    time.Duration
	tlsOpts                       []func(*tls.Config)
}

func bindStartupFlags(fs *flag.FlagSet, cfg *startupConfig) {
	fs.StringVar(&cfg.metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	fs.StringVar(&cfg.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.BoolVar(&cfg.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	fs.BoolVar(&cfg.secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	fs.BoolVar(&cfg.validatingWebhooksEnabled, "validating-webhooks-enabled", true,
		"Register in-process validating webhooks and start the webhook TLS server.")
	fs.BoolVar(&cfg.tenantMode, "tenant-mode", false,
		"Operator runs with namespaced RBAC only (Helm tenantMode). Cluster-scoped kinds "+
			"(KollectClusterTarget/KollectClusterInventory) are rejected at admission.")
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
