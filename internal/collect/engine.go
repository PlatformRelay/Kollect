// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

const informerResync = 12 * time.Hour

type targetState struct {
	target  kollectdevv1alpha1.KollectTarget
	profile kollectdevv1alpha1.KollectProfile
}

// Engine registers dynamic informers per profile GVK and writes extracted attributes to Store.
type Engine struct {
	dynamic   dynamic.Interface
	extractor *Extractor
	store     *Store
	runCtx    context.Context

	mu        sync.RWMutex
	factories map[schema.GroupVersionResource]dynamicinformer.DynamicSharedInformerFactory
	started   map[schema.GroupVersionResource]bool
	targets   map[string]targetState
}

// NewEngine constructs a collection engine.
func NewEngine(dynamicClient dynamic.Interface, store *Store) (*Engine, error) {
	ext, err := NewExtractor()
	if err != nil {
		return nil, err
	}

	return &Engine{
		dynamic:   dynamicClient,
		extractor: ext,
		store:     store,
		factories: make(map[schema.GroupVersionResource]dynamicinformer.DynamicSharedInformerFactory),
		started:   make(map[schema.GroupVersionResource]bool),
		targets:   make(map[string]targetState),
	}, nil
}

// RegisterTarget records the target and ensures a dynamic informer exists for its profile GVK.
func (e *Engine) RegisterTarget(
	ctx context.Context,
	target *kollectdevv1alpha1.KollectTarget,
	profile *kollectdevv1alpha1.KollectProfile,
) error {
	key := targetKey(target.Namespace, target.Name)

	if target.Spec.Suspend {
		e.UnregisterTarget(target.Namespace, target.Name)

		return nil
	}

	gvr := gvrFromProfile(profile.Spec.TargetGVK)

	e.mu.Lock()
	e.targets[key] = targetState{target: *target.DeepCopy(), profile: *profile.DeepCopy()}
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
	e.mu.Unlock()

	e.store.RemoveTarget(namespace, name)
}

// ItemCount returns collected items for a target.
func (e *Engine) ItemCount(namespace, name string) int {
	return e.store.CountForTarget(namespace, name)
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

func (e *Engine) startInformer(ctx context.Context, gvr schema.GroupVersionResource) error {
	e.mu.Lock()
	if e.started[gvr] {
		e.mu.Unlock()

		return nil
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		e.dynamic,
		informerResync,
		metav1.NamespaceAll,
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

	return nil
}

func (e *Engine) dispatch(ctx context.Context, gvr schema.GroupVersionResource, obj interface{}, deleted bool) {
	u := toUnstructured(obj)
	if u == nil {
		return
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
		if !matchesTarget(&target, u) {
			continue
		}

		if deleted {
			e.store.Remove(target.Namespace, target.Name, string(u.GetUID()))

			continue
		}

		attrs, err := e.extractor.Extract(u, st.profile.Spec.Attributes)
		if err != nil {
			log.FromContext(ctx).Error(err, "extract attributes",
				"target", target.Namespace+"/"+target.Name,
				"resource", u.GetNamespace()+"/"+u.GetName())

			continue
		}

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
	}
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

func matchesTarget(target *kollectdevv1alpha1.KollectTarget, u *unstructured.Unstructured) bool {
	if len(target.Spec.Names) > 0 {
		found := false
		for _, n := range target.Spec.Names {
			if n == u.GetName() {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	if target.Spec.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(target.Spec.LabelSelector)
		if err != nil {
			return false
		}

		if !selector.Matches(labels.Set(u.GetLabels())) {
			return false
		}
	}

	return true
}

func gvrFromProfile(gvk kollectdevv1alpha1.GroupVersionKind) schema.GroupVersionResource {
	plural, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	})

	return plural
}
