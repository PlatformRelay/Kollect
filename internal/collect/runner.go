// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// Runner performs a one-shot List-based collection pass for a set of profiles and targets
// (ADR-0801 pipeline CLI mode). Unlike Engine, it has no informers, no dispatch queue, and
// no resync period: construct it, call Run once, read Store, discard.
type Runner struct {
	dynClient  dynamic.Interface
	kubeClient kubernetes.Interface
	mapper     meta.RESTMapper
	extractor  *Extractor
	scrubber   *Scrubber
	store      *Store
	log        logr.Logger
}

// NewRunner constructs a Runner using a discovery-based RESTMapper built from restCfg.
// scrubKeys supplements the built-in sensitive-key denylist (ADR-0303).
func NewRunner(
	restCfg *rest.Config,
	dynClient dynamic.Interface,
	kubeClient kubernetes.Interface,
	scrubKeys []string,
) (*Runner, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("build discovery client: %w", err)
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))

	return NewRunnerWithMapper(dynClient, kubeClient, mapper, scrubKeys)
}

// NewRunnerWithMapper constructs a Runner with an injected RESTMapper. This is the test
// seam: unit tests provide a static mapper so GVK->GVR resolution never hits a live API
// server. NewRunner is a thin wrapper that builds a discovery-backed mapper for real use.
func NewRunnerWithMapper(
	dynClient dynamic.Interface,
	kubeClient kubernetes.Interface,
	mapper meta.RESTMapper,
	scrubKeys []string,
) (*Runner, error) {
	extractor, err := NewExtractor()
	if err != nil {
		return nil, fmt.Errorf("build extractor: %w", err)
	}

	return &Runner{
		dynClient:  dynClient,
		kubeClient: kubeClient,
		mapper:     mapper,
		extractor:  extractor,
		scrubber:   NewScrubber(scrubKeys),
		store:      NewStore(),
		log:        logr.Discard(),
	}, nil
}

// Store returns the populated store after Run has been called. Caller reads it to produce exports.
func (r *Runner) Store() *Store {
	return r.store
}

// SkippedTarget records a target (or the whole run for that target) that could not be
// fully collected.
type SkippedTarget struct {
	// Name is "<namespace>/<name>" of the KollectTarget.
	Name string
	// Reason is one of "profile-not-found", "gvk-not-found", "forbidden", "transient".
	Reason string
}

// ExtractionFailure is a redacted record of a per-object attribute extraction failure
// (REL-02). It carries target and object identity only — never the object payload or
// secret-bearing attribute values. Failures are aggregated deterministically into
// RunResult; successfully extracted sibling objects are still retained and exported.
type ExtractionFailure struct {
	// Target is "<namespace>/<name>" of the KollectTarget.
	Target string
	// Namespace is the object's namespace (empty for cluster-scoped resources).
	Namespace string
	// Name is the object's name.
	Name string
	// UID is the object's Kubernetes UID.
	UID string
	// Reason is an actionable, redacted explanation (attribute name + error class).
	Reason string
}

// Error implements error so pipeline exit classification can fold extraction failures
// into ContextResult.Errs without losing identity in the message.
func (f ExtractionFailure) Error() string {
	object := f.Name
	if f.Namespace != "" {
		object = f.Namespace + "/" + f.Name
	}

	return fmt.Sprintf("target %s: extract %s (uid=%s): %s", f.Target, object, f.UID, f.Reason)
}

// RunResult summarises the outcome of a Run call.
type RunResult struct {
	ItemCount      int
	SkippedTargets []SkippedTarget
	// ExtractionFailures holds per-object required-attribute extraction failures.
	// Presence of any entry means the run is partial/degraded (REL-02): good objects are
	// still counted in ItemCount, failing objects are omitted from the store.
	ExtractionFailures []ExtractionFailure
	// Errors holds fatal per-target errors: failures in a step that isn't a per-namespace
	// List call (currently: namespace resolution). Forbidden/transient/gvk-not-found List
	// failures are non-fatal and recorded in SkippedTargets instead.
	Errors []error
}

// Degraded reports whether the run was incomplete: skipped targets, fatal per-target
// errors, or per-object extraction failures. A degraded result must not be classified as
// full success by pipeline exit-code aggregation.
func (r RunResult) Degraded() bool {
	return len(r.SkippedTargets) > 0 || len(r.Errors) > 0 || len(r.ExtractionFailures) > 0
}

// Run executes the collection pass for all profiles + targets. Partial failures (forbidden
// GVKs, empty namespace lists, RBAC-forbidden lists, and per-object extraction failures) are
// recorded in the returned RunResult, not returned as an error. Successfully extracted
// objects are retained; failing objects are omitted with a structured ExtractionFailure
// (REL-02). Only a structural failure (extractor construction, which happens in NewRunner)
// prevents Run from returning a result at all.
func (r *Runner) Run(
	ctx context.Context,
	profiles []kollectdevv1alpha1.KollectProfile,
	targets []kollectdevv1alpha1.KollectTarget,
) (RunResult, error) {
	profileByName := make(map[string]kollectdevv1alpha1.KollectProfile, len(profiles))
	for _, p := range profiles {
		profileByName[p.Name] = p
	}

	var result RunResult

	for _, target := range targets {
		profile, ok := profileByName[target.Spec.ProfileRef]
		if !ok {
			result.SkippedTargets = append(result.SkippedTargets, SkippedTarget{
				Name:   targetKeyName(target),
				Reason: "profile-not-found",
			})

			continue
		}

		r.runTarget(ctx, profile, target, &result)
	}

	sortExtractionFailures(result.ExtractionFailures)

	return result, nil
}

func targetKeyName(target kollectdevv1alpha1.KollectTarget) string {
	return target.Namespace + "/" + target.Name
}

func (r *Runner) runTarget(
	ctx context.Context,
	profile kollectdevv1alpha1.KollectProfile,
	target kollectdevv1alpha1.KollectTarget,
	result *RunResult,
) {
	gvk := profile.Spec.TargetGVK

	mapping, err := r.mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
	if err != nil {
		result.SkippedTargets = append(result.SkippedTargets, SkippedTarget{
			Name:   targetKeyName(target),
			Reason: "gvk-not-found",
		})

		return
	}

	namespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace

	namespaces, err := r.resolveNamespaces(ctx, target, namespaced)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("target %s: resolve namespaces: %w", targetKeyName(target), err))

		return
	}

	labelSelector := labelSelectorString(target.Spec.LabelSelector)
	nameFilter := namesSet(target.Spec.Names)

	for _, ns := range namespaces {
		count, extractFails, listErr := r.listAndExtract(ctx, mapping.Resource, ns, labelSelector, profile, target, nameFilter)

		switch {
		case listErr == nil:
			result.ItemCount += count
			result.ExtractionFailures = append(result.ExtractionFailures, extractFails...)
		case kollecterrors.IsForbidden(listErr):
			result.SkippedTargets = append(result.SkippedTargets, SkippedTarget{
				Name:   targetKeyName(target),
				Reason: "forbidden",
			})
		default:
			result.SkippedTargets = append(result.SkippedTargets, SkippedTarget{
				Name:   targetKeyName(target),
				Reason: "transient",
			})
		}
	}
}

// resolveNamespaces determines the effective namespace list for target. Cluster-scoped
// resources return a single "" sentinel entry (skip per-namespace listing entirely).
func (r *Runner) resolveNamespaces(
	ctx context.Context,
	target kollectdevv1alpha1.KollectTarget,
	namespaced bool,
) ([]string, error) {
	if !namespaced {
		return []string{""}, nil
	}

	namespaces, err := r.baseNamespaces(ctx, target)
	if err != nil {
		return nil, err
	}

	return excludeNamespaces(namespaces, target.Spec.ExcludedNamespaces), nil
}

func (r *Runner) baseNamespaces(ctx context.Context, target kollectdevv1alpha1.KollectTarget) ([]string, error) {
	if len(target.Spec.IncludedNamespaces) > 0 {
		return append([]string(nil), target.Spec.IncludedNamespaces...), nil
	}

	opts := metav1.ListOptions{}

	if target.Spec.NamespaceSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(target.Spec.NamespaceSelector)
		if err != nil {
			return nil, fmt.Errorf("parse namespaceSelector: %w", err)
		}

		opts.LabelSelector = selector.String()
	}

	nsList, err := r.kubeClient.CoreV1().Namespaces().List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	namespaces := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		namespaces = append(namespaces, ns.Name)
	}

	return namespaces, nil
}

func excludeNamespaces(namespaces, excluded []string) []string {
	if len(excluded) == 0 {
		return namespaces
	}

	excludedSet := make(map[string]struct{}, len(excluded))
	for _, ns := range excluded {
		excludedSet[ns] = struct{}{}
	}

	filtered := make([]string, 0, len(namespaces))

	for _, ns := range namespaces {
		if _, skip := excludedSet[ns]; !skip {
			filtered = append(filtered, ns)
		}
	}

	return filtered
}

func (r *Runner) listAndExtract(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace string,
	labelSelector string,
	profile kollectdevv1alpha1.KollectProfile,
	target kollectdevv1alpha1.KollectTarget,
	nameFilter map[string]struct{},
) (int, []ExtractionFailure, error) {
	opts := metav1.ListOptions{LabelSelector: labelSelector}

	var (
		items []unstructured.Unstructured
		err   error
	)

	if namespace == "" {
		list, listErr := r.dynClient.Resource(gvr).List(ctx, opts)
		err = listErr

		if list != nil {
			items = list.Items
		}
	} else {
		list, listErr := r.dynClient.Resource(gvr).Namespace(namespace).List(ctx, opts)
		err = listErr

		if list != nil {
			items = list.Items
		}
	}

	if err != nil {
		return 0, nil, err
	}

	count := 0
	var failures []ExtractionFailure

	for i := range items {
		item := items[i]

		if len(nameFilter) > 0 {
			if _, ok := nameFilter[item.GetName()]; !ok {
				continue
			}
		}

		attrs, extractErr := r.extractor.Extract(&item, profile.Spec.Attributes)
		if extractErr != nil {
			// Required attribute failed: omit this object, keep siblings, record a redacted
			// structured failure (REL-02). Never attach the object payload or attribute values.
			failures = append(failures, ExtractionFailure{
				Target:    targetKeyName(target),
				Namespace: item.GetNamespace(),
				Name:      item.GetName(),
				UID:       string(item.GetUID()),
				Reason:    redactExtractionReason(extractErr),
			})

			continue
		}

		attrs = r.scrubber.ScrubAttributes(attrs)

		r.store.Upsert(Item{
			TargetNamespace: target.Namespace,
			TargetName:      target.Name,
			Namespace:       item.GetNamespace(),
			Name:            item.GetName(),
			Group:           gvr.Group,
			Version:         gvr.Version,
			Kind:            profile.Spec.TargetGVK.Kind,
			UID:             string(item.GetUID()),
			Attributes:      attrs,
		})
		count++
	}

	return count, failures, nil
}

// redactExtractionReason keeps the extractor's attribute-scoped message and strips anything
// that looks like an embedded payload. Extractor errors are already attribute-scoped
// (`attribute "x": …`); we still bound length so a pathological backend cannot flood logs.
func redactExtractionReason(err error) string {
	if err == nil {
		return "extraction failed"
	}

	msg := err.Error()
	const maxReasonLen = 256
	if len(msg) > maxReasonLen {
		msg = msg[:maxReasonLen] + "…"
	}

	return msg
}

func sortExtractionFailures(failures []ExtractionFailure) {
	sort.SliceStable(failures, func(i, j int) bool {
		a, b := failures[i], failures[j]
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}

		return a.UID < b.UID
	})
}

func labelSelectorString(sel *metav1.LabelSelector) string {
	if sel == nil {
		return ""
	}

	selector, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return ""
	}

	return selector.String()
}

func namesSet(names []string) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}

	return set
}
