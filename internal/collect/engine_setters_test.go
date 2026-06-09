// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"
)

func TestEngineNamespaceDefaultsAndSnapshot(t *testing.T) {
	t.Parallel()

	e, err := NewEngine(nil, nil, NewStore(), EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	defaults := NamespaceDefaults{
		Included: []string{"team-a"},
		Excluded: []string{"kube-system"},
	}
	e.SetNamespaceDefaults(defaults)

	got := e.NamespaceDefaultsSnapshot()
	if len(got.Included) != 1 || got.Included[0] != "team-a" {
		t.Fatalf("NamespaceDefaultsSnapshot included = %#v", got.Included)
	}
	if len(got.Excluded) != 1 || got.Excluded[0] != "kube-system" {
		t.Fatalf("NamespaceDefaultsSnapshot excluded = %#v", got.Excluded)
	}
}

func TestEngineSetScrubKeysAndBindClusterTargetNamespaces(t *testing.T) {
	t.Parallel()

	e, err := NewEngine(nil, nil, NewStore(), EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	e.SetScrubKeys([]string{"password", "token"})
	e.mu.RLock()
	if len(e.scrubKeys) != 2 || e.scrubKeys[0] != "password" || e.scrubKeys[1] != "token" {
		e.mu.RUnlock()
		t.Fatalf("scrubKeys = %#v", e.scrubKeys)
	}
	e.mu.RUnlock()

	e.BindClusterTargetNamespaces("shared-target", []string{"team-b", "team-a"})
	got := e.NamespacesForClusterTarget("shared-target")
	if len(got) != 2 || got[0] != "team-a" || got[1] != "team-b" {
		t.Fatalf("NamespacesForClusterTarget = %#v, want sorted team-a/team-b", got)
	}
}

func TestEngineNamespaceMetaSnapshot(t *testing.T) {
	t.Parallel()

	e, err := NewEngine(nil, nil, NewStore(), EngineConfig{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	e.nsMu.Lock()
	e.nsMeta["team-a"] = namespaceMeta{
		Labels:      labels.Set{"team": "a"},
		Annotations: map[string]string{"owner": "platform"},
	}
	e.nsMu.Unlock()

	got := e.NamespaceMetaSnapshot()
	meta, ok := got["team-a"]
	if !ok {
		t.Fatal("NamespaceMetaSnapshot missing team-a")
	}
	if meta.Labels["team"] != "a" || meta.Annotations["owner"] != "platform" {
		t.Fatalf("NamespaceMetaSnapshot[team-a] = %#v", meta)
	}
}
