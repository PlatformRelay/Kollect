// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/platformrelay/kollect/internal/validation"
)

// AllowSecretExtractionAnnotation is the Profile annotation that opts into Secret.data paths.
// Re-exported so init YAML generation stays aligned with admission validation.
const AllowSecretExtractionAnnotation = validation.AllowSecretExtractionAnnotation

// InitSampleRef identifies one candidate object for consented attribute sampling.
type InitSampleRef struct {
	Group     string
	Version   string
	Kind      string
	Resource  string
	Namespace string
	Name      string
}

// Label returns a select-option label for this sample candidate.
func (r InitSampleRef) Label() string {
	gvk := InitResourceInfo{Group: r.Group, Version: r.Version, Kind: r.Kind}.Label()
	if r.Namespace == "" {
		return fmt.Sprintf("%s — %s", gvk, r.Name)
	}
	return fmt.Sprintf("%s — %s/%s", gvk, r.Namespace, r.Name)
}

// Identity returns a human-readable GVK/namespace/name line shown before reading.
func (r InitSampleRef) Identity() string {
	gvk := InitResourceInfo{Group: r.Group, Version: r.Version, Kind: r.Kind}.Label()
	if r.Namespace == "" {
		return fmt.Sprintf("%s name=%s", gvk, r.Name)
	}
	return fmt.Sprintf("%s namespace=%s name=%s", gvk, r.Namespace, r.Name)
}

// InitSampler lists and reads representative objects for attribute suggestions.
// Discovery alone must not read object contents — callers gate GetSampleObject
// behind an explicit consent prompt that shows Identity() first (ADR-0802 §6).
type InitSampler interface {
	ListSampleCandidates(
		ctx context.Context, res InitResourceInfo, namespaces []string, limit int,
	) ([]InitSampleRef, error)
	GetSampleObject(ctx context.Context, ref InitSampleRef) (map[string]any, error)
}

// InitSensitiveKind names a GVK denied for sampling without a distinct confirmation.
type InitSensitiveKind struct {
	Group string
	Kind  string
}

// DefaultInitSensitiveKinds are denied for representative-object sampling unless the
// operator confirms a distinct sensitive-kind guard (Secret is always included).
func DefaultInitSensitiveKinds() []InitSensitiveKind {
	return []InitSensitiveKind{{Group: "", Kind: "Secret"}}
}

func isSensitiveInitKind(res InitResourceInfo, extra []InitSensitiveKind) bool {
	kinds := DefaultInitSensitiveKinds()
	kinds = append(kinds, extra...)
	for _, k := range kinds {
		if !strings.EqualFold(k.Kind, res.Kind) {
			continue
		}
		kg := k.Group
		rg := res.Group
		if kg == "core" {
			kg = ""
		}
		if rg == "core" {
			rg = ""
		}
		if strings.EqualFold(kg, rg) {
			return true
		}
	}
	return false
}

const (
	initSampleLimit              = 20
	initSampleConsentPrompt      = "Sample a representative object for additional attribute suggestions?"
	initSensitiveGuardPrompt     = "This kind is sensitive (e.g. Secret). Confirm a distinct sampling guard?"
	initSampleIdentityPromptPref = "Read object"
)

// KubeInitSampler uses the caller's kubeconfig RBAC to list/get objects for sampling.
type KubeInitSampler struct {
	dyn dynamic.Interface
}

// NewKubeInitSampler builds an InitSampler from kubeconfig path + optional context.
func NewKubeInitSampler(kubeconfig, contextName string) (*KubeInitSampler, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build rest config for sampler: %w", err)
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("build dynamic client: %w", err)
	}
	return &KubeInitSampler{dyn: dyn}, nil
}

// ListSampleCandidates implements InitSampler.
func (s *KubeInitSampler) ListSampleCandidates(
	ctx context.Context, res InitResourceInfo, namespaces []string, limit int,
) ([]InitSampleRef, error) {
	if limit <= 0 {
		limit = initSampleLimit
	}
	gvr := schema.GroupVersionResource{
		Group: res.Group, Version: res.Version, Resource: res.Resource,
	}

	var out []InitSampleRef
	if !res.Namespaced {
		list, err := s.dyn.Resource(gvr).List(ctx, metav1.ListOptions{Limit: int64(limit)})
		if err != nil {
			return nil, fmt.Errorf("list %s for sampling: %w", res.Label(), err)
		}
		for _, item := range list.Items {
			out = append(out, sampleRefFromUnstructured(res, &item))
			if len(out) >= limit {
				break
			}
		}
		return out, nil
	}

	nsList := namespaces
	if len(nsList) == 0 {
		// Empty scope = all accessible; leave namespace unset and use cluster-wide list
		// when the API allows it; otherwise the caller should pass namespaces.
		list, err := s.dyn.Resource(gvr).Namespace(metav1.NamespaceAll).
			List(ctx, metav1.ListOptions{Limit: int64(limit)})
		if err != nil {
			return nil, fmt.Errorf("list %s for sampling: %w", res.Label(), err)
		}
		for _, item := range list.Items {
			out = append(out, sampleRefFromUnstructured(res, &item))
			if len(out) >= limit {
				break
			}
		}
		return out, nil
	}

	for _, ns := range nsList {
		if len(out) >= limit {
			break
		}
		remain := int64(limit - len(out))
		list, err := s.dyn.Resource(gvr).Namespace(ns).
			List(ctx, metav1.ListOptions{Limit: remain})
		if err != nil {
			return nil, fmt.Errorf("list %s in namespace %q for sampling: %w", res.Label(), ns, err)
		}
		for _, item := range list.Items {
			out = append(out, sampleRefFromUnstructured(res, &item))
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// GetSampleObject implements InitSampler.
func (s *KubeInitSampler) GetSampleObject(ctx context.Context, ref InitSampleRef) (map[string]any, error) {
	gvr := schema.GroupVersionResource{
		Group: ref.Group, Version: ref.Version, Resource: ref.Resource,
	}
	var (
		obj *unstructured.Unstructured
		err error
	)
	if ref.Namespace == "" {
		obj, err = s.dyn.Resource(gvr).Get(ctx, ref.Name, metav1.GetOptions{})
	} else {
		obj, err = s.dyn.Resource(gvr).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("get sample %s: %w", ref.Identity(), err)
	}
	return obj.Object, nil
}

func sampleRefFromUnstructured(res InitResourceInfo, item *unstructured.Unstructured) InitSampleRef {
	return InitSampleRef{
		Group:     res.Group,
		Version:   res.Version,
		Kind:      res.Kind,
		Resource:  res.Resource,
		Namespace: item.GetNamespace(),
		Name:      item.GetName(),
	}
}

// suggestAttributesFromSample walks a representative object into attribute options.
// When sensitive is true, .data / .stringData values are never included in preview
// lines or option names — only key paths are offered.
func suggestAttributesFromSample(obj map[string]any, sensitive bool) (opts []initAttributeOpt, preview []string) {
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch t := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if skipSampleField(k, prefix) {
					continue
				}
				child := joinSamplePath(prefix, k)
				if isSecretDataMap(prefix, k) {
					// Offer each data key path; never print or descend into values.
					if m, ok := t[k].(map[string]any); ok {
						subKeys := make([]string, 0, len(m))
						for sk := range m {
							subKeys = append(subKeys, sk)
						}
						sort.Strings(subKeys)
						for _, sk := range subKeys {
							path := joinSamplePath(child, sk)
							opts = append(opts, initAttributeOpt{
								Name: sampleAttrName(path),
								Path: "$." + path,
								Type: "string",
							})
							preview = append(preview, path+"  (key only; value hidden)")
						}
					}
					continue
				}
				walk(child, t[k])
			}
		case []any:
			if len(t) == 0 {
				return
			}
			// Sample the first element only for suggestion shape.
			walk(prefix+"[0]", t[0])
		default:
			if prefix == "" {
				return
			}
			name := sampleAttrName(prefix)
			typ := sampleAttrType(t)
			opts = append(opts, initAttributeOpt{Name: name, Path: "$." + prefix, Type: typ})
			preview = append(preview, fmt.Sprintf("%s = %s", prefix, samplePreviewValue(t, sensitive)))
		}
	}
	walk("", obj)

	// Cap suggestion volume for the MultiSelect UI.
	const maxSuggest = 40
	if len(opts) > maxSuggest {
		opts = opts[:maxSuggest]
		preview = preview[:maxSuggest]
	}
	return opts, preview
}

func skipSampleField(key, prefix string) bool {
	switch key {
	case "managedFields", "resourceVersion", "uid", "generation",
		"creationTimestamp", "selfLink", "ownerReferences", "finalizers":
		return true
	}
	_ = prefix
	return false
}

func isSecretDataMap(prefix, key string) bool {
	if key != "data" && key != "stringData" {
		return false
	}
	// Secret puts data at the object root; treat nested data maps conservatively too.
	return prefix == "" || prefix == "spec"
}

func joinSamplePath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func sampleAttrName(path string) string {
	path = strings.ReplaceAll(path, "[0]", "")
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return "field"
	}
	name := parts[len(parts)-1]
	if name == "" {
		return "field"
	}
	// Prefer dotted names for nested secret keys (data.password).
	if len(parts) >= 2 && (parts[0] == "data" || parts[0] == "stringData") {
		return strings.Join(parts, ".")
	}
	if len(parts) >= 2 && parts[0] == "spec" {
		return strings.Join(parts[1:], ".")
	}
	if len(parts) >= 2 && parts[0] == "metadata" {
		return parts[len(parts)-1]
	}
	return name
}

func sampleAttrType(v any) string {
	switch v.(type) {
	case bool:
		return "boolean"
	case float64, int, int32, int64:
		return "number"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return "string"
	}
}

func samplePreviewValue(v any, sensitive bool) string {
	if sensitive {
		return "(redacted)"
	}
	switch t := v.(type) {
	case string:
		if len(t) > 48 {
			return fmt.Sprintf("%q…", t[:45])
		}
		return fmt.Sprintf("%q", t)
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("%T", t)
	}
}

func mergeInitAttributeOpts(base, extra []initAttributeOpt) []initAttributeOpt {
	seen := make(map[string]struct{}, len(base)+len(extra))
	var out []initAttributeOpt
	for _, o := range base {
		if _, ok := seen[o.Name]; ok {
			continue
		}
		seen[o.Name] = struct{}{}
		out = append(out, o)
	}
	for _, o := range extra {
		if _, ok := seen[o.Name]; ok {
			continue
		}
		seen[o.Name] = struct{}{}
		out = append(out, o)
	}
	return out
}

func samplingNamespaces(intent *initDraft) []string {
	if len(intent.IncludedNamespaces) > 0 {
		return append([]string(nil), intent.IncludedNamespaces...)
	}
	return nil
}
