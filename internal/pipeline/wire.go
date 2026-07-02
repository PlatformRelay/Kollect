// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/collect"
	"github.com/konih/kollect/internal/sink"
)

// LocalSinkType is the sink type string synthesized when --output is given instead of a
// KollectSnapshotSink YAML (ADR-0801 "--output shorthand").
const LocalSinkType = "local"

const defaultPathTemplate = "inventory/{namespace}/{name}.yaml"

// ResolveSink determines the single sink to use for a pipeline run: a synthetic local sink
// from --output, or the one KollectSnapshotSink loaded from the config directory. Exactly
// one of the two must be present; both or neither is a configuration error.
func ResolveSink(loaded LoadResult, output string) (kollectdevv1alpha1.KollectSinkSpec, error) {
	switch {
	case output != "" && len(loaded.Sinks) > 0:
		return kollectdevv1alpha1.KollectSinkSpec{}, fmt.Errorf(
			"--output and a KollectSnapshotSink found in the config directory are ambiguous; use one or the other")
	case output != "":
		return kollectdevv1alpha1.KollectSinkSpec{Type: LocalSinkType, Endpoint: output}, nil
	case len(loaded.Sinks) == 1:
		return loaded.Sinks[0].Spec.ToKollectSinkSpec(), nil
	case len(loaded.Sinks) == 0:
		return kollectdevv1alpha1.KollectSinkSpec{}, fmt.Errorf(
			"no KollectSnapshotSink found in config directory and no --output given")
	default:
		return kollectdevv1alpha1.KollectSinkSpec{}, fmt.Errorf(
			"%d KollectSnapshotSink objects found in config directory; only one is supported per run (v0.8.0)",
			len(loaded.Sinks))
	}
}

// ResolveSinkSecretData resolves sinkSpec.SecretRef against Secrets loaded from the config
// directory (pipeline mode reads a local v1.Secret manifest instead of the cluster API).
// Returns nil when SecretRef is unset (no credentials required, e.g. type: local).
func ResolveSinkSecretData(sinkSpec kollectdevv1alpha1.KollectSinkSpec, secrets []corev1.Secret) (map[string][]byte, error) {
	if sinkSpec.SecretRef == nil {
		return nil, nil
	}

	for _, s := range secrets {
		if s.Name != sinkSpec.SecretRef.Name {
			continue
		}

		if sinkSpec.SecretRef.Namespace != "" && s.Namespace != sinkSpec.SecretRef.Namespace {
			continue
		}

		return s.Data, nil
	}

	return nil, fmt.Errorf(
		"sink secretRef %q not found in config directory (expected a v1.Secret YAML manifest)", sinkSpec.SecretRef.Name)
}

// ExportTargets serializes each target's collected items from store and writes them via
// backend, rendering the export path from sinkSpec.PathTemplate. In dry-run mode it logs
// what would be written instead of calling backend.Export. A per-target failure is
// collected in errs; it does not stop the remaining targets from being attempted.
func ExportTargets(
	ctx context.Context,
	store *collect.Store,
	targets []kollectdevv1alpha1.KollectTarget,
	backend sink.Backend,
	sinkSpec kollectdevv1alpha1.KollectSinkSpec,
	contextName string,
	dryRun bool,
) (exported int, errs []error) {
	tmpl := sinkSpec.PathTemplate
	if tmpl == "" {
		tmpl = defaultPathTemplate
	}

	cluster := sinkSpec.Cluster
	if cluster == "" {
		cluster = contextName
	}

	for _, target := range targets {
		payload, err := store.MarshalTargetExport(target.Namespace, target.Name, collect.ExportMetadata{Cluster: cluster})
		if err != nil {
			errs = append(errs, fmt.Errorf("target %s/%s: marshal export: %w", target.Namespace, target.Name, err))

			continue
		}

		path := renderPath(tmpl, target.Namespace, target.Name, cluster)

		if dryRun {
			ctrllog.FromContext(ctx).Info("dry-run: would write export",
				"target", target.Namespace+"/"+target.Name, "path", path, "bytes", len(payload))

			continue
		}

		if err := backend.Export(ctx, payload, path); err != nil {
			errs = append(errs, fmt.Errorf("target %s/%s: export: %w", target.Namespace, target.Name, err))

			continue
		}

		exported++
	}

	return exported, errs
}

func renderPath(tmpl, namespace, name, cluster string) string {
	r := strings.NewReplacer(
		"{namespace}", namespace,
		"{name}", name,
		"{cluster}", cluster,
	)

	return r.Replace(tmpl)
}

// ContextResult is the outcome of one resolved kubecontext's full collect+export pass.
type ContextResult struct {
	Context  string
	Exported int
	Errs     []error
	// Fatal is set when this context could not even start (REST config / client / runner
	// construction, or a structural collection failure). A fatal context does not stop the
	// others from running.
	Fatal error
}

// RunAllContexts executes the full collect+export pipeline once per resolved context,
// sequentially. A fatal error in one context does not stop the others (ADR-0801
// "Multi-context selection").
func RunAllContexts(
	ctx context.Context,
	contexts []string,
	kubeconfigPath string,
	loaded LoadResult,
	sinkSpec kollectdevv1alpha1.KollectSinkSpec,
	secretData map[string][]byte,
	registry *sink.Registry,
	scrubKeys []string,
	dryRun bool,
) []ContextResult {
	results := make([]ContextResult, 0, len(contexts))

	for _, contextName := range contexts {
		results = append(results, runOneContext(ctx, contextName, kubeconfigPath, loaded, sinkSpec, secretData, registry, scrubKeys, dryRun))
	}

	return results
}

func runOneContext(
	ctx context.Context,
	contextName string,
	kubeconfigPath string,
	loaded LoadResult,
	sinkSpec kollectdevv1alpha1.KollectSinkSpec,
	secretData map[string][]byte,
	registry *sink.Registry,
	scrubKeys []string,
	dryRun bool,
) ContextResult {
	restCfg, err := clientcmd.BuildConfigFromFlags(contextName, kubeconfigPath)
	if err != nil {
		return ContextResult{Context: contextName, Fatal: fmt.Errorf("build REST config for context %q: %w", contextName, err)}
	}

	dynClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return ContextResult{Context: contextName, Fatal: fmt.Errorf("build dynamic client for context %q: %w", contextName, err)}
	}

	kubeClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return ContextResult{Context: contextName, Fatal: fmt.Errorf("build kube client for context %q: %w", contextName, err)}
	}

	runner, err := collect.NewRunner(restCfg, dynClient, kubeClient, scrubKeys)
	if err != nil {
		return ContextResult{Context: contextName, Fatal: fmt.Errorf("build runner for context %q: %w", contextName, err)}
	}

	runResult, err := runner.Run(ctx, loaded.Profiles, loaded.Targets)
	if err != nil {
		return ContextResult{Context: contextName, Fatal: fmt.Errorf("collect for context %q: %w", contextName, err)}
	}

	for _, skipped := range runResult.SkippedTargets {
		ctrllog.FromContext(ctx).Info("target skipped",
			"context", contextName, "target", skipped.Name, "reason", skipped.Reason)
	}

	backend, err := registry.NewBackend(sinkSpec, sink.BuildContext{Ctx: ctx, SecretData: secretData})
	if err != nil {
		return ContextResult{Context: contextName, Fatal: fmt.Errorf("build sink backend: %w", err)}
	}

	exported, exportErrs := ExportTargets(ctx, runner.Store(), loaded.Targets, backend, sinkSpec, contextName, dryRun)

	errs := make([]error, 0, len(runResult.Errors)+len(exportErrs))
	errs = append(errs, runResult.Errors...)
	errs = append(errs, exportErrs...)

	return ContextResult{Context: contextName, Exported: exported, Errs: errs}
}
