// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

// ErrSecretEnvVarNotSet is returned when a config-dir Secret value is a ${env:VAR}
// placeholder but VAR is unset or empty in the process environment.
var ErrSecretEnvVarNotSet = errors.New("environment variable for secret placeholder not set")

// envPlaceholderPattern matches a Secret value that consists entirely of one
// ${env:VAR_NAME} placeholder. Full-value match only — a value that merely contains a
// placeholder mid-string is left verbatim, so a real credential can never be partially
// rewritten.
var envPlaceholderPattern = regexp.MustCompile(`^\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}$`)

// ResolveSinkSecretData resolves sinkSpec.SecretRef against Secrets loaded from the config
// directory (pipeline mode reads a local v1.Secret manifest instead of the cluster API).
// Returns nil when SecretRef is unset (no credentials required, e.g. type: local).
//
// The returned map merges the manifest's stringData over data (apiserver semantics —
// client-side decoding never performs that merge), then substitutes any value that is
// exactly a ${env:VAR} placeholder from the process environment. This is the pipeline
// "secretRef.env" binding: CI systems inject the credential via the environment (e.g. a
// GitLab masked variable) and the committed Secret manifest carries only the placeholder.
// An unset or empty variable is a hard error naming the secret, key, and variable —
// never a silently empty credential.
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

		return resolveSecretValues(&s)
	}

	return nil, fmt.Errorf(
		"sink secretRef %q not found in config directory (expected a v1.Secret YAML manifest)", sinkSpec.SecretRef.Name)
}

// resolveSecretValues builds the effective credential map for one config-dir Secret:
// data first, stringData overlaid, then ${env:VAR} placeholder substitution. The source
// Secret is never mutated.
func resolveSecretValues(secret *corev1.Secret) (map[string][]byte, error) {
	merged := make(map[string][]byte, len(secret.Data)+len(secret.StringData))
	for k, v := range secret.Data {
		merged[k] = v
	}

	for k, v := range secret.StringData {
		merged[k] = []byte(v)
	}

	for key, value := range merged {
		m := envPlaceholderPattern.FindStringSubmatch(string(value))
		if m == nil {
			continue
		}

		envName := m[1]
		envValue := os.Getenv(envName)
		if envValue == "" {
			return nil, fmt.Errorf(
				"secret %q key %q: %w: %s (set it in the CI environment, e.g. a GitLab masked variable)",
				secret.Name, key, ErrSecretEnvVarNotSet, envName)
		}

		merged[key] = []byte(envValue)
	}

	return merged, nil
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
	// Skipped is the number of targets the runner could not fully collect (forbidden,
	// transient List error, or GVK not found in the cluster) -- see collect.SkippedTarget.
	// It is folded into exit-code aggregation: skips alone (no other errors) still degrade
	// a run below ExitSuccess, matching the collect.RunResult contract this bridges from.
	Skipped int
	Errs    []error
	// Fatal is set when this context could not even start (REST config / client / runner
	// construction, or a structural collection failure). A fatal context does not stop the
	// others from running.
	Fatal error
}

// ApplyNamespaceOverride returns a copy of targets with spec.includedNamespaces forced to
// [namespace] for every target, overriding whatever namespace selectors/lists they declared
// (the --namespace flag's documented behavior). A blank namespace is a no-op: targets are
// returned unchanged.
func ApplyNamespaceOverride(targets []kollectdevv1alpha1.KollectTarget, namespace string) []kollectdevv1alpha1.KollectTarget {
	if namespace == "" {
		return targets
	}

	out := make([]kollectdevv1alpha1.KollectTarget, len(targets))
	for i, t := range targets {
		t.Spec.IncludedNamespaces = []string{namespace}
		out[i] = t
	}

	return out
}

// buildContextResult assembles a ContextResult from a completed collection pass. It is
// separated from runOneContext so the RunResult -> ContextResult bridge -- in particular,
// that SkippedTargets must survive into the exit-code decision, not just get logged -- is
// unit-testable without a cluster.
func buildContextResult(contextName string, runResult collect.RunResult, exported int, exportErrs []error) ContextResult {
	// Concatenate run and export errors onto a nil slice; append grows as needed. We
	// deliberately avoid a computed make() capacity (len(a)+len(b)) here -- the sum is a
	// size-computation CodeQL flags as a potential overflow, and pre-sizing a handful of
	// error values buys nothing.
	//nolint:prealloc // computed make() capacity is flagged by CodeQL go/size-computation-overflow; see comment above
	var errs []error
	errs = append(errs, runResult.Errors...)
	errs = append(errs, exportErrs...)

	return ContextResult{
		Context:  contextName,
		Exported: exported,
		Skipped:  len(runResult.SkippedTargets),
		Errs:     errs,
	}
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

// restConfigForContext builds a *rest.Config for the named context from kubeconfigPath.
//
// clientcmd.BuildConfigFromFlags(masterUrl, kubeconfigPath) is NOT the right call here even
// though it looks like it: its first argument is a master-URL override, not a context name.
// Passing a context name there silently sets the API server host to that literal string and
// never selects the context at all -- every named context then dials a bogus host. Context
// selection has to go through ConfigOverrides.CurrentContext instead.
func restConfigForContext(kubeconfigPath, contextName string) (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
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
	restCfg, err := restConfigForContext(kubeconfigPath, contextName)
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

	return buildContextResult(contextName, runResult, exported, exportErrs)
}
