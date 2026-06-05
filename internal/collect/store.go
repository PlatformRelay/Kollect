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
type Store struct {
	mu    sync.RWMutex
	items map[string]map[string]Item
}

// NewStore returns an empty in-memory collection store.
func NewStore() *Store {
	return &Store{items: make(map[string]map[string]Item)}
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
}

// RemoveTarget drops all items for a target.
func (s *Store) RemoveTarget(targetNamespace, targetName string) {
	key := targetKey(targetNamespace, targetName)

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.items, key)
}

// CountForTarget returns items collected for one target.
func (s *Store) CountForTarget(targetNamespace, targetName string) int {
	key := targetKey(targetNamespace, targetName)

	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.items[key])
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
