// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package main

import (
	"crypto/tls"
	"flag"
	"os"

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
	cfg := startupConfig{}
	bindStartupFlags(flag.CommandLine, &cfg)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	validation.SetMaxExportBytesGlobal(cfg.maxExportBytes)

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

	if !cfg.enableHTTP2 {
		cfg.tlsOpts = append(cfg.tlsOpts, disableHTTP2)
	}

	var webhookServer webhook.Server
	if cfg.validatingWebhooksEnabled {
		// Initial webhook TLS options
		webhookTLSOpts := cfg.tlsOpts
		webhookServerOptions := webhook.Options{
			TLSOpts: webhookTLSOpts,
		}

		if len(cfg.webhookCertPath) > 0 {
			setupLog.Info(
				"Initializing webhook certificate watcher using provided certificates",
				"webhook-cert-path", cfg.webhookCertPath,
				"webhook-cert-name", cfg.webhookCertName,
				"webhook-cert-key", cfg.webhookCertKey,
			)

			webhookServerOptions.CertDir = cfg.webhookCertPath
			webhookServerOptions.CertName = cfg.webhookCertName
			webhookServerOptions.KeyName = cfg.webhookCertKey
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
		BindAddress:   cfg.metricsAddr,
		SecureServing: cfg.secureMetrics,
		TLSOpts:       cfg.tlsOpts,
	}

	if cfg.secureMetrics {
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
	if len(cfg.metricsCertPath) > 0 {
		setupLog.Info(
			"Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", cfg.metricsCertPath,
			"metrics-cert-name", cfg.metricsCertName,
			"metrics-cert-key", cfg.metricsCertKey,
		)

		metricsServerOptions.CertDir = cfg.metricsCertPath
		metricsServerOptions.CertName = cfg.metricsCertName
		metricsServerOptions.KeyName = cfg.metricsCertKey
	}

	watchNamespaces := operator.ParseWatchNamespaces(cfg.watchNamespacesRaw)
	cacheOpts := operator.CacheOptionsForWatchNamespaces(watchNamespaces)
	if len(watchNamespaces) > 0 {
		setupLog.Info("Restricting manager cache to namespaces", "watchNamespaces", watchNamespaces)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Cache:                  cacheOpts,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: cfg.probeAddr,
		LeaderElection:         cfg.enableLeaderElection,
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
		DispatchWorkers:       cfg.collectDispatchWorkers,
		DispatchQueueSize:     cfg.collectDispatchQueueSize,
		ResyncPeriod:          cfg.informerResyncPeriod,
		MetricsSampleInterval: cfg.collectMetricsSampleInterval,
		DispatchEnqueueWait:   cfg.collectDispatchEnqueueWait,
	})
	if err != nil {
		setupLog.Error(err, "Failed to create collection engine")
		os.Exit(1)
	}
	collectEngine.SetNamespaceDefaults(collect.NamespaceDefaults{
		Included: operator.ParseWatchNamespaces(cfg.defaultIncludedNamespacesRaw),
		Excluded: operator.ParseWatchNamespaces(cfg.defaultExcludedNamespacesRaw),
	})
	collectEngine.SetScrubKeys(operator.ParseScrubKeys(cfg.scrubKeysRaw))

	if err := mgr.Add(collectEngine); err != nil {
		setupLog.Error(err, "Failed to add collection engine")
		os.Exit(1)
	}

	sinkRegistry := sink.NewRegistry()

	ctrlOpts := controller.RuntimeOptions{
		MaxConcurrentTarget:           cfg.maxConcurrentTarget,
		MaxConcurrentInventory:        cfg.maxConcurrentInventory,
		MaxConcurrentClusterTarget:    cfg.maxConcurrentClusterTarget,
		MaxConcurrentClusterInventory: cfg.maxConcurrentClusterInventory,
		ReconcileRateLimitBase:        cfg.reconcileRateLimit,
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
	if cfg.validatingWebhooksEnabled {
		if err := webhookv1alpha1.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Failed to set up webhooks")
			os.Exit(1)
		}
	}
	metrics.Register()
	if cfg.enablePprof {
		if err := mgr.Add(&pprofServer{Addr: cfg.pprofAddr}); err != nil {
			setupLog.Error(err, "Failed to add pprof server")
			os.Exit(1)
		}

		setupLog.Info("pprof enabled", "bindAddress", cfg.pprofAddr)
	}

	if cfg.inventoryHTTPEnabled {
		if cfg.inventoryAuthMode == inventory.AuthModeDisabled {
			setupLog.Info("WARNING: inventory HTTP auth disabled — for local dev and CI only")
		}

		//nolint:gosec // G115: port comes from operator flag (default 8082)
		invSrv := &inventory.Server{
			Enabled: true,
			Port:    int32(cfg.inventoryHTTPPort),
			Store:   collectStore,
			Status:  &inventory.ClientStatusReader{Client: mgr.GetClient()},
			Auth: &inventory.AuthConfig{
				Mode:                cfg.inventoryAuthMode,
				Client:              kubeClient,
				RequireInventoryGet: cfg.inventoryAuthMode == inventory.AuthModeKubernetes,
				CacheTTL:            cfg.inventoryAuthCacheTTL,
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
