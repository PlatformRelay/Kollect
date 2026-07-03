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

// storeShard holds targets for one target namespace (PERF-06 namespace sharding).
type storeShard struct {
	mu      sync.RWMutex
	targets map[string]map[string]Item // targetName -> uid -> Item
}

// Store holds collected items keyed by target namespace/name and resource UID.
//
// Scale notes (10k+ objects / 100+ clusters):
//   - Memory is O(n) in collected object count; one map entry per resource UID.
//   - Shards partition by target namespace so export snapshots and upserts contend
//     on narrower locks than a single global store mutex.
type Store struct {
	shardsMu   sync.RWMutex
	shards     map[string]*storeShard
	watchMu    sync.RWMutex
	watchers   map[chan struct{}]struct{}
	nsWatchers map[chan string]struct{}

	// versionMu/versions track a per-namespace mutation counter (AR-10),
	// deliberately kept separate from shards: RemoveCluster deletes the
	// shard entirely, and a counter living on the shard would reset to 0
	// and could re-issue a value a stale cache entry already holds. Keeping
	// it in its own map (never deleted, only incremented) makes it survive
	// shard deletion/recreation and stay strictly monotonic per namespace.
	versionMu sync.Mutex
	versions  map[string]uint64
}

// NewStore returns an empty in-memory collection store.
func NewStore() *Store {
	return &Store{
		shards:     make(map[string]*storeShard),
		watchers:   make(map[chan struct{}]struct{}),
		nsWatchers: make(map[chan string]struct{}),
		versions:   make(map[string]uint64),
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

func (s *Store) shardFor(targetNamespace string) *storeShard {
	s.shardsMu.RLock()
	sh, ok := s.shards[targetNamespace]
	s.shardsMu.RUnlock()
	if ok {
		return sh
	}

	s.shardsMu.Lock()
	defer s.shardsMu.Unlock()

	if sh, ok = s.shards[targetNamespace]; ok {
		return sh
	}

	sh = &storeShard{targets: make(map[string]map[string]Item)}
	s.shards[targetNamespace] = sh

	return sh
}

// Upsert records or replaces an item for a target.
func (s *Store) Upsert(item Item) {
	sh := s.shardFor(item.TargetNamespace)

	sh.mu.Lock()
	if sh.targets[item.TargetName] == nil {
		sh.targets[item.TargetName] = make(map[string]Item)
	}

	sh.targets[item.TargetName][item.UID] = item
	// Bump while still holding sh.mu (AR-10): content and version must
	// commit atomically, or a reader can observe the new content via
	// SnapshotNamespace while NamespaceVersion still reports the old value
	// — a stale cache *hit* that returns a fingerprint for content that no
	// longer matches it, not just a harmless miss.
	s.bumpNamespaceVersion(item.TargetNamespace)
	sh.mu.Unlock()

	s.notifyWatchers(item.TargetNamespace)
}

// bumpNamespaceVersion increments the mutation counter for namespace (AR-10).
// Deliberately independent of the shard map: see the Store.versions doc comment.
func (s *Store) bumpNamespaceVersion(namespace string) {
	s.versionMu.Lock()
	s.versions[namespace]++
	s.versionMu.Unlock()
}

// NamespaceVersion returns a counter bumped on every mutation (Upsert/Remove/
// RemoveTarget/RemoveCluster) scoped to this namespace (AR-10). Two reads
// returning the same value guarantee the namespace's item content has not
// changed in between, so callers can skip a full SnapshotNamespace + content
// fingerprint recompute when the version is unchanged since the last one
// they computed. The counter is strictly monotonic per namespace for the
// lifetime of the Store — including across RemoveCluster, which deletes the
// shard but never the version entry, so a value is never re-issued. Every
// bump happens while the caller still holds the content lock (shard mu or
// shardsMu) that made the corresponding change visible, so "this version was
// observed" always implies "this content (or later) is visible" — never the
// reverse, which would let a cache entry serve a fingerprint for content
// that has already moved on.
func (s *Store) NamespaceVersion(namespace string) uint64 {
	s.versionMu.Lock()
	defer s.versionMu.Unlock()

	return s.versions[namespace]
}

// RemoveTarget drops all items for a target.
func (s *Store) RemoveTarget(targetNamespace, targetName string) {
	sh := s.shardFor(targetNamespace)

	sh.mu.Lock()
	delete(sh.targets, targetName)
	s.bumpNamespaceVersion(targetNamespace) // atomic with the content change; see Upsert
	sh.mu.Unlock()

	s.notifyWatchers(targetNamespace)
}

// RemoveCluster drops all targets for one cluster target namespace.
func (s *Store) RemoveCluster(cluster string) {
	if cluster == "" {
		return
	}

	s.shardsMu.Lock()
	delete(s.shards, cluster)
	s.bumpNamespaceVersion(cluster) // atomic with the content change; see Upsert
	s.shardsMu.Unlock()

	s.notifyWatchers(cluster)
}

// CountForTarget returns items collected for one target.
func (s *Store) CountForTarget(targetNamespace, targetName string) int {
	sh := s.shardFor(targetNamespace)

	sh.mu.RLock()
	defer sh.mu.RUnlock()

	return len(sh.targets[targetName])
}

// SnapshotTarget returns all items for one target.
func (s *Store) SnapshotTarget(targetNamespace, targetName string) []Item {
	sh := s.shardFor(targetNamespace)

	sh.mu.RLock()
	defer sh.mu.RUnlock()

	bucket := sh.targets[targetName]
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
	sh := s.shardFor(targetNamespace)

	sh.mu.Lock()
	if bucket, ok := sh.targets[targetName]; ok {
		delete(bucket, uid)
		if len(bucket) == 0 {
			delete(sh.targets, targetName)
		}
		s.bumpNamespaceVersion(targetNamespace) // atomic with the content change; see Upsert
	}
	sh.mu.Unlock()

	s.notifyWatchers(targetNamespace)
}

// CountForNamespace returns total items for targets in the given namespace.
func (s *Store) CountForNamespace(namespace string) int {
	sh := s.shardFor(namespace)

	sh.mu.RLock()
	defer sh.mu.RUnlock()

	total := 0
	for _, bucket := range sh.targets {
		total += len(bucket)
	}

	return total
}

// SnapshotNamespace returns all items for targets in a namespace.
func (s *Store) SnapshotNamespace(namespace string) []Item {
	sh := s.shardFor(namespace)

	sh.mu.RLock()
	defer sh.mu.RUnlock()

	var out []Item
	for _, bucket := range sh.targets {
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
	s.shardsMu.RLock()
	shards := make([]*storeShard, 0, len(s.shards))
	for _, sh := range s.shards {
		shards = append(shards, sh)
	}
	s.shardsMu.RUnlock()

	total := 0
	for _, sh := range shards {
		sh.mu.RLock()
		for _, bucket := range sh.targets {
			total += len(bucket)
		}
		sh.mu.RUnlock()
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
	s.shardsMu.RLock()
	shards := make([]*storeShard, 0, len(s.shards))
	for _, sh := range s.shards {
		shards = append(shards, sh)
	}
	s.shardsMu.RUnlock()

	var out []Item
	for _, sh := range shards {
		sh.mu.RLock()
		for _, bucket := range sh.targets {
			for _, item := range bucket {
				out = append(out, item)
			}
		}
		sh.mu.RUnlock()
	}

	return out
}
