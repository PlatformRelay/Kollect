// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithRepoExportLock_serializesConcurrentExports(t *testing.T) {
	t.Parallel()

	var concurrent int32
	var maxConcurrent int32

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_ = withRepoExportLock("https://git.example/repo", "main", func() error {
				cur := atomic.AddInt32(&concurrent, 1)
				for {
					peak := atomic.LoadInt32(&maxConcurrent)
					if cur > peak {
						if atomic.CompareAndSwapInt32(&maxConcurrent, peak, cur) {
							break
						}
						continue
					}
					break
				}

				atomic.AddInt32(&concurrent, -1)

				return nil
			})
		}()
	}

	wg.Wait()

	if maxConcurrent > 1 {
		t.Fatalf("max concurrent = %d, want 1", maxConcurrent)
	}
}

// MR-08: every inventory of a branchMR sink shares one warm mirror keyed by
// (clone URL, clone branch). Operations on DIFFERENT push branches must
// therefore serialize on that mirror lock: while it is held, exports to two
// other feature branches must both wait, and both must complete once it is
// released.
func TestMirrorLock_serializesDifferentPushBranchesOnOneMirror(t *testing.T) {
	skipWithoutGit(t)

	remote := createBareRemoteWithMainCommit(t)
	cfg := Config{Endpoint: "file://" + remote, Engine: GitEngineCLI}.withDefaults()

	cloneURL, _, err := parseRemote(cfg.Endpoint)
	if err != nil {
		t.Fatalf("parseRemote: %v", err)
	}

	held := make(chan struct{})
	release := make(chan struct{})
	holderDone := make(chan struct{})
	go func() {
		defer close(holderDone)
		_ = withRepoExportLock(cloneURL, "main", func() error {
			close(held)
			<-release

			return nil
		})
	}()
	<-held

	ops := map[string]chan error{}
	for _, team := range []string{"team-a", "team-b"} {
		done := make(chan error, 1)
		ops["export "+team] = done
		go func() {
			path := "inventory/" + team + "/inv.json"
			done <- ExportFilesWithBranch(t.Context(), cfg, Auth{}, []FileEntry{
				{Path: path, Data: []byte(`{"items":[]}`)},
			}, &BranchSpec{PushBranch: "kollect/" + team + "/inv", CloneBranch: "main"},
				CommitContextFromObjectPath(path, "prod"))
		}()
	}

	select {
	case <-time.After(300 * time.Millisecond):
	case <-firstDone(ops):
		t.Fatal("an operation on another push branch ran while the mirror lock was held")
	}

	close(release)
	<-holderDone

	for name, ch := range ops {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("%s after release: %v", name, err)
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("%s did not complete after the mirror lock was released", name)
		}
	}
}

// firstDone fires when any operation finishes, putting its result back so the
// caller can still read it.
func firstDone(ops map[string]chan error) <-chan struct{} {
	fired := make(chan struct{})
	var once sync.Once
	for _, ch := range ops {
		go func() {
			err := <-ch
			ch <- err
			once.Do(func() { close(fired) })
		}()
	}

	return fired
}
