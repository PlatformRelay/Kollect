// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"testing"
	"time"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	"github.com/platformrelay/kollect/internal/collect"
	"github.com/platformrelay/kollect/internal/export"
	"github.com/platformrelay/kollect/internal/sink"
	"github.com/platformrelay/kollect/internal/sink/git"
)

// FuzzOwnedPrune drives sequences of git-layout exports through the production entry point
// (sink.RunExportEnvelope -> layout projection -> prune-owner / multipart / legacy-identity
// decisions -> git ownership engine on an in-memory worktree) and checks every call against a
// small reference model of owned-path pruning. The model never consults the implementation: it
// knows which inventory owns which path from the operations it has seen, and predicts the repo
// file set, the rejections and the deletions from that alone.
//
// Program bytes (each opcode byte is taken mod 5; missing bytes read as 0):
//
//	0 owner mask             complete single-part export of the resources in mask
//	1 owner n masks... cut   multipart export of 2+n%2 parts; cut%3==0 interrupts it
//	2 idx                    a non-Kollect file appears at unknownPaths[idx]
//	3 back                   replay an earlier export operation (A->B->A, retries)
//	4                        retry the final part of the latest multipart set, same accumulator
func FuzzOwnedPrune(f *testing.F) {
	for _, seed := range ownedPruneSeeds() {
		f.Add(seed)
	}

	sink.DisableBackendPoolForTest()
	f.Cleanup(func() {
		sink.EnableBackendPoolForTest()
		sink.ResetBackendPoolForTest()
		sink.ResetBreakersForTest()
	})

	f.Fuzz(func(t *testing.T, program []byte) {
		newOwnedPruneHarness(t).run(program)
	})
}

// --- vocabulary --------------------------------------------------------------------------------

type fuzzOwner struct {
	label      string
	objectPath string // single-part object path; multipart parts are partitioned from it
	manifest   string // per-set sidecar path a complete multipart set writes
	legacy     bool   // the shared inventory/cluster identity: must never prune
}

// Owner 2 is a single-part inventory whose literal name is suffix-shaped; owner 0's multipart part
// 1 renders the very same object path, but the two are distinct inventories.
var fuzzOwners = []fuzzOwner{
	{label: "A(team-a/apps)", objectPath: "inventory/team-a/apps.json", manifest: "inventory/team-a/apps.manifest.json"},
	{label: "B(team-b/apps)", objectPath: "inventory/team-b/apps.json", manifest: "inventory/team-b/apps.manifest.json"},
	{
		label:      "C(team-a/apps.part-0001-of-0002)",
		objectPath: "inventory/team-a/apps.part-0001-of-0002.json",
		manifest:   "inventory/team-a/apps.part-0001-of-0002.manifest.json",
	},
	{label: "L(cluster/platform)", objectPath: "inventory/cluster/platform.json", manifest: "inventory/cluster/platform.manifest.json", legacy: true},
}

type fuzzResource struct {
	namespace, kind, name string
	path                  string // where the default per-resource layout puts it
}

// Same name across kinds and namespaces, cluster scope, and multipart-shaped literal names.
var fuzzResources = []fuzzResource{
	{"team-a", "Deployment", "api", "default/team-a/deployment/api.yaml"},
	{"team-a", "Service", "api", "default/team-a/service/api.yaml"},
	{"team-b", "Deployment", "api", "default/team-b/deployment/api.yaml"},
	{"team-a", "Deployment", "web", "default/team-a/deployment/web.yaml"},
	{"team-a", "ConfigMap", "apps.part-0001-of-0002", "default/team-a/configmap/apps.part-0001-of-0002.yaml"},
	{"", "ClusterRole", "admin", "default/clusterrole/admin.yaml"},
	{"team-a", "Deployment", "db", "default/team-a/deployment/db.yaml"},
	{"team-b", "ConfigMap", "web.part-0002-of-0002", "default/team-b/configmap/web.part-0002-of-0002.yaml"},
}

// Files Kollect never recorded: a neighbour, a hand edit on a resource path, a file squatting on a
// set-manifest path.
var unknownPaths = []string{
	"custom/notes.yaml",
	"default/team-a/deployment/api.yaml",
	"inventory/team-a/apps.manifest.json",
	"default/team-b/deployment/legacy.yaml",
}

const (
	opSingle = iota
	opMultipart
	opUnknown
	opReplay
	opRetryFinal
	opCount

	maxSteps = 10
)

// --- reference model ---------------------------------------------------------------------------

// ownedModel is the oracle. present is the repo file set; recordedBy maps a path to the owner
// whose last COMPLETE export claimed it. Paths written by a suppressed or interrupted call stay
// unrecorded, exactly like files Kollect never wrote.
type ownedModel struct {
	present    map[string]bool
	recordedBy map[string]int
}

type exportCall struct {
	owner    int
	writes   []string // files this call writes
	keep     []string // the paths this call claims (multipart final: the whole set)
	complete bool     // single-part or final part of a set
}

// predict returns whether the call must be rejected and, if not, which paths it must delete.
func (m *ownedModel) predict(c exportCall) (reject bool, deletes map[string]bool) {
	for _, p := range c.keep {
		if o, ok := m.recordedBy[p]; ok && o != c.owner {
			return true, nil
		}
	}
	deletes = map[string]bool{}
	if !c.complete || fuzzOwners[c.owner].legacy {
		return false, deletes
	}
	keep := toSet(c.keep)
	for p, o := range m.recordedBy {
		if o == c.owner && !keep[p] {
			deletes[p] = true
		}
	}

	return false, deletes
}

func (m *ownedModel) apply(c exportCall, deletes map[string]bool) {
	for p := range deletes {
		delete(m.present, p)
		delete(m.recordedBy, p)
	}
	for _, p := range c.writes {
		m.present[p] = true
	}
	if c.complete && !fuzzOwners[c.owner].legacy {
		for _, p := range c.keep {
			m.recordedBy[p] = c.owner
		}
	}
}

// --- harness -----------------------------------------------------------------------------------

type exportOp struct {
	kind   int
	owner  int
	masks  []byte
	cutAt  int // parts actually sent; == len(masks) when complete
	plan   *sink.PrunePlan
	single bool
}

type ownedPruneHarness struct {
	t          *testing.T
	fs         billy.Filesystem
	registry   *sink.Registry
	model      *ownedModel
	history    []*exportOp
	generation int64
	step       int
}

type memBackend struct {
	*git.Backend
	fs billy.Filesystem
}

func (b *memBackend) Export(context.Context, []byte, string) error {
	return errors.New("fuzz: unexpected single-document export for a perResource git layout")
}

func (b *memBackend) ExportFiles(_ context.Context, files []git.FileEntry, opts git.ExportFilesOptions) error {
	return b.ExportFilesToFilesystemForTest(b.fs, files, opts)
}

func ownedPruneSpec() kollectdevv1alpha1.KollectSinkSpec {
	return kollectdevv1alpha1.KollectSinkSpec{
		Type:     kollectdevv1alpha1.SinkTypeGit,
		Endpoint: "file:///nonexistent/kollect-owned-prune-fuzz.git",
		Layout:   &kollectdevv1alpha1.LayoutSpec{Mode: kollectdevv1alpha1.LayoutModePerResource},
	}
}

func newOwnedPruneHarness(t *testing.T) *ownedPruneHarness {
	t.Helper()

	fs := memfs.New()
	gb, err := git.NewBackend(ownedPruneSpec(), nil, git.Auth{}, nil)
	if err != nil {
		t.Fatalf("build git backend: %v", err)
	}
	backend := &memBackend{Backend: gb, fs: fs}
	registry := sink.NewRegistry()
	registry.Register(git.TypeName, func(kollectdevv1alpha1.KollectSinkSpec, sink.BuildContext) (sink.Backend, error) {
		return backend, nil
	})

	return &ownedPruneHarness{
		t:        t,
		fs:       fs,
		registry: registry,
		model:    &ownedModel{present: map[string]bool{}, recordedBy: map[string]int{}},
	}
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) done() bool { return r.pos >= len(r.data) }

func (r *byteReader) next() int {
	if r.done() {
		return 0
	}
	b := r.data[r.pos]
	r.pos++

	return int(b)
}

func (r *byteReader) nextByte() byte {
	if r.done() {
		return 0
	}
	b := r.data[r.pos]
	r.pos++

	return b
}

func (h *ownedPruneHarness) run(program []byte) {
	r := &byteReader{data: program}
	for steps := 0; steps < maxSteps && !r.done(); steps++ {
		switch r.next() % opCount {
		case opSingle:
			op := &exportOp{kind: opSingle, owner: r.next() % len(fuzzOwners), masks: []byte{r.nextByte()}, single: true}
			op.cutAt = 1
			h.history = append(h.history, op)
			h.runExport(op)
		case opMultipart:
			owner := r.next() % len(fuzzOwners)
			n := 2 + r.next()%2
			masks := make([]byte, n)
			for i := range masks {
				// Partitioning never emits an empty part.
				if masks[i] = r.nextByte(); masks[i] == 0 {
					masks[i] = 1 << (i % len(fuzzResources))
				}
			}
			cut := r.next()
			cutAt := n
			if cut%3 == 0 {
				cutAt = 1 + (cut/3)%(n-1)
			}
			op := &exportOp{kind: opMultipart, owner: owner, masks: masks, cutAt: cutAt}
			h.history = append(h.history, op)
			h.runExport(op)
		case opUnknown:
			h.writeUnknown(unknownPaths[r.next()%len(unknownPaths)])
		case opReplay:
			back := r.next()
			if len(h.history) == 0 {
				continue
			}
			prev := h.history[len(h.history)-1-back%len(h.history)]
			replay := &exportOp{kind: prev.kind, owner: prev.owner, masks: prev.masks, cutAt: prev.cutAt, single: prev.single}
			h.history = append(h.history, replay)
			h.runExport(replay)
		case opRetryFinal:
			if len(h.history) == 0 {
				continue
			}
			last := h.history[len(h.history)-1]
			if last.single || last.cutAt != len(last.masks) || last.plan == nil {
				continue
			}
			h.sendPart(last, len(last.masks))
		}
	}
}

func (h *ownedPruneHarness) runExport(op *exportOp) {
	if op.single {
		h.sendSingle(op)

		return
	}
	op.plan = sink.NewPrunePlan()
	for i := 1; i <= op.cutAt; i++ {
		if !h.sendPart(op, i) {
			// A rejected part ends the set; it is now interrupted.
			op.cutAt = i - 1

			return
		}
	}
}

func (h *ownedPruneHarness) sendSingle(op *exportOp) {
	paths := maskPaths(op.masks[0])
	call := exportCall{owner: op.owner, writes: paths, keep: paths, complete: true}
	h.check(call, fmt.Sprintf("single-part export by %s of %v", fuzzOwners[op.owner].label, paths), "", func() error {
		return h.export(fuzzOwners[op.owner].objectPath, maskItems(op.masks[0]), export.Metadata{}, nil)
	})
}

// sendPart sends part index (1-based) of op's set and reports whether it was accepted.
func (h *ownedPruneHarness) sendPart(op *exportOp, index int) bool {
	total := len(op.masks)
	owner := fuzzOwners[op.owner]
	paths := maskPaths(op.masks[index-1])
	call := exportCall{owner: op.owner, writes: paths, keep: paths, complete: index == total}
	if call.complete {
		union := map[string]bool{}
		for _, m := range op.masks {
			for _, p := range maskPaths(m) {
				union[p] = true
			}
		}
		union[owner.manifest] = true
		call.keep = sortedKeys(union)
		call.writes = append(append([]string(nil), paths...), owner.manifest)
	}
	partial := ""
	if !call.complete {
		partial = fmt.Sprintf("part %d/%d", index, total)
	}
	desc := fmt.Sprintf("multipart export by %s part %d/%d of %v", owner.label, index, total, paths)

	accepted := true
	h.check(call, desc, partial, func() error {
		err := h.export(export.PartitionObjectPath(owner.objectPath, index, total), maskItems(op.masks[index-1]),
			export.Metadata{PartIndex: index, PartTotal: total}, op.plan)
		accepted = err == nil

		return err
	})

	return accepted
}

func (h *ownedPruneHarness) export(objectPath string, items []collect.Item, meta export.Metadata, plan *sink.PrunePlan) error {
	h.generation++
	meta.Generation = h.generation
	// Rejected collisions are expected outcomes here; five in a row would trip the per-sink
	// breaker and turn every later step into a "breaker open" error the model does not describe.
	sink.ResetBreakersForTest()
	meta.ExportedAt = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	envelope, err := export.MarshalEnvelope(items, meta)
	if err != nil {
		h.t.Fatalf("marshal envelope: %v", err)
	}
	_, err = sink.RunExportEnvelope(sink.ExportEnvelopeRequest{
		Ctx:           context.Background(),
		Registry:      h.registry,
		SinkNamespace: "default",
		SinkName:      "owned-prune-fuzz",
		ObjectPath:    objectPath,
		Envelope:      envelope,
		SinkSpec:      ownedPruneSpec(),
		PrunePlan:     plan,
	})

	return err
}

func (h *ownedPruneHarness) writeUnknown(p string) {
	h.step++
	if err := h.fs.MkdirAll(path.Dir(p), 0o750); err != nil {
		h.t.Fatalf("mkdir unknown file: %v", err)
	}
	if err := util.WriteFile(h.fs, p, []byte("hand-written\n"), 0o600); err != nil {
		h.t.Fatalf("write unknown file: %v", err)
	}
	h.model.present[p] = true
}

// check runs one export call and asserts every invariant against the model before advancing it.
func (h *ownedPruneHarness) check(c exportCall, desc, partial string, run func() error) {
	t := h.t
	t.Helper()
	h.step++
	actor := fuzzOwners[c.owner]
	where := fmt.Sprintf("step %d, %s", h.step, desc)

	before := h.repoFiles()
	reject, wantDeletes := h.model.predict(c)
	err := run()
	after := h.repoFiles()

	deleted := map[string]bool{}
	for p := range before {
		if !after[p] {
			deleted[p] = true
		}
	}
	if partial != "" && len(deleted) > 0 {
		t.Fatalf("FORBIDDEN (invariant 4): %s is an incomplete multipart set (%s) yet deleted %v", where, partial, sortedKeys(deleted))
	}
	if actor.legacy && len(deleted) > 0 {
		t.Fatalf("FORBIDDEN (invariant 5): %s uses the ambiguous legacy cluster identity yet deleted %v", where, sortedKeys(deleted))
	}
	for _, p := range sortedKeys(deleted) {
		o, recorded := h.model.recordedBy[p]
		if !recorded {
			t.Fatalf("FORBIDDEN (invariant 2): %s deleted %q, a file no ownership record claims", where, p)
		}
		if o != c.owner {
			t.Fatalf("FORBIDDEN (invariant 1): %s deleted %q, which belongs to %s", where, p, fuzzOwners[o].label)
		}
	}

	switch {
	case reject && err == nil:
		t.Fatalf("FORBIDDEN (invariant 1): %s adopted a path another inventory owns instead of rejecting the export", where)
	case !reject && err != nil:
		t.Fatalf("model predicts %s succeeds, got error: %v", where, err)
	case reject:
		if len(deleted) > 0 || !sameSet(before, after) {
			t.Fatalf("FORBIDDEN: rejected %s still changed the repo: before %v after %v", where, sortedKeys(before), sortedKeys(after))
		}

		return
	}

	if partial == "" && !actor.legacy && !sameSet(deleted, wantDeletes) {
		t.Fatalf("FORBIDDEN (invariant 3): complete %s must delete exactly %v (previously owned minus current set), deleted %v",
			where, sortedKeys(wantDeletes), sortedKeys(deleted))
	}

	h.model.apply(c, wantDeletes)
	if !sameSet(after, h.model.present) {
		t.Fatalf("FORBIDDEN (invariant 6): after %s the repo diverges from the model:\n model %v\n repo  %v",
			where, sortedKeys(h.model.present), sortedKeys(after))
	}
}

// repoFiles lists every regular file in the worktree except Kollect's own ownership records.
func (h *ownedPruneHarness) repoFiles() map[string]bool {
	out := map[string]bool{}
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := h.fs.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			h.t.Fatalf("list %q: %v", dir, err)
		}
		for _, e := range entries {
			p := e.Name()
			if dir != "" {
				p = dir + "/" + e.Name()
			}
			if p == ".kollect-prune" {
				continue
			}
			if e.IsDir() {
				walk(p)
				continue
			}
			out[p] = true
		}
	}
	walk("")

	return out
}

// --- helpers -----------------------------------------------------------------------------------

func maskItems(mask byte) []collect.Item {
	var items []collect.Item
	for i, r := range fuzzResources {
		if mask&(1<<i) == 0 {
			continue
		}
		metadata := map[string]any{"name": r.name}
		if r.namespace != "" {
			metadata["namespace"] = r.namespace
		}
		items = append(items, collect.Item{
			Namespace: r.namespace, Name: r.name, Kind: r.kind, Version: "v1",
			UID: fmt.Sprintf("uid-%d", i),
			Attributes: map[string]any{"payload": map[string]any{
				"apiVersion": "v1", "kind": r.kind, "metadata": metadata,
			}},
		})
	}

	return items
}

func maskPaths(mask byte) []string {
	var out []string
	for i, r := range fuzzResources {
		if mask&(1<<i) != 0 {
			out = append(out, r.path)
		}
	}
	sort.Strings(out)

	return out
}

func toSet(paths []string) map[string]bool {
	out := make(map[string]bool, len(paths))
	for _, p := range paths {
		out[p] = true
	}

	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)

	return out
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}

	return true
}

// --- seeds -------------------------------------------------------------------------------------

// Resource bits.
const (
	rDeployAPI  = 1 << 0
	rServiceAPI = 1 << 1
	rTeamBAPI   = 1 << 2
	rWeb        = 1 << 3
	rPartShaped = 1 << 4
	rCluster    = 1 << 5
	rDB         = 1 << 6
	rPartShapeB = 1 << 7
)

// Owners.
const (
	oA = 0
	oB = 1
	oC = 2
	oL = 3
)

func single(owner, mask byte) []byte { return []byte{opSingle, owner, mask} }

// multi encodes a set; interrupted keeps only the first `sent` parts (sent < len(masks)).
func multi(owner byte, masks []byte, sent int) []byte {
	extra := byte(0) // 2 parts
	if len(masks) == 3 {
		extra = 1
	}
	out := append([]byte{opMultipart, owner, extra}, masks...)
	cut := byte(1) // complete
	if sent < len(masks) {
		// cut%3 == 0 interrupts; (cut/3)%(n-1) == sent-1 picks where.
		cut = 0
		for i := 1; i < sent; i++ {
			cut += 3
		}
	}

	return append(out, cut)
}

func prog(steps ...[]byte) []byte {
	var out []byte
	for _, s := range steps {
		out = append(out, s...)
	}

	return out
}

func ownedPruneSeeds() [][]byte {
	return [][]byte{
		// Invariant 1: B's drop and empty export never touch A's files; B claiming A's path is rejected.
		prog(single(oA, rDeployAPI|rWeb), single(oB, rTeamBAPI|rServiceAPI), single(oB, 0), single(oB, rDeployAPI)),
		// Invariant 1, multipart-shaped identity: C (literal suffix-shaped name) and A's multipart set
		// share an object path but not an owner.
		prog(single(oC, rPartShaped|rDB), multi(oA, []byte{rDeployAPI, rWeb}, 2), single(oC, 0), multi(oA, []byte{rWeb, rServiceAPI}, 2)),
		// Invariant 2: unknown neighbours and an unknown file on a manifest path survive owned prunes.
		prog([]byte{opUnknown, 0}, []byte{opUnknown, 3}, single(oA, rDeployAPI|rCluster), single(oA, 0), []byte{opUnknown, 2}, multi(oA, []byte{rDB, rWeb}, 2), single(oA, rDB)),
		// Invariant 3: a complete export deletes exactly previous-minus-current.
		prog(single(oA, rDeployAPI|rServiceAPI|rWeb|rCluster), single(oA, rServiceAPI|rDB), multi(oA, []byte{rDB, rPartShapeB, rWeb}, 3), single(oA, rPartShaped)),
		// Invariant 4: an interrupted multipart set (1 of 2, then 2 of 3) deletes nothing.
		prog(single(oA, rDeployAPI|rWeb|rDB), multi(oA, []byte{rServiceAPI, rCluster}, 1), multi(oA, []byte{rServiceAPI, rCluster, rPartShaped}, 2), single(oA, rDeployAPI)),
		// Invariant 5: the legacy cluster identity never prunes, not even on an empty or shrinking set.
		prog(single(oL, rDeployAPI|rWeb), single(oL, rDB), single(oL, 0), multi(oL, []byte{rServiceAPI, rCluster}, 2), single(oA, rDeployAPI), single(oL, rDeployAPI)),
		// Invariant 6: A -> B -> A, then replays and a retried final part.
		prog(single(oA, rDeployAPI|rWeb), single(oB, rTeamBAPI|rPartShapeB), []byte{opReplay, 1}, []byte{opReplay, 1}, single(oA, rWeb), multi(oB, []byte{rTeamBAPI, rDB}, 2), []byte{opRetryFinal}, []byte{opReplay, 2}),
	}
}
