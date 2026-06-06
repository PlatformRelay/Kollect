// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"sync"
)

// Item is one collected resource with extracted attributes.
type Item struct {
	TargetNamespace string         `json:"targetNamespace"`
	TargetName      string         `json:"targetName"`
	Namespace       string         `json:"namespace"`
	Name            string         `json:"name"`
	Group           string         `json:"group,omitempty"`
	Version         string         `json:"version"`
	Kind            string         `json:"kind"`
	UID             string         `json:"uid"`
	Attributes      map[string]any `json:"attributes"`
}

// Store holds collected items keyed by target namespace/name and resource UID.
//
// Scale notes (10k+ objects / 100+ clusters):
//   - Memory is O(n) in collected object count; one map entry per resource UID.
//   - A single RWMutex guards nested maps — export snapshots hold RLock for O(n) copies;
//     at 10k+ rows consider namespace-sharded stores (shard key = target namespace) to
//     reduce lock contention and snapshot payload size.
//   - Hub deployments should not mirror full spoke stores; push summarized deltas only.
type Store struct {
	mu         sync.RWMutex
	items      map[string]map[string]Item
	watchMu    sync.RWMutex
	watchers   map[chan struct{}]struct{}
	nsWatchers map[chan string]struct{}
}

// NewStore returns an empty in-memory collection store.
func NewStore() *Store {
	return &Store{
		items:      make(map[string]map[string]Item),
		watchers:   make(map[chan struct{}]struct{}),
		nsWatchers: make(map[chan string]struct{}),
	}
}

// Subscribe returns a channel that receives a signal whenever the store changes.
// The caller must call Unsubscribe when done.
func (s *Store) Subscribe() chan struct{} {
	ch := make(chan struct{}, 1)

	s.watchMu.Lock()
	s.watchers[ch] = struct{}{}
	s.watchMu.Unlock()

	return ch
}

// Unsubscribe removes a watcher created by Subscribe.
func (s *Store) Unsubscribe(ch chan struct{}) {
	s.watchMu.Lock()
	delete(s.watchers, ch)
	s.watchMu.Unlock()
}

// SubscribeNamespaces returns a channel that receives the target namespace on store changes.
// The caller must call UnsubscribeNamespaces when done.
func (s *Store) SubscribeNamespaces() chan string {
	ch := make(chan string, 8)

	s.watchMu.Lock()
	s.nsWatchers[ch] = struct{}{}
	s.watchMu.Unlock()

	return ch
}

// UnsubscribeNamespaces removes a watcher created by SubscribeNamespaces.
func (s *Store) UnsubscribeNamespaces(ch chan string) {
	s.watchMu.Lock()
	delete(s.nsWatchers, ch)
	s.watchMu.Unlock()
}

func (s *Store) notifyWatchers(targetNamespace string) {
	s.watchMu.RLock()
	defer s.watchMu.RUnlock()

	for ch := range s.watchers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	for ch := range s.nsWatchers {
		select {
		case ch <- targetNamespace:
		default:
		}
	}
}

func targetKey(namespace, name string) string {
	return namespace + "/" + name
}

// Upsert records or replaces an item for a target.
func (s *Store) Upsert(item Item) {
	key := targetKey(item.TargetNamespace, item.TargetName)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.items[key] == nil {
		s.items[key] = make(map[string]Item)
	}

	s.items[key][item.UID] = item
	s.notifyWatchers(item.TargetNamespace)
}

// RemoveTarget drops all items for a target.
func (s *Store) RemoveTarget(targetNamespace, targetName string) {
	key := targetKey(targetNamespace, targetName)

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.items, key)
	s.notifyWatchers(targetNamespace)
}

// RemoveCluster drops all targets keyed under cluster (hub merge uses cluster as target namespace).
func (s *Store) RemoveCluster(cluster string) {
	if cluster == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for key := range s.items {
		if !hasPrefixNamespace(key, cluster) {
			continue
		}

		delete(s.items, key)
	}

	s.notifyWatchers(cluster)
}

// CountForTarget returns items collected for one target.
func (s *Store) CountForTarget(targetNamespace, targetName string) int {
	key := targetKey(targetNamespace, targetName)

	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.items[key])
}

// CloneTargetItems returns a shallow copy of the target bucket for rollback (hub merge+export).
func (s *Store) CloneTargetItems(targetNamespace, targetName string) map[string]Item {
	key := targetKey(targetNamespace, targetName)

	s.mu.RLock()
	defer s.mu.RUnlock()

	bucket := s.items[key]
	if len(bucket) == 0 {
		return nil
	}

	out := make(map[string]Item, len(bucket))
	for uid, item := range bucket {
		out[uid] = item
	}

	return out
}

// RestoreTarget replaces all items for a target (nil or empty prior removes the target).
func (s *Store) RestoreTarget(targetNamespace, targetName string, prior map[string]Item) {
	key := targetKey(targetNamespace, targetName)

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(prior) == 0 {
		delete(s.items, key)
	} else {
		cp := make(map[string]Item, len(prior))
		for uid, item := range prior {
			cp[uid] = item
		}

		s.items[key] = cp
	}

	s.notifyWatchers(targetNamespace)
}

// SnapshotTarget returns all items for one target (hub merge uses cluster as target namespace).
func (s *Store) SnapshotTarget(targetNamespace, targetName string) []Item {
	key := targetKey(targetNamespace, targetName)

	s.mu.RLock()
	defer s.mu.RUnlock()

	bucket := s.items[key]
	if len(bucket) == 0 {
		return nil
	}

	out := make([]Item, 0, len(bucket))
	for _, item := range bucket {
		out = append(out, item)
	}

	return out
}

// MarshalTargetJSON returns a versioned export envelope for one target (ADR-0405).
func (s *Store) MarshalTargetJSON(targetNamespace, targetName string) ([]byte, error) {
	return s.MarshalTargetExport(targetNamespace, targetName, ExportMetadata{})
}

// MarshalTargetExport returns a versioned export envelope for one target.
func (s *Store) MarshalTargetExport(
	targetNamespace, targetName string,
	meta ExportMetadata,
) ([]byte, error) {
	return MarshalExportEnvelope(s.SnapshotTarget(targetNamespace, targetName), meta)
}

// Remove deletes an item by target and resource UID.
func (s *Store) Remove(targetNamespace, targetName, uid string) {
	key := targetKey(targetNamespace, targetName)

	s.mu.Lock()
	defer s.mu.Unlock()

	if bucket, ok := s.items[key]; ok {
		delete(bucket, uid)
		if len(bucket) == 0 {
			delete(s.items, key)
		}
	}

	s.notifyWatchers(targetNamespace)
}

// CountForNamespace returns total items for targets in the given namespace.
func (s *Store) CountForNamespace(namespace string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for key, bucket := range s.items {
		if !hasPrefixNamespace(key, namespace) {
			continue
		}

		total += len(bucket)
	}

	return total
}

func hasPrefixNamespace(targetKey, namespace string) bool {
	prefix := namespace + "/"
	return len(targetKey) > len(prefix) && targetKey[:len(prefix)] == prefix
}

// SnapshotNamespace returns all items for targets in a namespace.
func (s *Store) SnapshotNamespace(namespace string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []Item
	for key, bucket := range s.items {
		if !hasPrefixNamespace(key, namespace) {
			continue
		}

		for _, item := range bucket {
			out = append(out, item)
		}
	}

	return out
}

// MarshalNamespaceJSON returns a versioned export envelope for the namespace (ADR-0405).
func (s *Store) MarshalNamespaceJSON(namespace string) ([]byte, error) {
	return s.MarshalNamespaceExport(namespace, ExportMetadata{})
}

// MarshalNamespaceExport returns a versioned export envelope for the namespace.
func (s *Store) MarshalNamespaceExport(namespace string, meta ExportMetadata) ([]byte, error) {
	return MarshalExportEnvelope(s.SnapshotNamespace(namespace), meta)
}

// TotalCount returns the number of items across all targets.
func (s *Store) TotalCount() int {
	return s.Len()
}

// Len returns the number of items across all targets (used by metrics and HTTP summary).
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, bucket := range s.items {
		total += len(bucket)
	}

	return total
}

// NamespaceSummary is the HTTP inventory payload for one namespace.
type NamespaceSummary struct {
	Namespace string `json:"namespace"`
	ItemCount int    `json:"itemCount"`
	Items     []Item `json:"items"`
}

// Summary builds an aggregated inventory snapshot for optional namespace filter.
func (s *Store) Summary(namespace string) NamespaceSummary {
	items := s.SnapshotNamespace(namespace)
	if namespace == "" {
		items = s.snapshotAll()
	}

	return NamespaceSummary{
		Namespace: namespace,
		ItemCount: len(items),
		Items:     items,
	}
}

func (s *Store) snapshotAll() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []Item
	for _, bucket := range s.items {
		for _, item := range bucket {
			out = append(out, item)
		}
	}

	return out
}
