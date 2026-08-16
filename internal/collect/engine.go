// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

package collect

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	// fingerprint summarises everything that decides which objects this target
	// collects and how they are extracted. RegisterTarget backfills the store from
	// the informer cache only when it changes (see targetStateFingerprint).
	fingerprint string
}

// targetStateFingerprint hashes the collection-relevant state of a target: its
// effective namespace set, its resource rules, and the target/profile generations
// (which cover every other spec change). Identical state across two registrations
// means the target cannot have missed anything the running informer delivered, so
// no store backfill is needed.
func targetStateFingerprint(
	target *kollectdevv1alpha1.KollectTarget,
	profile *kollectdevv1alpha1.KollectProfile,
	effective []string,
) string {
	h := sha256.New()
	for _, ns := range sortedUniqueStrings(effective) {
		_, _ = h.Write([]byte(ns))
		_, _ = h.Write([]byte{0})
	}

	_, _ = fmt.Fprintf(h, "|targetGen=%d|profileGen=%d|", target.Generation, profile.Generation)

	rules, err := json.Marshal(target.Spec.ResourceRules)
	if err != nil {
		rules = []byte(err.Error())
	}
	_, _ = h.Write(rules)

	spec, err := json.Marshal(profile.Spec)
	if err != nil {
		spec = []byte(err.Error())
	}
	_, _ = h.Write(spec)

	return hex.EncodeToString(h.Sum(nil))
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
	retireInformer        func(dynamicinformer.DynamicSharedInformerFactory, context.CancelFunc)
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
	// backfillDispatches counts store backfills triggered against an already-running
	// informer. Steady-state re-registration must never move it (see the fingerprint
	// gate in RegisterTarget).
	backfillDispatches atomic.Int64
	stopCtx            context.Context
	stopFn             context.CancelFunc
	workersWG          sync.WaitGroup
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

	// stopCtx is created at construction (not in Start) so dispatch workers have a
	// guaranteed lifecycle signal even when dispatch() starts them before Start()
	// runs. Start() wires the manager context into stopFn so ctx cancellation
	// terminates the worker pool (REL-04).
	stopCtx, stopFn := context.WithCancel(context.Background()) //nolint:gosec // G118: stopFn (cancel) is invoked by Start's ctx-cancel watcher goroutine (REL-04)

	return &Engine{
		stopCtx:               stopCtx,
		stopFn:                stopFn,
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
		retireInformer:        retireDynamicInformer,
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
	ScopeCeiling ScopeCeiling
	// EffectiveNamespaces is the namespace set the caller already resolved. Supplying it
	// carries a contract: the caller is responsible for the freshness of the engine's
	// namespace metadata cache (call RefreshNamespaces first), because RegisterTarget
	// then skips the cluster-wide namespace LIST. Leave it empty to have the engine
	// refresh and recompute the set itself.
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

	gvr := gvrFromProfile(profile.Spec.TargetGVK)

	compiled, err := CompileResourceRules(target.Spec.ResourceRules, e.extractor.celEnv)
	if err != nil {
		return fmt.Errorf("compile resourceRules: %w", err)
	}

	// namespaceSource records who decided the namespace set. "opts" means the
	// reconciler resolved it from its own namespace snapshot; "recomputed" means the
	// engine derived it from its (just refreshed) cache. The two can disagree, and
	// when they do every object outside the chosen set is dropped, so the source
	// belongs in the log next to the set itself.
	namespaceSource := "opts"

	effective := opts.EffectiveNamespaces
	if len(effective) == 0 {
		namespaceSource = "recomputed"

		// Only the recompute branch reads the namespace cache here, so only it has to
		// pay for a live cluster-wide namespace LIST. Callers that supply
		// EffectiveNamespaces have already resolved the set from a snapshot they
		// refreshed themselves (RefreshNamespaces), and reconcilers re-register on every
		// pass — refreshing unconditionally made every resync a LIST for every target
		// (PERF-FIX-05 review finding F2).
		if err := e.refreshNamespaceCache(ctx); err != nil {
			log.FromContext(ctx).Error(err, "refresh namespace cache")
		}

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

	fingerprint := targetStateFingerprint(target, profile, effective)

	e.mu.Lock()
	old, registered := e.targets[key]
	if registered {
		oldGVR := gvrFromProfile(old.profile.Spec.TargetGVK)
		e.unindexTargetLocked(key, oldGVR)
	}

	e.targets[key] = targetState{
		target:              *target.DeepCopy(),
		profile:             *profile.DeepCopy(),
		effectiveNamespaces: EffectiveNamespaceSet(effective),
		compiledRules:       compiled,
		fingerprint:         fingerprint,
	}
	e.indexTargetLocked(key, gvr)
	e.mu.Unlock()

	// A target that is new, or whose collection state changed, may have missed
	// objects the running informer already delivered — it must be backfilled from
	// the informer cache. A re-registration with identical state cannot have, and
	// must stay free: reconcilers re-register on every pass and a backfill is
	// O(objects for the GVR) x O(targets for the GVR) through the dispatch queue.
	needsBackfill := !registered || old.fingerprint != fingerprint

	log.FromContext(ctx).V(1).Info("registered collection target",
		"target", key,
		"gvr", gvr.String(),
		"effectiveNamespaces", effective,
		"namespaceSource", namespaceSource,
		"backfill", needsBackfill,
	)

	return e.startInformer(e.informerContext(), gvr, needsBackfill)
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
//
// It writes a bare targetState, so any collection-state fingerprint for that key is
// cleared and the next RegisterTarget backfills the store once. That is the safe
// direction (re-dispatch rather than silent under-collection), but it means this must
// not be called per-reconcile alongside RegisterTarget.
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
//
// Start is non-blocking (it is a manager Runnable): it wires the manager context into the
// engine's stop function via a goroutine so that cancelling ctx cancels stopCtx, which in
// turn drains and terminates every dispatch worker (REL-04). The wiring goroutine itself
// exits as soon as ctx is done, so it does not leak.
func (e *Engine) Start(ctx context.Context) error {
	e.runCtx = ctx
	e.startDispatchWorkers()

	go func() {
		<-ctx.Done()
		e.stopFn()
	}()

	return nil
}

func (e *Engine) startDispatchWorkers() {
	e.dispatchOnce.Do(func() {
		workers := e.dispatchWorkers
		if workers <= 0 {
			workers = defaultDispatchWorkers
		}
		e.workersWG.Add(workers)
		for i := 0; i < workers; i++ {
			go e.dispatchWorker()
		}
	})
}

// dispatchWorker drains the dispatch queue until the engine's stop context is
// cancelled (REL-04). It never closes dispatchCh — a producer may be mid-send —
// so shutdown is signalled purely by stopCtx. The stop check is prioritised
// before the receive so a cancelled engine stops promptly under continuous load;
// a job already handed to processDispatch runs to completion (processDispatch is
// not preemptible) before the worker re-checks and returns.
func (e *Engine) dispatchWorker() {
	defer e.workersWG.Done()
	for {
		select {
		case <-e.stopCtx.Done():
			return
		default:
		}

		select {
		case <-e.stopCtx.Done():
			return
		case job, ok := <-e.dispatchCh:
			if !ok {
				return
			}
			e.processDispatch(job.ctx, job.gvr, job.obj, job.deleted)
		}
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

// RefreshNamespaces reloads the engine's namespace metadata cache from the API server.
//
// Callers that read NamespaceMetaSnapshot before RegisterTarget must call this first:
// RegisterTarget is the only other writer of that cache, so a read taken before it runs
// is one registration behind and can omit a namespace created since the last register.
// An effective namespace set computed from such a snapshot silently rejects every object
// in the missing namespace (see namespaceMatches).
func (e *Engine) RefreshNamespaces(ctx context.Context) error {
	return e.refreshNamespaceCache(ctx)
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

// startInformer ensures an informer for gvr runs at a scope covering every registered
// target. backfill re-dispatches the running informer's cache when the caller's target
// state changed; it is ignored on the replace path, which rehydrates the store anyway.
func (e *Engine) startInformer(ctx context.Context, gvr schema.GroupVersionResource, backfill bool) error {
	// Registration can race across reconcilers. Serialize informer lifecycle transitions so
	// exactly one replacement is built for a GVR and the previous factory is cancelled only
	// after its replacement has completed the initial List and cache sync.
	e.informerMu.Lock()
	defer e.informerMu.Unlock()

	desiredScope := e.watchNamespaceForGVR(gvr)
	e.mu.RLock()
	currentScope := e.informerScopes[gvr]
	informerStarted := e.started[gvr]
	runningFactory := e.factories[gvr]
	e.mu.RUnlock()
	if informerStarted && (currentScope == metav1.NamespaceAll || currentScope == desiredScope) {
		// The informer already covers the desired scope, so it replays nothing: a target
		// registered (or corrected) now never sees the objects already sitting in its
		// cache until one of them mutates or the resync period (12h by default) elapses.
		// Re-dispatch the cache so the new state takes effect immediately.
		//
		// This holds informerMu across the dispatch exactly as the replace path below
		// does; the fingerprint gate in RegisterTarget is what keeps it off the
		// steady-state re-registration path.
		if backfill && runningFactory != nil {
			e.backfillDispatches.Add(1)
			e.resyncInformerStore(e.informerContext(), gvr, runningFactory.ForResource(gvr).Informer())
		}

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
		retireDynamicInformer(factory, cancel)
		return fmt.Errorf("sync informer cache for %s", gvr.String())
	}

	e.mu.Lock()
	oldCancel := e.informerCancels[gvr]
	oldFactory := e.factories[gvr]
	e.factories[gvr] = factory
	e.started[gvr] = true
	e.informerScopes[gvr] = watchNS
	e.informerCancels[gvr] = cancel
	e.mu.Unlock()

	if oldCancel != nil && oldFactory != nil {
		retire := e.retireInformer
		if retire == nil {
			retire = retireDynamicInformer
		}
		retire(oldFactory, oldCancel)
		// Rehydrate after retire: a narrow→wide replace can drop Store rows when
		// old Delete delivery races the replacement's initial Adds (multitenant
		// Kind: one tenant Ready with collecting 0 until a later Update).
		e.resyncInformerStore(runCtx, gvr, informer)
	}
	e.updateInformerMetrics(gvr, informer)

	return nil
}

// resyncInformerStore dispatches every object currently in the informer cache.
// Idempotent with Upsert; used after informer replace to close the retire race.
func (e *Engine) resyncInformerStore(ctx context.Context, gvr schema.GroupVersionResource, informer cache.SharedIndexInformer) {
	if informer == nil {
		return
	}
	for _, obj := range informer.GetStore().List() {
		e.dispatch(ctx, gvr, obj, false)
	}
}

func retireDynamicInformer(factory dynamicinformer.DynamicSharedInformerFactory, cancel context.CancelFunc) {
	cancel()
	factory.Shutdown()
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

	// Namespace mismatches are tallied per dispatched object and reported once, so
	// the label lookup stays off the per-target inner loop.
	namespaceMismatches := 0
	defer func() {
		if namespaceMismatches > 0 {
			metrics.CollectNamespaceMismatchTotal.
				WithLabelValues(gvr.Group, gvr.Version, gvr.Resource).
				Add(float64(namespaceMismatches))
		}
	}()

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

		if match := e.matchesTarget(ctx, st, gvr, u); match != targetMatchAccepted {
			if match == targetMatchNamespaceMismatch {
				namespaceMismatches++
			}

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
			// REL-01: fail closed — drop any prior last-good row for this UID
			// before recording degradation so exports never present stale values.
			e.store.Remove(target.Namespace, target.Name, resourceUID)
			metrics.CollectItemsTotal.Set(float64(e.store.Len()))
			e.refreshTargetSnapshotMetrics(st, target)
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

// targetMatch classifies why an object was accepted or dropped for a target.
// Namespace mismatches are called out separately because they are the one rejection
// that can be caused by stale operator state rather than by user intent, and they
// used to be entirely invisible (COLLECT-NS-BACKFILL).
type targetMatch int

const (
	targetMatchAccepted targetMatch = iota
	targetMatchNamespaceMismatch
	targetMatchFiltered
)

func (e *Engine) matchesTarget(
	ctx context.Context,
	st targetState,
	gvr schema.GroupVersionResource,
	u *unstructured.Unstructured,
) targetMatch {
	target := st.target
	resourceNS := u.GetNamespace()
	if resourceNS == "" {
		resourceNS = corev1.NamespaceDefault
	}

	if !e.namespaceMatches(&target, st.effectiveNamespaces, resourceNS) {
		return targetMatchNamespaceMismatch
	}

	e.nsMu.RLock()
	nsMetaCopy := e.nsMeta
	e.nsMu.RUnlock()

	if !ResourceMatchesRules(u, gvr, &target, &st.profile, st.compiledRules, namespaceMetaMapToFilter(nsMetaCopy)) {
		return targetMatchFiltered
	}

	if !ShouldCollect(labels.Set(u.GetLabels()), e.namespaceMetaFor(resourceNS), &target) {
		return targetMatchFiltered
	}

	_ = ctx

	return targetMatchAccepted
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
