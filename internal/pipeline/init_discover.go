// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeInitDiscoverer uses the caller's kubeconfig RBAC for API and namespace discovery.
type KubeInitDiscoverer struct {
	discovery discovery.DiscoveryInterface
	client    kubernetes.Interface
}

// NewKubeInitDiscoverer builds an InitDiscoverer from a kubeconfig path and optional context.
func NewKubeInitDiscoverer(kubeconfig, contextName string) (*KubeInitDiscoverer, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build rest config: %w", err)
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("build kubernetes client: %w", err)
	}
	return &KubeInitDiscoverer{discovery: client.Discovery(), client: client}, nil
}

// ListResources implements InitDiscoverer.
func (d *KubeInitDiscoverer) ListResources(ctx context.Context) ([]InitResourceInfo, error) {
	_, lists, err := d.discovery.ServerGroupsAndResources()
	if err != nil && len(lists) == 0 {
		return nil, fmt.Errorf("server groups/resources: %w", err)
	}
	_ = ctx

	seen := map[string]struct{}{}
	var out []InitResourceInfo
	for _, list := range lists {
		if list == nil {
			continue
		}
		group, version := splitInitGV(list.GroupVersion)
		for _, r := range list.APIResources {
			if initResourceHasSlash(r.Name) {
				continue
			}
			key := group + "/" + version + "/" + r.Kind
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, InitResourceInfo{
				Group:      group,
				Version:    version,
				Kind:       r.Kind,
				Resource:   r.Name,
				Namespaced: r.Namespaced,
				Verbs:      append([]string(nil), r.Verbs...),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Label() < out[j].Label()
	})
	return out, nil
}

// ListNamespaces implements InitDiscoverer.
func (d *KubeInitDiscoverer) ListNamespaces(ctx context.Context) ([]string, error) {
	list, err := d.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces (RBAC?): %w", err)
	}
	names := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		names = append(names, ns.Name)
	}
	sort.Strings(names)
	return names, nil
}

func splitInitGV(gv string) (group, version string) {
	for i := 0; i < len(gv); i++ {
		if gv[i] == '/' {
			return gv[:i], gv[i+1:]
		}
	}
	return "", gv
}

func initResourceHasSlash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return true
		}
	}
	return false
}
