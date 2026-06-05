// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

package collect

import (
	"context"
	"fmt"
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

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
)

const informerResync = 12 * time.Hour

type targetState struct {
	target              kollectdevv1alpha1.KollectTarget
	profile             kollectdevv1alpha1.KollectProfile
	effectiveNamespaces map[string]struct{}
	compiledRules       []CompiledResourceRule
}

// Engine registers dynamic informers per profile GVK and writes extracted attributes to Store.
//
// Scale notes (10k+ objects / 100+ clusters):
//   - dispatch() scans all targets for a GVR on every informer event — O(targets) per event;
//     split profiles/GVKs when target count grows.
//   - Cluster-wide informers (metav1.NamespaceAll) cache every object for a GVR; namespace-scoped
//     watches are preferred when targets agree on one namespace via namespaceSelector.
//   - extract + store.Upsert run on the informer thread; slow CEL paths block cache delivery.
type Engine struct {
	dynamic   dynamic.Interface
	kube      kubernetes.Interface
	access    *AccessChecker
	extractor *Extractor
	store     *Store
	runCtx    context.Context

	mu        sync.RWMutex
	factories map[schema.GroupVersionResource]dynamicinformer.DynamicSharedInformerFactory
	started   map[schema.GroupVersionResource]bool
	targets   map[string]targetState
	nsMeta    map[string]namespaceMeta
	nsMu      sync.RWMutex
	forbidden map[string]struct{}
	accessErr map[string]struct{}
	defaults  NamespaceDefaults
}

// NewEngine constructs a collection engine.
func NewEngine(dynamicClient dynamic.Interface, kubeClient kubernetes.Interface, store *Store) (*Engine, error) {
	ext, err := NewExtractor()
	if err != nil {
		return nil, err
	}

	return &Engine{
		dynamic:   dynamicClient,
		kube:      kubeClient,
		access:    NewAccessChecker(kubeClient),
		extractor: ext,
		store:     store,
		factories: make(map[schema.GroupVersionResource]dynamicinformer.DynamicSharedInformerFactory),
		started:   make(map[schema.GroupVersionResource]bool),
		targets:   make(map[string]targetState),
		nsMeta:    make(map[string]namespaceMeta),
		forbidden: make(map[string]struct{}),
		accessErr: make(map[string]struct{}),
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
	e.targets[key] = targetState{
		target:              *target.DeepCopy(),
		profile:             *profile.DeepCopy(),
		effectiveNamespaces: EffectiveNamespaceSet(effective),
		compiledRules:       compiled,
	}
	needStart := !e.started[gvr]
	e.mu.Unlock()

	if needStart {
		if err := e.startInformer(e.informerContext(), gvr); err != nil {
			return err
		}
	}

	return nil
}

// UnregisterTarget stops tracking a target and removes its items from the store.
func (e *Engine) UnregisterTarget(namespace, name string) {
	key := targetKey(namespace, name)

	e.mu.Lock()
	delete(e.targets, key)
	delete(e.forbidden, key)
	delete(e.accessErr, key)
	e.mu.Unlock()

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

// Start stores the manager context used for informer factories.
func (e *Engine) Start(ctx context.Context) error {
	e.runCtx = ctx

	return nil
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
	e.mu.Lock()
	if e.started[gvr] {
		e.mu.Unlock()

		return nil
	}
	e.mu.Unlock()

	watchNS := e.watchNamespaceForGVR(gvr)

	e.mu.Lock()
	if e.started[gvr] {
		e.mu.Unlock()

		return nil
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		e.dynamic,
		informerResync,
		watchNS,
		nil,
	)
	e.factories[gvr] = factory
	e.started[gvr] = true
	e.mu.Unlock()

	informer := factory.ForResource(gvr).Informer()
	runCtx := e.informerContext()
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			e.dispatch(runCtx, gvr, obj, false)
		},
		UpdateFunc: func(_, newObj interface{}) {
			e.dispatch(runCtx, gvr, newObj, false)
		},
		DeleteFunc: func(obj interface{}) {
			e.dispatch(runCtx, gvr, obj, true)
		},
	})
	if err != nil {
		return fmt.Errorf("add informer handler: %w", err)
	}

	go factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	e.updateInformerMetrics(gvr, informer)

	return nil
}

func (e *Engine) updateInformerMetrics(gvr schema.GroupVersionResource, informer cache.SharedIndexInformer) {
	if informer == nil {
		return
	}

	count := len(informer.GetStore().ListKeys())
	metrics.InformerObjects.WithLabelValues(gvr.Group, gvr.Version, gvr.Resource).Set(float64(count))
}

func (e *Engine) dispatch(ctx context.Context, gvr schema.GroupVersionResource, obj interface{}, deleted bool) {
	u := toUnstructured(obj)
	if u == nil {
		return
	}

	resourceNS := u.GetNamespace()
	if resourceNS == "" {
		resourceNS = corev1.NamespaceDefault
	}

	e.mu.RLock()
	states := make([]targetState, 0, len(e.targets))
	for _, st := range e.targets {
		tgvr := gvrFromProfile(st.profile.Spec.TargetGVK)
		if tgvr != gvr {
			continue
		}

		states = append(states, st)
	}
	e.mu.RUnlock()

	for _, st := range states {
		target := st.target
		targetKeyStr := targetKey(target.Namespace, target.Name)

		if deleted {
			e.store.Remove(target.Namespace, target.Name, string(u.GetUID()))
			metrics.CollectItemsTotal.Set(float64(e.store.Len()))
			e.refreshTargetSnapshotMetrics(st, target)

			continue
		}

		if !e.matchesTarget(ctx, st, gvr, u) {
			e.store.Remove(target.Namespace, target.Name, string(u.GetUID()))
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

		attrs, err := e.extractor.Extract(u, st.profile.Spec.Attributes)
		if err != nil {
			log.FromContext(ctx).Error(err, "extract attributes",
				"target", target.Namespace+"/"+target.Name,
				"resource", u.GetNamespace()+"/"+u.GetName())

			continue
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
	gvkLabel := fmt.Sprintf("%s/%s/%s", st.profile.Spec.TargetGVK.Group,
		st.profile.Spec.TargetGVK.Version, st.profile.Spec.TargetGVK.Kind)
	items := e.store.SnapshotTarget(target.Namespace, target.Name)
	metricPaths := MetricPathsFromProfile(st.profile.Spec)
	recordTargetSnapshotMetrics(target.Spec.ProfileRef, gvkLabel, items, metricPaths)
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
