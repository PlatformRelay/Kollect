// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

package collect

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/metrics"
)

const (
	defaultInformerResync        = 12 * time.Hour
	defaultDispatchWorkers       = 4
	defaultDispatchQueueSize     = 512
	defaultMetricsSampleInterval = 30 * time.Second
	defaultDispatchEnqueueWait   = 25 * time.Millisecond
)

// EngineConfig tunes collection engine concurrency and observability (PERF-03/08/15).
type EngineConfig struct {
	DispatchWorkers       int
	DispatchQueueSize     int
	ResyncPeriod          time.Duration
	MetricsSampleInterval time.Duration
	DispatchEnqueueWait   time.Duration
}

func normalizeEngineConfig(cfg EngineConfig) EngineConfig {
	if cfg.DispatchWorkers <= 0 {
		cfg.DispatchWorkers = defaultDispatchWorkers
	}
	if cfg.DispatchQueueSize <= 0 {
		cfg.DispatchQueueSize = defaultDispatchQueueSize
	}
	if cfg.ResyncPeriod <= 0 {
		cfg.ResyncPeriod = defaultInformerResync
	}
	if cfg.MetricsSampleInterval <= 0 {
		cfg.MetricsSampleInterval = defaultMetricsSampleInterval
	}
	if cfg.DispatchEnqueueWait < 0 {
		cfg.DispatchEnqueueWait = 0
	}

	return cfg
}

type dispatchJob struct {
	ctx     context.Context
	gvr     schema.GroupVersionResource
	obj     interface{}
	deleted bool
}

type targetState struct {
	target              kollectdevv1alpha1.KollectTarget
	profile             kollectdevv1alpha1.KollectProfile
	effectiveNamespaces map[string]struct{}
	compiledRules       []CompiledResourceRule
}

// Engine registers dynamic informers per profile GVK and writes extracted attributes to Store.
//
// Scale notes (10k+ objects / 100+ clusters):
//   - targetsByGVR indexes targets per GVR so dispatch is O(targets-for-GVR) not O(all targets).
//   - dispatch workers drain a bounded queue so extract/upsert does not block informer delivery.
//   - Cluster-wide informers (metav1.NamespaceAll) cache every object for a GVR; namespace-scoped
//     watches are preferred when targets agree on one namespace via namespaceSelector.
type Engine struct {
	dynamic   dynamic.Interface
	kube      kubernetes.Interface
	access    *AccessChecker
	extractor *Extractor
	scrubber  *Scrubber
	scrubKeys []string
	store     *Store
	runCtx    context.Context

	mu                    sync.RWMutex
	informerMu            sync.Mutex
	factories             map[schema.GroupVersionResource]dynamicinformer.DynamicSharedInformerFactory
	started               map[schema.GroupVersionResource]bool
	informerScopes        map[schema.GroupVersionResource]string
	informerCancels       map[schema.GroupVersionResource]context.CancelFunc
	targets               map[string]targetState
	targetsByGVR          map[schema.GroupVersionResource][]string
	nsMeta                map[string]namespaceMeta
	nsMu                  sync.RWMutex
	forbidden             map[string]struct{}
	accessErr             map[string]struct{}
	extractErr            map[string]*extractFailureState
	defaults              NamespaceDefaults
	dispatchCh            chan dispatchJob
	dispatchWorkers       int
	dispatchQueueCap      int
	dispatchEnqueueWait   time.Duration
	resyncPeriod          time.Duration
	metricsSampleInterval time.Duration
	metricsLastRefresh    map[string]time.Time
	metricsMu             sync.Mutex
	dispatchOnce          sync.Once
}

// NewEngine constructs a collection engine.
func NewEngine(
	dynamicClient dynamic.Interface,
	kubeClient kubernetes.Interface,
	store *Store,
	cfg EngineConfig,
) (*Engine, error) {
	ext, err := NewExtractor()
	if err != nil {
		return nil, err
	}

	cfg = normalizeEngineConfig(cfg)

	return &Engine{
		dynamic:               dynamicClient,
		kube:                  kubeClient,
		access:                NewAccessChecker(kubeClient),
		extractor:             ext,
		scrubber:              NewScrubber(nil),
		store:                 store,
		factories:             make(map[schema.GroupVersionResource]dynamicinformer.DynamicSharedInformerFactory),
		started:               make(map[schema.GroupVersionResource]bool),
		informerScopes:        make(map[schema.GroupVersionResource]string),
		informerCancels:       make(map[schema.GroupVersionResource]context.CancelFunc),
		targets:               make(map[string]targetState),
		targetsByGVR:          make(map[schema.GroupVersionResource][]string),
		nsMeta:                make(map[string]namespaceMeta),
		forbidden:             make(map[string]struct{}),
		accessErr:             make(map[string]struct{}),
		extractErr:            make(map[string]*extractFailureState),
		dispatchCh:            make(chan dispatchJob, cfg.DispatchQueueSize),
		dispatchWorkers:       cfg.DispatchWorkers,
		dispatchQueueCap:      cfg.DispatchQueueSize,
		dispatchEnqueueWait:   cfg.DispatchEnqueueWait,
		resyncPeriod:          cfg.ResyncPeriod,
		metricsSampleInterval: cfg.MetricsSampleInterval,
		metricsLastRefresh:    make(map[string]time.Time),
	}, nil
}

// RegisterTargetOptions carries resolved namespace and rule state for collection filtering.
type RegisterTargetOptions struct {
	ScopeCeiling        ScopeCeiling
	EffectiveNamespaces []string
}

// SetNamespaceDefaults configures Helm-provided default include/exclude namespace lists.
func (e *Engine) SetNamespaceDefaults(defaults NamespaceDefaults) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.defaults = defaults
}

// SetScrubKeys configures operator scrubKeys[] extensions (built-in denylist always applies).
func (e *Engine) SetScrubKeys(keys []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.scrubKeys = append([]string(nil), keys...)
	e.scrubber = NewScrubber(keys)
}

// scrubberForProfile returns the operator scrubber, merging in profile prune.scrubKeys
// for full-resource export (ADR-0306 §Security). Built-in denylist always applies.
func (e *Engine) scrubberForProfile(profile kollectdevv1alpha1.KollectProfile) *Scrubber {
	e.mu.RLock()
	base := e.scrubber
	opKeys := e.scrubKeys
	e.mu.RUnlock()

	var extra []string
	if profile.Spec.Export != nil && profile.Spec.Export.Prune != nil {
		extra = profile.Spec.Export.Prune.ScrubKeys
	}

	if len(extra) == 0 {
		return base
	}

	merged := make([]string, 0, len(opKeys)+len(extra))
	merged = append(merged, opKeys...)
	merged = append(merged, extra...)

	return NewScrubber(merged)
}

// RegisterTarget records the target and ensures a dynamic informer exists for its profile GVK.
func (e *Engine) RegisterTarget(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
	profile *kollectdevv1alpha1.KollectProfile,
	opts RegisterTargetOptions,
) error {
	key := targetKey(target.Namespace, target.Name)

	if target.Spec.Suspend {
		e.UnregisterTarget(target.Namespace, target.Name)

		return nil
	}

	if err := e.refreshNamespaceCache(ctx); err != nil {
		log.FromContext(ctx).Error(err, "refresh namespace cache")
	}

	gvr := gvrFromProfile(profile.Spec.TargetGVK)

	compiled, err := CompileResourceRules(target.Spec.ResourceRules, e.extractor.celEnv)
	if err != nil {
		return fmt.Errorf("compile resourceRules: %w", err)
	}

	effective := opts.EffectiveNamespaces
	if len(effective) == 0 {
		e.nsMu.RLock()
		matched := MatchIntentNamespaces(
			target.Spec.CollectionFilterSpec,
			target.Spec.NamespaceSelector,
			namespaceMetaMapToFilter(e.nsMeta),
			e.defaults,
		)
		e.nsMu.RUnlock()
		effective = EffectiveNamespaces(matched, opts.ScopeCeiling, target.Spec.CollectionFilterSpec, e.defaults)
	}

	e.mu.Lock()
	if old, ok := e.targets[key]; ok {
		oldGVR := gvrFromProfile(old.profile.Spec.TargetGVK)
		e.unindexTargetLocked(key, oldGVR)
	}

	e.targets[key] = targetState{
		target:              *target.DeepCopy(),
		profile:             *profile.DeepCopy(),
		effectiveNamespaces: EffectiveNamespaceSet(effective),
		compiledRules:       compiled,
	}
	e.indexTargetLocked(key, gvr)
	e.mu.Unlock()

	if err := e.startInformer(e.informerContext(), gvr); err != nil {
		return err
	}

	return nil
}

// UnregisterTarget stops tracking a target and removes its items from the store.
func (e *Engine) UnregisterTarget(namespace, name string) {
	key := targetKey(namespace, name)

	e.mu.Lock()
	if st, ok := e.targets[key]; ok {
		gvr := gvrFromProfile(st.profile.Spec.TargetGVK)
		e.unindexTargetLocked(key, gvr)
	}

	delete(e.targets, key)
	delete(e.forbidden, key)
	delete(e.accessErr, key)
	delete(e.extractErr, key)
	e.mu.Unlock()

	e.metricsMu.Lock()
	delete(e.metricsLastRefresh, key)
	e.metricsMu.Unlock()

	e.store.RemoveTarget(namespace, name)
}

// ItemCount returns collected items for a target.
func (e *Engine) ItemCount(namespace, name string) int {
	return e.store.CountForTarget(namespace, name)
}

// BindClusterTargetNamespaces records workload namespaces for a cluster-scoped target name.
func (e *Engine) BindClusterTargetNamespaces(targetName string, namespaces []string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, ns := range namespaces {
		key := targetKey(ns, targetName)
		e.targets[key] = targetState{
			target: kollectdevv1alpha1.KollectTarget{
				ObjectMeta: metav1.ObjectMeta{Name: targetName, Namespace: ns},
			},
		}
	}
}

// NamespacesForClusterTarget returns workload namespaces where a cluster target name is registered.
func (e *Engine) NamespacesForClusterTarget(targetName string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var namespaces []string
	for key, st := range e.targets {
		if st.target.Name != targetName {
			continue
		}

		ns, _, ok := strings.Cut(key, "/")
		if !ok || ns == "" {
			continue
		}

		namespaces = append(namespaces, ns)
	}

	slices.Sort(namespaces)

	return namespaces
}

// HasForbiddenScope reports whether collection was denied for the target namespace/GVK pair.
func (e *Engine) HasForbiddenScope(targetNamespace, targetName string) bool {
	key := targetKey(targetNamespace, targetName)

	e.mu.RLock()
	defer e.mu.RUnlock()

	_, ok := e.forbidden[key]

	return ok
}

// HasAccessCheckFailure reports whether SAR API errors blocked collection for the target.
func (e *Engine) HasAccessCheckFailure(targetNamespace, targetName string) bool {
	key := targetKey(targetNamespace, targetName)

	e.mu.RLock()
	defer e.mu.RUnlock()

	_, ok := e.accessErr[key]

	return ok
}

// extractFailureState tracks resources currently failing attribute extraction for a target
// (ADR-0020 ErrTerminal class — invalid CEL/JSONPath or per-resource evaluation error).
type extractFailureState struct {
	resources map[string]struct{} // resource UID -> currently failing
	lastErr   string
}

// ExtractFailures reports how many resources are currently failing attribute extraction for
// the target, and the most recently observed extraction error message (GUIDELINES.md §1, §2:
// a count + last message only, never per-resource payload).
func (e *Engine) ExtractFailures(targetNamespace, targetName string) (count int, lastErr string) {
	key := targetKey(targetNamespace, targetName)

	e.mu.RLock()
	defer e.mu.RUnlock()

	st, ok := e.extractErr[key]
	if !ok {
		return 0, ""
	}

	return len(st.resources), st.lastErr
}

// recordExtractFailure marks resourceUID as currently failing extraction for targetKeyStr.
func (e *Engine) recordExtractFailure(targetKeyStr, resourceUID, errMsg string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	st, ok := e.extractErr[targetKeyStr]
	if !ok {
		st = &extractFailureState{resources: make(map[string]struct{})}
		e.extractErr[targetKeyStr] = st
	}

	st.resources[resourceUID] = struct{}{}
	st.lastErr = errMsg
}

// clearExtractFailure clears resourceUID's extraction-failure state for targetKeyStr, if any.
func (e *Engine) clearExtractFailure(targetKeyStr, resourceUID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	st, ok := e.extractErr[targetKeyStr]
	if !ok {
		return
	}

	delete(st.resources, resourceUID)
	if len(st.resources) == 0 {
		delete(e.extractErr, targetKeyStr)
	}
}

// Start stores the manager context used for informer factories and starts dispatch workers.
func (e *Engine) Start(ctx context.Context) error {
	e.runCtx = ctx
	e.startDispatchWorkers()

	return nil
}

func (e *Engine) startDispatchWorkers() {
	e.dispatchOnce.Do(func() {
		workers := e.dispatchWorkers
		if workers <= 0 {
			workers = defaultDispatchWorkers
		}
		for i := 0; i < workers; i++ {
			go e.dispatchWorker()
		}
	})
}

func (e *Engine) dispatchWorker() {
	for job := range e.dispatchCh {
		e.processDispatch(job.ctx, job.gvr, job.obj, job.deleted)
	}
}

func (e *Engine) indexTargetLocked(key string, gvr schema.GroupVersionResource) {
	for _, existing := range e.targetsByGVR[gvr] {
		if existing == key {
			return
		}
	}

	e.targetsByGVR[gvr] = append(e.targetsByGVR[gvr], key)
}

func (e *Engine) unindexTargetLocked(key string, gvr schema.GroupVersionResource) {
	keys := e.targetsByGVR[gvr]
	for i, existing := range keys {
		if existing == key {
			e.targetsByGVR[gvr] = append(keys[:i], keys[i+1:]...)

			return
		}
	}
}

func (e *Engine) informerContext() context.Context {
	if e.runCtx != nil {
		return e.runCtx
	}

	return context.Background()
}

func (e *Engine) refreshNamespaceCache(ctx context.Context) error {
	if e.kube == nil {
		return nil
	}

	list, err := e.kube.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list namespaces: %w", err)
	}

	metaByNS := make(map[string]namespaceMeta, len(list.Items))
	for i := range list.Items {
		ns := &list.Items[i]
		metaByNS[ns.Name] = namespaceMeta{
			Labels:      labels.Set(ns.Labels),
			Annotations: ns.Annotations,
		}
	}

	e.nsMu.Lock()
	e.nsMeta = metaByNS
	e.nsMu.Unlock()

	return nil
}

func (e *Engine) startInformer(ctx context.Context, gvr schema.GroupVersionResource) error {
	// Registration can race across reconcilers. Serialize informer lifecycle transitions so
	// exactly one replacement is built for a GVR and the previous factory is cancelled only
	// after its replacement has completed the initial List and cache sync.
	e.informerMu.Lock()
	defer e.informerMu.Unlock()

	desiredScope := e.watchNamespaceForGVR(gvr)
	e.mu.RLock()
	currentScope := e.informerScopes[gvr]
	informerStarted := e.started[gvr]
	e.mu.RUnlock()
	if informerStarted && (currentScope == metav1.NamespaceAll || currentScope == desiredScope) {
		return nil
	}

	// A running namespace-scoped informer may only widen. If active targets disagree about
	// their single namespace (or any target spans namespaces), watchNamespaceForGVR returns
	// NamespaceAll. Narrowing an all-namespace informer is deliberately deferred to avoid
	// churn and gaps when targets are removed.
	watchNS := desiredScope

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		e.dynamic,
		e.resyncPeriod,
		watchNS,
		nil,
	)
	gvrLabels := []string{gvr.Group, gvr.Version, gvr.Resource}
	if watchNS == metav1.NamespaceAll {
		log.FromContext(ctx).Info(
			"cluster-wide informer scope for GVR; prefer namespace-scoped targets at scale",
			"group", gvr.Group, "version", gvr.Version, "resource", gvr.Resource,
		)
		metrics.InformerClusterWideScope.WithLabelValues(gvrLabels...).Set(1)
	} else {
		metrics.InformerClusterWideScope.WithLabelValues(gvrLabels...).Set(0)
	}

	informer := factory.ForResource(gvr).Informer()
	runCtx := e.informerContext()
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			e.dispatch(runCtx, gvr, obj, false)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if isInformerResync(oldObj, newObj) {
				metrics.InformerResyncDispatchesTotal.WithLabelValues(gvrLabels...).Inc()
			}
			e.dispatch(runCtx, gvr, newObj, false)
		},
		DeleteFunc: func(obj interface{}) {
			e.dispatch(runCtx, gvr, obj, true)
		},
	})
	if err != nil {
		return fmt.Errorf("add informer handler: %w", err)
	}

	informerCtx, cancel := context.WithCancel(ctx)
	factory.Start(informerCtx.Done())
	syncs := factory.WaitForCacheSync(informerCtx.Done())
	if synced, ok := syncs[gvr]; !ok || !synced {
		cancel()
		return fmt.Errorf("sync informer cache for %s", gvr.String())
	}

	e.mu.Lock()
	oldCancel := e.informerCancels[gvr]
	e.factories[gvr] = factory
	e.started[gvr] = true
	e.informerScopes[gvr] = watchNS
	e.informerCancels[gvr] = cancel
	e.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}
	e.updateInformerMetrics(gvr, informer)

	return nil
}

func (e *Engine) informerScope(gvr schema.GroupVersionResource) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.started[gvr] {
		return ""
	}

	return e.informerScopes[gvr]
}

func (e *Engine) updateInformerMetrics(gvr schema.GroupVersionResource, informer cache.SharedIndexInformer) {
	if informer == nil {
		return
	}

	count := len(informer.GetStore().ListKeys())
	metrics.InformerObjects.WithLabelValues(gvr.Group, gvr.Version, gvr.Resource).Set(float64(count))
}

func (e *Engine) dispatch(ctx context.Context, gvr schema.GroupVersionResource, obj interface{}, deleted bool) {
	e.startDispatchWorkers()

	job := dispatchJob{ctx: ctx, gvr: gvr, obj: obj, deleted: deleted}
	metrics.CollectDispatchQueueDepth.Set(float64(len(e.dispatchCh)))
	select {
	case e.dispatchCh <- job:
		return
	default:
	}

	wait := e.dispatchEnqueueWait
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case e.dispatchCh <- job:
			return
		case <-timer.C:
		}
	}

	// Backpressure: block on the queue rather than running the job inline on
	// this goroutine (the informer's event-handler thread). Inline execution
	// bypasses the dispatch worker pool's concurrency cap entirely, including
	// API-server calls (access checks), which removes the very backpressure
	// the queue+workers are meant to provide (EC-P0-02). Blocking still
	// respects ctx cancellation so shutdown doesn't leak this goroutine.
	metrics.CollectDispatchBackpressureTotal.Inc()
	select {
	case e.dispatchCh <- job:
	case <-ctx.Done():
	}
}

func (e *Engine) processDispatch(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	obj interface{},
	deleted bool,
) {
	start := time.Now()
	defer func() {
		metrics.CollectDispatchDurationSeconds.Observe(time.Since(start).Seconds())
		metrics.CollectDispatchQueueDepth.Set(float64(len(e.dispatchCh)))
	}()

	u := toUnstructured(obj)
	if u == nil {
		return
	}

	resourceNS := u.GetNamespace()
	if resourceNS == "" {
		resourceNS = corev1.NamespaceDefault
	}

	e.mu.RLock()
	keys := e.targetsByGVR[gvr]
	states := make([]targetState, 0, len(keys))
	for _, key := range keys {
		if st, ok := e.targets[key]; ok {
			states = append(states, st)
		}
	}
	e.mu.RUnlock()

	for _, st := range states {
		target := st.target
		targetKeyStr := targetKey(target.Namespace, target.Name)

		if deleted {
			e.store.Remove(target.Namespace, target.Name, string(u.GetUID()))
			metrics.CollectItemsTotal.Set(float64(e.store.Len()))
			e.refreshTargetSnapshotMetrics(st, target)
			e.clearExtractFailure(targetKeyStr, string(u.GetUID()))

			continue
		}

		if !e.matchesTarget(ctx, st, gvr, u) {
			e.store.Remove(target.Namespace, target.Name, string(u.GetUID()))
			e.clearExtractFailure(targetKeyStr, string(u.GetUID()))
			continue
		}

		allowed, err := e.access.CanAccess(ctx, gvr, resourceNS, "list")
		if err != nil {
			log.FromContext(ctx).Error(err, "access check failed",
				"target", target.Namespace+"/"+target.Name,
				"namespace", resourceNS)
			e.mu.Lock()
			e.accessErr[targetKeyStr] = struct{}{}
			e.mu.Unlock()
			metrics.ReconcileErrorsTotal.WithLabelValues("KollectTarget", metrics.ErrorClassTransient).Inc()

			continue
		}

		e.mu.Lock()
		delete(e.accessErr, targetKeyStr)
		e.mu.Unlock()

		if !allowed {
			e.mu.Lock()
			e.forbidden[targetKeyStr] = struct{}{}
			e.mu.Unlock()
			metrics.ReconcileErrorsTotal.WithLabelValues("KollectTarget", metrics.ErrorClassForbidden).Inc()

			continue
		}

		e.mu.Lock()
		delete(e.forbidden, targetKeyStr)
		e.mu.Unlock()

		resourceUID := string(u.GetUID())

		attrs, err := e.extractor.Extract(u, st.profile.Spec.Attributes)
		if err != nil {
			log.FromContext(ctx).Error(err, "extract attributes",
				"target", target.Namespace+"/"+target.Name,
				"resource", u.GetNamespace()+"/"+u.GetName())
			e.recordExtractFailure(targetKeyStr, resourceUID, err.Error())
			metrics.ReconcileErrorsTotal.WithLabelValues("KollectTarget", metrics.ErrorClassTerminal).Inc()

			continue
		}

		e.clearExtractFailure(targetKeyStr, resourceUID)

		scrubber := e.scrubberForProfile(st.profile)
		attrs = scrubber.ScrubAttributes(attrs)

		if st.profile.Spec.Export.ResourceExportEnabled() {
			if attrs == nil {
				attrs = make(map[string]any, 1)
			}

			attrs[st.profile.Spec.Export.AttributeKey()] = PruneResource(u, st.profile.Spec.Export, scrubber)
		}

		gvkLabel := fmt.Sprintf("%s/%s/%s", st.profile.Spec.TargetGVK.Group,
			st.profile.Spec.TargetGVK.Version, st.profile.Spec.TargetGVK.Kind)

		e.store.Upsert(Item{
			TargetNamespace: target.Namespace,
			TargetName:      target.Name,
			Namespace:       u.GetNamespace(),
			Name:            u.GetName(),
			Group:           st.profile.Spec.TargetGVK.Group,
			Version:         st.profile.Spec.TargetGVK.Version,
			Kind:            st.profile.Spec.TargetGVK.Kind,
			UID:             string(u.GetUID()),
			Attributes:      attrs,
		})
		metrics.CollectedObjects.WithLabelValues(target.Spec.ProfileRef, gvkLabel).
			Set(float64(e.store.CountForTarget(target.Namespace, target.Name)))
		metrics.CollectItemsTotal.Set(float64(e.store.Len()))
		e.refreshTargetSnapshotMetrics(st, target)
	}
}

func (e *Engine) refreshTargetSnapshotMetrics(st targetState, target kollectdevv1alpha1.KollectTarget) {
	key := targetKey(target.Namespace, target.Name)
	interval := e.metricsSampleInterval
	if interval <= 0 {
		interval = defaultMetricsSampleInterval
	}

	now := time.Now()
	e.metricsMu.Lock()
	if e.metricsLastRefresh == nil {
		e.metricsLastRefresh = make(map[string]time.Time)
	}
	if last, ok := e.metricsLastRefresh[key]; ok && now.Sub(last) < interval {
		e.metricsMu.Unlock()

		return
	}
	e.metricsLastRefresh[key] = now
	e.metricsMu.Unlock()

	gvkLabel := fmt.Sprintf("%s/%s/%s", st.profile.Spec.TargetGVK.Group,
		st.profile.Spec.TargetGVK.Version, st.profile.Spec.TargetGVK.Kind)
	items := e.store.SnapshotTarget(target.Namespace, target.Name)
	metricPaths := MetricPathsFromProfile(st.profile.Spec)
	recordTargetSnapshotMetrics(target.Spec.ProfileRef, gvkLabel, items, metricPaths)
}

func isInformerResync(oldObj, newObj interface{}) bool {
	oldU := toUnstructured(oldObj)
	newU := toUnstructured(newObj)
	if oldU == nil || newU == nil {
		return false
	}

	return oldU.GetResourceVersion() == newU.GetResourceVersion()
}

func (e *Engine) matchesTarget(
	ctx context.Context,
	st targetState,
	gvr schema.GroupVersionResource,
	u *unstructured.Unstructured,
) bool {
	target := st.target
	resourceNS := u.GetNamespace()
	if resourceNS == "" {
		resourceNS = corev1.NamespaceDefault
	}

	if !e.namespaceMatches(&target, st.effectiveNamespaces, resourceNS) {
		return false
	}

	e.nsMu.RLock()
	nsMetaCopy := e.nsMeta
	e.nsMu.RUnlock()

	if !ResourceMatchesRules(u, gvr, &target, &st.profile, st.compiledRules, namespaceMetaMapToFilter(nsMetaCopy)) {
		return false
	}

	if !ShouldCollect(labels.Set(u.GetLabels()), e.namespaceMetaFor(resourceNS), &target) {
		return false
	}

	_ = ctx

	return true
}

func (e *Engine) namespaceMetaFor(name string) namespaceMeta {
	e.nsMu.RLock()
	defer e.nsMu.RUnlock()

	meta, ok := e.nsMeta[name]
	if !ok {
		return namespaceMeta{}
	}

	return meta
}

func (e *Engine) namespaceMatches(
	target *kollectdevv1alpha1.KollectTarget,
	effective map[string]struct{},
	resourceNamespace string,
) bool {
	if len(effective) > 0 {
		_, ok := effective[resourceNamespace]
		return ok
	}

	// Cluster-scoped targets register one synthetic KollectTarget per workload namespace
	// using a metadata.name pin; skip tenant/label selectors for that path.
	if target.Spec.NamespaceSelector != nil {
		if name, ok := target.Spec.NamespaceSelector.MatchLabels[corev1.LabelMetadataName]; ok {
			return resourceNamespace == name
		}
	}

	e.nsMu.RLock()
	meta, ok := e.nsMeta[resourceNamespace]
	e.nsMu.RUnlock()

	if !ok {
		return false
	}

	return namespaceMatchesSelector(target.Spec.NamespaceSelector, meta.Labels)
}

func toUnstructured(obj interface{}) *unstructured.Unstructured {
	u, ok := obj.(*unstructured.Unstructured)
	if ok {
		return u
	}

	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if !ok {
		return nil
	}

	u, ok = tombstone.Obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	return u
}

func gvrFromProfile(gvk kollectdevv1alpha1.GroupVersionKind) schema.GroupVersionResource {
	plural, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	})

	return plural
}

// NamespaceMetaSnapshot returns a copy of cached namespace metadata for filter resolution.
func (e *Engine) NamespaceMetaSnapshot() map[string]NamespaceMeta {
	e.nsMu.RLock()
	defer e.nsMu.RUnlock()

	out := make(map[string]NamespaceMeta, len(e.nsMeta))
	for k, v := range e.nsMeta {
		out[k] = NamespaceMeta(v)
	}

	return out
}

// NamespaceDefaultsSnapshot returns configured Helm namespace defaults.
func (e *Engine) NamespaceDefaultsSnapshot() NamespaceDefaults {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.defaults
}
