// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package sink

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/validation"
)

type poolKey string

type pooledEntry struct {
	backend  Backend
	specHash string
	// key is poolEntryKey(specHash, secrets): an entry is reused only while the spec and every
	// referenced Secret's UID and resourceVersion are unchanged (SCR-1).
	key string
	// secrets is the metadata of the Secrets the backend was built from (no values).
	secrets  []secretVersion
	lastUsed time.Time
	// holders counts acquirers that have not released the entry yet. A retired entry is out of
	// the pool; its backend is closed by retireLocked when holders is 0, else by the last release
	// (SCR-3). retireReason names the retire in that Close's log line.
	holders      int
	retired      bool
	retireReason string
}

// poolGeneration is a key's store fence (design *Fingerprint and acquire* step 4): every store
// and every retire for the key gives it a new value from a pool-wide counter, and a key without
// one gets a fresh value when first read, so a value an acquire recorded never comes back, not
// even after the key aged out (an absent key reads as 0, which is never recorded). Kept while the
// key has no entry, aged out with the tombstones.
type poolGeneration struct {
	value   uint64
	changed time.Time
}

// backendPoolTTL bounds the lifetime of an idle pooled backend entry and of a
// delete-tombstone. globalBackendPool is a process-lifetime map keyed by sink
// UID/namespace-name; the family-sink controllers' delete watches evict a
// deleted sink's entry immediately (EvictBackendPoolForSink, BEP-1), so the
// TTL is the backstop that ages out entries and tombstones whose sink never
// produced a delete event (pipeline-mode one-shots, controller restarts) and
// keeps the pool bounded over the life of a long-running controller.
//
// This mirrors coalesceStateTTL's reasoning in internal/controller: it must
// stay comfortably above validation.MaxExportInterval (24h) so a slow-but-
// active sink's pooled backend is never pruned out from under it between
// export cycles, which would force an unnecessary reconnect/rebuild.
const backendPoolTTL = 2 * validation.MaxExportInterval

// timeNow is a test seam for backendPoolTTL pruning.
var timeNow = time.Now

var (
	backendPoolDisabled atomic.Bool

	globalBackendPool = struct {
		mu          sync.Mutex
		entries     map[poolKey]*pooledEntry
		tombstones  map[poolKey]time.Time
		generations map[poolKey]poolGeneration
		counter     uint64
	}{
		entries:     make(map[poolKey]*pooledEntry),
		tombstones:  make(map[poolKey]time.Time),
		generations: make(map[poolKey]poolGeneration),
	}
)

// DisableBackendPoolForTest turns off cross-export pooling (controller/envtest isolation).
func DisableBackendPoolForTest() {
	backendPoolDisabled.Store(true)
	ResetBackendPoolForTest()
}

// EnableBackendPoolForTest re-enables pooling after DisableBackendPoolForTest (test cleanup).
func EnableBackendPoolForTest() {
	backendPoolDisabled.Store(false)
}

// ResetBackendPoolForTest evicts all pooled backends, delete-tombstones and generations. It closes
// every pooled backend at once, held or not (test only); a later release of a detached entry
// closes nothing.
func ResetBackendPoolForTest() {
	globalBackendPool.mu.Lock()
	defer globalBackendPool.mu.Unlock()

	for k, e := range globalBackendPool.entries {
		closeBackendLogged(e.backend, "pool reset")
		delete(globalBackendPool.entries, k)
	}
	for k := range globalBackendPool.tombstones {
		delete(globalBackendPool.tombstones, k)
	}
	for k := range globalBackendPool.generations {
		delete(globalBackendPool.generations, k)
	}
}

func poolKeyForSink(sinkUID types.UID, sinkNamespace, sinkName string) poolKey {
	if sinkUID != "" {
		return poolKey("uid:" + string(sinkUID))
	}

	return poolKey("ns:" + sinkNamespace + "/" + sinkName)
}

func acquireBackend(
	ctx context.Context,
	c client.Client,
	reg *Registry,
	sinkNamespace, sinkName string,
	sinkUID types.UID,
	spec kollectdevv1alpha1.KollectSinkSpec,
) (Backend, func(), error) {
	return acquireBackendWithReader(ctx, c, nil, reg, sinkNamespace, sinkName, sinkUID, spec)
}

// acquireBackendWithReader returns the sink's backend: the pooled one while the spec and the
// referenced Secrets' metadata are unchanged, else one built from the current Secrets.
// secretReader reads those Secrets (uncached in production); nil falls back to c. The
// pool-disabled path keeps reading through c.
func acquireBackendWithReader(
	ctx context.Context,
	c client.Client,
	secretReader client.Reader,
	reg *Registry,
	sinkNamespace, sinkName string,
	sinkUID types.UID,
	spec kollectdevv1alpha1.KollectSinkSpec,
) (Backend, func(), error) {
	if reg == nil {
		return nil, func() {}, fmt.Errorf("sink registry is not configured")
	}

	specHash, err := specFingerprint(spec)
	if err != nil {
		return nil, func() {}, err
	}

	if backendPoolDisabled.Load() {
		buildCtx, berr := BuildContextFromSpec(ctx, c, spec, sinkNamespace)
		if berr != nil {
			return nil, func() {}, berr
		}

		backend, berr := reg.NewBackend(spec, buildCtx)
		if berr != nil {
			return nil, func() {}, berr
		}

		return backend, func() { closeBackendLogged(backend, "pool disabled release") }, nil
	}

	var reader client.Reader = c
	if secretReader != nil {
		reader = secretReader
	}

	defaultNamespace := sinkNamespace
	if defaultNamespace == "" {
		defaultNamespace = DefaultSecretNamespace
	}

	// Same order BuildContextFromSpec keeps: the CA source check before any Secret read.
	if caErr := CheckCASources(spec.TLS); caErr != nil {
		return nil, func() {}, caErr
	}

	key := poolKeyForSink(sinkUID, sinkNamespace, sinkName)
	uses := referencedSecretRefs(spec, defaultNamespace)

	look, err := observeAndLookup(ctx, reader, spec, defaultNamespace, key, specHash, uses)
	if err != nil {
		return nil, func() {}, err
	}
	if look.hit != nil {
		return look.hit, look.release, nil
	}
	previous := look.previous
	now := look.now

	// Miss: read the Secrets in full and build from those objects. The stored key comes from
	// the same objects, so key and values are one snapshot even if a Secret changed after
	// the metadata read.
	secrets, err := fetchSecrets(ctx, reader, uses)
	if err != nil {
		final := caRulesFirst(ctx, reader, spec, defaultNamespace, err)
		if isSecretGone(err) || isSecretGone(final) {
			// The full read observed a missing or empty Secret after the metadata read: retire
			// what is pooled now and fence builds in flight, as the metadata path does (SCR-2).
			retirePoolEntry(key, 0, true, "referenced secret missing or empty")
		}

		return nil, func() {}, final
	}
	built := versionsOf(uses, secrets)

	buildCtx, err := buildContextFromSecrets(ctx, spec, defaultNamespace, secrets)
	if err != nil {
		return nil, func() {}, err
	}

	backend, err := reg.NewBackend(spec, buildCtx)
	if err != nil {
		return nil, func() {}, err
	}

	if changed := changedSecrets(previous, built); previous != nil && len(changed) > 0 {
		sinkLog().Info("rebuilt sink backend",
			"sinkNamespace", sinkNamespace, "sinkName", sinkName,
			"reason", "referenced secret changed", "secrets", changed)
	}

	stored, release, discard := storePooledBackend(key, look.generation, pooledEntry{
		backend:  backend,
		specHash: specHash,
		key:      poolEntryKey(specHash, built),
		secrets:  built,
		lastUsed: now,
	})
	if discard != nil {
		closeBackendLogged(discard, "duplicate pool backend")
	}

	return stored, release, nil
}

// caRulesFirst keeps BuildContextFromSpec's error precedence on the pool path: when a credential
// Secret read failed, a CA Secret that is missing or has no CA key is reported instead of readErr.
// A failed read of the CA Secret itself is returned as is, as the CA-first order returned it. It
// reads the CA Secret only on this error path, so a pool hit never transfers its data.
func caRulesFirst(
	ctx context.Context,
	reader client.Reader,
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
	readErr error,
) error {
	caUse, ok := caSecretUse(spec, defaultNamespace)
	var failed *secretReadError
	if !ok || (errors.As(readErr, &failed) && failed.use == caUse) {
		return readErr
	}

	read := func(use secretUse) (*corev1.Secret, error) {
		secret := &corev1.Secret{}
		if err := getSecretObject(ctx, reader, use, secret); err != nil {
			return nil, err
		}
		if SecretDataEmpty(secret.Data) {
			return nil, &secretReadError{use: use, empty: true, err: EmptySecretError(use.Namespace, use.Name)}
		}

		return secret, nil
	}
	if _, _, caErr := checkCASecret(spec, defaultNamespace, read); caErr != nil {
		return caErr
	}

	return readErr
}

// poolReadAttempts bounds the metadata reads of one acquire. An attempt whose read may predate the
// pooled entry (the entry was stored after the read began) reads again; the last attempt acts on
// what it read, as the retire rules say.
const poolReadAttempts = 2

// poolLookup is lookupOrRetire's result: a hit with its release, or a miss with the generation
// the store fence compares against; reread asks for a fresh metadata read, and staleKey is the key
// of the entry the read did not retire.
type poolLookup struct {
	hit        Backend
	release    func()
	previous   []secretVersion
	generation uint64
	now        time.Time
	reread     bool
	staleKey   string
}

// observeAndLookup reads the referenced Secrets' metadata and looks the pool up with it. It
// records the key's generation before each read: when the pool changed after that, the read may
// be older than the pooled entry, so it does not retire the entry on that read but reads again.
// The re-read begins after the entry was stored, so it is at least as new as the entry's
// snapshot, and the last attempt retires on what it observed. A re-read that fails for another
// reason (timeout, cancelled, forbidden) observed nothing new, so the earlier successful read
// stands: the entry it did not match is retired if it is still pooled.
func observeAndLookup(
	ctx context.Context,
	reader client.Reader,
	spec kollectdevv1alpha1.KollectSinkSpec,
	defaultNamespace string,
	key poolKey,
	specHash string,
	uses []secretUse,
) (poolLookup, error) {
	staleKey := ""
	for attempt := 1; ; attempt++ {
		force := attempt >= poolReadAttempts
		readGeneration := poolGenerationOf(key)
		versions, err := secretMeta(ctx, reader, uses)
		if err != nil {
			// A missing or empty Secret retires the pooled backend, whether the metadata read or
			// the CA-rules read behind it saw it; any other failed read (timeout, cancelled
			// context, forbidden) observed no change and keeps a backend others share, unless an
			// earlier read of this acquire already observed a change against it.
			final := caRulesFirst(ctx, reader, spec, defaultNamespace, err)
			if isSecretGone(err) || isSecretGone(final) {
				retired, entryKey := retirePoolEntry(key, readGeneration, force, "referenced secret missing or empty")
				if !retired {
					staleKey = entryKey
					continue
				}
			} else if staleKey != "" {
				retireEntryWithKey(key, staleKey, "referenced secret change")
			}

			return poolLookup{}, final
		}

		look := lookupOrRetire(key, specHash, poolEntryKey(specHash, versions), readGeneration, force, timeNow())
		if !look.reread {
			return look, nil
		}
		staleKey = look.staleKey
	}
}

// poolGenerationOf returns key's current store-fence generation.
func poolGenerationOf(key poolKey) uint64 {
	globalBackendPool.mu.Lock()
	defer globalBackendPool.mu.Unlock()

	return generationLocked(key, timeNow())
}

// generationLocked returns key's generation, giving a key without one a fresh value. Caller holds
// globalBackendPool.mu.
func generationLocked(key poolKey, now time.Time) uint64 {
	if _, ok := globalBackendPool.generations[key]; !ok {
		bumpGenerationLocked(key, now)
	}

	return globalBackendPool.generations[key].value
}

// lookupOrRetire returns the pooled backend for key, held, when its entry key matches entryKey.
// Otherwise the read observed a change: the old entry is retired now, whatever the rebuild then
// does, so a failed rebuild never leaves it in use (SCR-2, SCR-3); previous is its Secrets'
// metadata when only Secrets changed (for the rebuild log). Unless force is set, an entry stored
// after readGeneration was taken is left pooled and the lookup asks for a re-read (the read may
// predate it). It also prunes idle entries; every Close runs after Unlock, since Pool.Close must
// not run while globalBackendPool.mu is held.
func lookupOrRetire(key poolKey, specHash, entryKey string, readGeneration uint64, force bool, now time.Time) poolLookup {
	look := poolLookup{now: now}
	var closeNow Backend
	retireReason := "spec hash change"

	globalBackendPool.mu.Lock()
	stale := pruneStaleEntriesLocked(now)
	if entry, ok := globalBackendPool.entries[key]; ok {
		switch {
		case entry.key == entryKey:
			entry.lastUsed = now
			look.hit, look.release = entry.backend, holdLocked(entry)
		case !force && generationLocked(key, now) != readGeneration:
			look.reread, look.staleKey = true, entry.key
		default:
			if entry.specHash == specHash {
				look.previous = entry.secrets
				retireReason = "referenced secret change"
			}
			closeNow = retireLocked(key, retireReason, now)
		}
	}
	look.generation = generationLocked(key, now)
	globalBackendPool.mu.Unlock()

	closeBackendsLogged(stale, "ttl expired")
	if closeNow != nil {
		closeBackendLogged(closeNow, retireReason)
	}

	return look
}

// retirePoolEntry retires the pooled entry for key and reports true, or, unless force is set,
// keeps a pooled entry stored after readGeneration was taken (the observation may predate it) and
// reports false with that entry's key. The Close, if due now, runs after Unlock.
func retirePoolEntry(key poolKey, readGeneration uint64, force bool, reason string) (bool, string) {
	now := timeNow()
	globalBackendPool.mu.Lock()
	entry, pooled := globalBackendPool.entries[key]
	if pooled && !force && generationLocked(key, now) != readGeneration {
		globalBackendPool.mu.Unlock()

		return false, entry.key
	}
	closeNow := retireLocked(key, reason, now)
	globalBackendPool.mu.Unlock()
	if closeNow != nil {
		closeBackendLogged(closeNow, reason)
	}

	return true, ""
}

// retireEntryWithKey retires key's pooled entry only while it is still the entry with entryKey.
func retireEntryWithKey(key poolKey, entryKey, reason string) {
	globalBackendPool.mu.Lock()
	var closeNow Backend
	if entry, ok := globalBackendPool.entries[key]; ok && entry.key == entryKey {
		closeNow = retireLocked(key, reason, timeNow())
	}
	globalBackendPool.mu.Unlock()
	if closeNow != nil {
		closeBackendLogged(closeNow, reason)
	}
}

// bumpGenerationLocked gives key a new store-fence generation. Caller holds globalBackendPool.mu.
func bumpGenerationLocked(key poolKey, now time.Time) {
	globalBackendPool.counter++
	globalBackendPool.generations[key] = poolGeneration{value: globalBackendPool.counter, changed: now}
}

// retireLocked removes key's entry from the pool, marks it retired and bumps the key's generation
// (also when no entry is pooled, so a build in flight is not pooled after a retire). It returns
// the backend to close now when no acquirer holds it; otherwise the last release closes it. The
// caller holds globalBackendPool.mu and closes the returned backend after Unlock.
func retireLocked(key poolKey, reason string, now time.Time) Backend {
	bumpGenerationLocked(key, now)
	entry, ok := globalBackendPool.entries[key]
	if !ok {
		return nil
	}
	delete(globalBackendPool.entries, key)
	entry.retired = true
	entry.retireReason = reason
	if entry.holders > 0 {
		return nil
	}

	return entry.backend
}

// holdLocked takes a hold on entry and returns its release. The release is idempotent; the one
// that drops the last hold on a retired entry closes its backend, after Unlock. Caller holds
// globalBackendPool.mu.
func holdLocked(entry *pooledEntry) func() {
	entry.holders++
	var once sync.Once

	return func() {
		once.Do(func() {
			globalBackendPool.mu.Lock()
			entry.holders--
			drained := entry.retired && entry.holders == 0
			globalBackendPool.mu.Unlock()
			if drained {
				closeBackendLogged(entry.backend, entry.retireReason)
			}
		})
	}
}

// owningRelease closes backend once, for a build that was not pooled.
func owningRelease(backend Backend, reason string) func() {
	var once sync.Once

	return func() { once.Do(func() { closeBackendLogged(backend, reason) }) }
}

// storePooledBackend pools next under key behind the store fence (design *Fingerprint and
// acquire* step 4) and returns the backend for the caller, its release, and a surplus build to
// close now. In order: a key that carries a delete-tombstone pools nothing (BEP-1); an entry with
// next's key is shared (held) and next is surplus; the key's generation unchanged since the miss
// pools next (an aged-out generation reads as 0, never recorded); otherwise another acquire stored
// or retired in between, and next is
// handed to the caller under an owning release that closes it, leaving the pool as it is. The
// function unlocks before returning so the caller can Close without holding globalBackendPool.mu.
func storePooledBackend(key poolKey, generation uint64, next pooledEntry) (Backend, func(), Backend) {
	globalBackendPool.mu.Lock()
	defer globalBackendPool.mu.Unlock()

	if _, deleted := globalBackendPool.tombstones[key]; deleted {
		// Nothing was pooled for the deleted sink: the caller owns the built backend and its
		// release Closes it once the export is done.
		return next.backend, owningRelease(next.backend, "delete tombstone release"), nil
	}

	old, ok := globalBackendPool.entries[key]
	if ok && old.key == next.key {
		old.lastUsed = next.lastUsed

		return old.backend, holdLocked(old), next.backend
	}

	if !ok && globalBackendPool.generations[key].value == generation {
		entry := &next
		globalBackendPool.entries[key] = entry
		bumpGenerationLocked(key, next.lastUsed)

		return entry.backend, holdLocked(entry), nil
	}

	return next.backend, owningRelease(next.backend, "superseded build release"), nil
}

func closeBackendsLogged(backends []Backend, reason string) {
	for _, backend := range backends {
		closeBackendLogged(backend, reason)
	}
}

// pruneStaleEntriesLocked removes entries idle longer than backendPoolTTL and
// returns their backends, and ages out delete-tombstones on the same
// opportunistic cycle (a tombstone older than backendPoolTTL stops guarding
// its key). Caller holds globalBackendPool.mu and must Close those backends
// only after Unlock; Pool.Close under the mutex stalls every other acquire.
// Called on every acquire so deleted sinks age out (AR-11).
func pruneStaleEntriesLocked(now time.Time) []Backend {
	stale := make([]Backend, 0, len(globalBackendPool.entries))
	for k, entry := range globalBackendPool.entries {
		if entry == nil {
			delete(globalBackendPool.entries, k)
			continue
		}
		// A held entry is in use; the TTL never retires it (SCR-3).
		if entry.holders == 0 && now.Sub(entry.lastUsed) > backendPoolTTL {
			if b := retireLocked(k, "ttl expired", now); b != nil {
				stale = append(stale, b)
			}
		}
	}

	pruneExpiredTombstonesLocked(now)

	return stale
}

// pruneExpiredTombstonesLocked drops delete-tombstones older than
// backendPoolTTL (a tombstone older than the TTL stops guarding its key).
// Tombstones carry no backends, so no Close follows; caller holds
// globalBackendPool.mu. Run on the acquire path's opportunistic cycle and on
// the delete-eviction hook (evictPoolKeyForDelete) so a manager that never
// exports — its only pool touchpoint being delete evictions — still bounds
// the tombstone map instead of leaking one tombstone per delete for the life
// of the process (BEP-2).
func pruneExpiredTombstonesLocked(now time.Time) {
	for k, tombstoned := range globalBackendPool.tombstones {
		if now.Sub(tombstoned) > backendPoolTTL {
			delete(globalBackendPool.tombstones, k)
		}
	}
	// A generation guards only builds in flight; with no entry for its key it ages out on the
	// same cycle. Values come from a pool-wide counter, so a dropped one is never reissued.
	for k, generation := range globalBackendPool.generations {
		if _, pooled := globalBackendPool.entries[k]; !pooled && now.Sub(generation.changed) > backendPoolTTL {
			delete(globalBackendPool.generations, k)
		}
	}
}

func specFingerprint(spec kollectdevv1alpha1.KollectSinkSpec) (string, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("hash sink spec: %w", err)
	}

	sum := sha256.Sum256(b)

	return hex.EncodeToString(sum[:]), nil
}

// EvictBackendPool removes a cached backend by namespace/name (test utility;
// production evictions go through EvictBackendPoolForSink).
func EvictBackendPool(namespace, name string) {
	if backendPoolDisabled.Load() {
		return
	}

	evictPoolKey(poolKeyForSink("", namespace, name))
}

// EvictBackendPoolByUID removes a cached backend keyed by sink object UID
// (test utility; production evictions go through EvictBackendPoolForSink).
func EvictBackendPoolByUID(uid types.UID) {
	if backendPoolDisabled.Load() || uid == "" {
		return
	}

	evictPoolKey(poolKeyForSink(uid, "", ""))
}

// EvictBackendPoolForSink is the delete-hook seam for the family-sink
// controllers (BEP-1): evicting a deleted sink's pooled backend by its object
// UID, falling back to the sink's namespace/name key only when the delete
// event's object carries no UID. Eviction is best-effort and idempotent: it
// never re-exports, retracts or reconciles, and deleting a sink with no pooled
// entry must not fail. It also records a delete-tombstone for the evicted key
// so an acquire-build still in flight when the delete landed is discarded
// instead of re-pooled for the deleted sink.
func EvictBackendPoolForSink(sinkUID types.UID, sinkNamespace, sinkName string) {
	if backendPoolDisabled.Load() {
		return
	}

	evictPoolKeyForDelete(poolKeyForSink(sinkUID, sinkNamespace, sinkName))
}

// evictPoolKeyForDelete drops the entry for key and records a delete-tombstone
// so storePooledBackend discards any backend built for the key afterwards.
// Only the delete hook records tombstones: TTL pruning, the acquire-time
// spec-hash swap and the EvictBackendPool/EvictBackendPoolByUID test helpers
// evict live entries and must never block a live sink's next build.
func evictPoolKeyForDelete(key poolKey) {
	now := timeNow()
	globalBackendPool.mu.Lock()
	closeNow := retireLocked(key, "sink delete eviction", now)
	globalBackendPool.tombstones[key] = now
	// Opportunistic sweep on the eviction path: a manager that only deletes
	// sinks (no exports, so acquireBackend's prune cycle never runs) ages its
	// tombstones out here instead of leaking one entry per delete. Tombstones
	// carry no backends, so the sweep is mutex-only.
	pruneExpiredTombstonesLocked(now)
	globalBackendPool.mu.Unlock()
	if closeNow != nil {
		closeBackendLogged(closeNow, "sink delete eviction")
	}
}

func evictPoolKey(key poolKey) {
	globalBackendPool.mu.Lock()
	closeNow := retireLocked(key, "explicit eviction", timeNow())
	globalBackendPool.mu.Unlock()
	if closeNow != nil {
		closeBackendLogged(closeNow, "explicit eviction")
	}
}
