// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"encoding/json"
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
	mu       sync.RWMutex
	items    map[string]map[string]Item
	watchMu  sync.RWMutex
	watchers map[chan struct{}]struct{}
}

// NewStore returns an empty in-memory collection store.
func NewStore() *Store {
	return &Store{
		items:    make(map[string]map[string]Item),
		watchers: make(map[chan struct{}]struct{}),
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

func (s *Store) notifyWatchers() {
	s.watchMu.RLock()
	defer s.watchMu.RUnlock()

	for ch := range s.watchers {
		select {
		case ch <- struct{}{}:
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
	s.notifyWatchers()
}

// RemoveTarget drops all items for a target.
func (s *Store) RemoveTarget(targetNamespace, targetName string) {
	key := targetKey(targetNamespace, targetName)

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.items, key)
	s.notifyWatchers()
}

// CountForTarget returns items collected for one target.
func (s *Store) CountForTarget(targetNamespace, targetName string) int {
	key := targetKey(targetNamespace, targetName)

	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.items[key])
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

// MarshalTargetJSON returns a JSON array of items for one target.
func (s *Store) MarshalTargetJSON(targetNamespace, targetName string) ([]byte, error) {
	return json.Marshal(s.SnapshotTarget(targetNamespace, targetName))
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

	s.notifyWatchers()
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

// MarshalNamespaceJSON returns a JSON array of items in the namespace.
func (s *Store) MarshalNamespaceJSON(namespace string) ([]byte, error) {
	return json.Marshal(s.SnapshotNamespace(namespace))
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
