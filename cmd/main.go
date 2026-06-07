// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"crypto/tls"
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/controller"
	"github.com/konih/kollect/internal/inventory"
	"github.com/konih/kollect/internal/metrics"
	"github.com/konih/kollect/internal/operator"
	"github.com/konih/kollect/internal/pprof"
	"github.com/konih/kollect/internal/sink"
	"github.com/konih/kollect/internal/validation"
	webhookv1alpha1 "github.com/konih/kollect/internal/webhook/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(kollectdevv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var inventoryHTTPEnabled bool
	var inventoryHTTPPort int
	var inventoryAuthMode string
	var inventoryAuthCacheTTL time.Duration
	var maxExportBytes int64
	var maxConcurrentTarget int
	var maxConcurrentInventory int
	var maxConcurrentClusterTarget int
	var maxConcurrentClusterInventory int
	var reconcileRateLimit time.Duration
	var enablePprof bool
	var pprofAddr string
	var watchNamespacesRaw string
	var defaultIncludedNamespacesRaw string
	var defaultExcludedNamespacesRaw string
	var scrubKeysRaw string
	var validatingWebhooksEnabled bool
	var collectDispatchWorkers int
	var collectDispatchQueueSize int
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&validatingWebhooksEnabled, "validating-webhooks-enabled", true,
		"Register in-process validating webhooks and start the webhook TLS server.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.BoolVar(&inventoryHTTPEnabled, "inventory-http-enabled", false,
		"Expose GET /v1alpha1/inventory with aggregated summary JSON (debug only).")
	flag.IntVar(&inventoryHTTPPort, "inventory-http-port", 8082,
		"Port for the inventory HTTP server when --inventory-http-enabled is set.")
	flag.StringVar(&inventoryAuthMode, "inventory-auth-mode", inventory.AuthModeKubernetes,
		"Inventory HTTP auth mode: kubernetes (TokenReview+SAR) or disabled (dev/CI only).")
	flag.DurationVar(&inventoryAuthCacheTTL, "inventory-auth-cache-ttl", 30*time.Second,
		"TTL for in-memory TokenReview/SAR cache (0 disables cache).")
	flag.Int64Var(&maxExportBytes, "max-export-bytes", validation.MaxExportBytesGlobal(),
		"Global cap for KollectInventory.spec.maxExportBytes and export payload size.")
	flag.IntVar(&maxConcurrentTarget, "max-concurrent-reconciles-target", 5,
		"Max concurrent KollectTarget reconciles.")
	flag.IntVar(&maxConcurrentInventory, "max-concurrent-reconciles-inventory", 3,
		"Max concurrent KollectInventory reconciles.")
	flag.IntVar(&maxConcurrentClusterTarget, "max-concurrent-reconciles-cluster-target", 2,
		"Max concurrent KollectClusterTarget reconciles.")
	flag.IntVar(&maxConcurrentClusterInventory, "max-concurrent-reconciles-cluster-inventory", 2,
		"Max concurrent KollectClusterInventory reconciles.")
	flag.DurationVar(&reconcileRateLimit, "reconcile-rate-limit", 0,
		"Base delay for per-item exponential reconcile failure rate limiting (0 = controller-runtime default 5ms).")
	flag.BoolVar(&enablePprof, "enable-pprof", false,
		"Expose Go pprof on --pprof-bind-address (separate from metrics).")
	flag.StringVar(&pprofAddr, "pprof-bind-address", ":6060",
		"Bind address for pprof when --enable-pprof is set.")
	flag.StringVar(&watchNamespacesRaw, "watch-namespaces", "",
		"Comma-separated namespaces to watch (empty = all namespaces).")
	flag.StringVar(&defaultIncludedNamespacesRaw, "default-included-namespaces", "",
		"Comma-separated default Target includedNamespaces when unset on the CRD (Helm defaultIncludedNamespaces).")
	flag.StringVar(&defaultExcludedNamespacesRaw, "default-excluded-namespaces", "",
		"Comma-separated default Target excludedNamespaces when unset on the CRD (Helm defaultExcludedNamespaces).")
	flag.StringVar(&scrubKeysRaw, "scrub-keys", "",
		"Comma-separated extra attribute keys to redact before store insert (built-in denylist always applies).")
	flag.IntVar(&collectDispatchWorkers, "collect-dispatch-workers", 4,
		"Worker goroutines draining the collection informer dispatch queue (PERF-03).")
	flag.IntVar(&collectDispatchQueueSize, "collect-dispatch-queue-size", 512,
		"Bounded queue depth for collection informer dispatch jobs.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	validation.SetMaxExportBytesGlobal(maxExportBytes)

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("Disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	var webhookServer webhook.Server
	if validatingWebhooksEnabled {
		// Initial webhook TLS options
		webhookTLSOpts := tlsOpts
		webhookServerOptions := webhook.Options{
			TLSOpts: webhookTLSOpts,
		}

		if len(webhookCertPath) > 0 {
			setupLog.Info("Initializing webhook certificate watcher using provided certificates",
				"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

			webhookServerOptions.CertDir = webhookCertPath
			webhookServerOptions.CertName = webhookCertName
			webhookServerOptions.KeyName = webhookCertKey
		}

		webhookServer = webhook.NewServer(webhookServerOptions)
	} else {
		setupLog.Info("Validating webhooks disabled — skipping webhook server and admission handlers")
	}

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	watchNamespaces := operator.ParseWatchNamespaces(watchNamespacesRaw)
	cacheOpts := operator.CacheOptionsForWatchNamespaces(watchNamespaces)
	if len(watchNamespaces) > 0 {
		setupLog.Info("Restricting manager cache to namespaces", "watchNamespaces", watchNamespaces)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Cache:                  cacheOpts,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "3274ac8a.kollect.dev",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	dynamicClient, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "Failed to create dynamic client")
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "Failed to create kubernetes client")
		os.Exit(1)
	}

	collectStore := collect.NewStore()
	collectEngine, err := collect.NewEngine(dynamicClient, kubeClient, collectStore, collect.EngineConfig{
		DispatchWorkers:   collectDispatchWorkers,
		DispatchQueueSize: collectDispatchQueueSize,
	})
	if err != nil {
		setupLog.Error(err, "Failed to create collection engine")
		os.Exit(1)
	}
	collectEngine.SetNamespaceDefaults(collect.NamespaceDefaults{
		Included: operator.ParseWatchNamespaces(defaultIncludedNamespacesRaw),
		Excluded: operator.ParseWatchNamespaces(defaultExcludedNamespacesRaw),
	})
	collectEngine.SetScrubKeys(operator.ParseScrubKeys(scrubKeysRaw))

	if err := mgr.Add(collectEngine); err != nil {
		setupLog.Error(err, "Failed to add collection engine")
		os.Exit(1)
	}

	sinkRegistry := sink.NewRegistry()

	ctrlOpts := controller.RuntimeOptions{
		MaxConcurrentTarget:           maxConcurrentTarget,
		MaxConcurrentInventory:        maxConcurrentInventory,
		MaxConcurrentClusterTarget:    maxConcurrentClusterTarget,
		MaxConcurrentClusterInventory: maxConcurrentClusterInventory,
		ReconcileRateLimitBase:        reconcileRateLimit,
	}

	if err := (&controller.KollectTargetReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Engine:  collectEngine,
		Options: ctrlOpts,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollecttarget")
		os.Exit(1)
	}
	if err := (&controller.KollectInventoryReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Store:    collectStore,
		Registry: sinkRegistry,
		Options:  ctrlOpts,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollectinventory")
		os.Exit(1)
	}
	if err := (&controller.FamilySnapshotSinkReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollectsnapshotsink")
		os.Exit(1)
	}
	if err := (&controller.FamilyClusterSnapshotSinkReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollectclustersnapshotsink")
		os.Exit(1)
	}
	if err := (&controller.FamilyDatabaseSinkReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollectdatabasesink")
		os.Exit(1)
	}
	if err := (&controller.FamilyClusterDatabaseSinkReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollectclusterdatabasesink")
		os.Exit(1)
	}
	if err := (&controller.FamilyEventSinkReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollecteventsink")
		os.Exit(1)
	}
	if err := (&controller.FamilyClusterEventSinkReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollectclustereventsink")
		os.Exit(1)
	}
	if err := (&controller.KollectConnectionTestReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollectconnectiontest")
		os.Exit(1)
	}
	if err := (&controller.KollectClusterTargetReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Engine:  collectEngine,
		Options: ctrlOpts,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollectclustertarget")
		os.Exit(1)
	}
	if err := (&controller.KollectClusterInventoryReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Store:    collectStore,
		Engine:   collectEngine,
		Registry: sinkRegistry,
		Options:  ctrlOpts,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "kollectclusterinventory")
		os.Exit(1)
	}
	if validatingWebhooksEnabled {
		if err := webhookv1alpha1.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Failed to set up webhooks")
			os.Exit(1)
		}
	}
	metrics.Register()
	if enablePprof {
		if err := mgr.Add(&pprof.Server{Addr: pprofAddr}); err != nil {
			setupLog.Error(err, "Failed to add pprof server")
			os.Exit(1)
		}

		setupLog.Info("pprof enabled", "bindAddress", pprofAddr)
	}

	if inventoryHTTPEnabled {
		if inventoryAuthMode == inventory.AuthModeDisabled {
			setupLog.Info("WARNING: inventory HTTP auth disabled — for local dev and CI only")
		}

		//nolint:gosec // G115: port comes from operator flag (default 8082)
		invSrv := &inventory.Server{
			Enabled: true,
			Port:    int32(inventoryHTTPPort),
			Store:   collectStore,
			Status:  &inventory.ClientStatusReader{Client: mgr.GetClient()},
			Auth: &inventory.AuthConfig{
				Mode:                inventoryAuthMode,
				Client:              kubeClient,
				RequireInventoryGet: inventoryAuthMode == inventory.AuthModeKubernetes,
				CacheTTL:            inventoryAuthCacheTTL,
			},
		}
		if err := mgr.Add(invSrv); err != nil {
			setupLog.Error(err, "Failed to add inventory HTTP server")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}
