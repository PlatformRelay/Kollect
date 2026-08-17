// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// NamespaceDefaults are optional Helm-provided include/exclude lists. CRD fields take precedence.
type NamespaceDefaults struct {
	Included []string
	Excluded []string
}

// ScopeCeiling is the resolved tenancy guardrail for namespace checks at collect
// time. GVKs are not filtered here: they are enforced at admission and, as
// backstop, at reconcile (ScopeGVKDenied).
type ScopeCeiling struct {
	AllowedNamespaces []string
	DeniedNamespaces  []string
}

// CompiledResourceRule holds a parsed resource rule ready for runtime matching.
type CompiledResourceRule struct {
	Rule            kollectdevv1alpha1.ResourceRule
	GVR             schema.GroupVersionResource
	NamespaceSel    labels.Selector
	ResourceSel     labels.Selector
	MatchPolicyProg cel.Program
}

// MatchIntentNamespaces returns workload namespaces matched by Target intent before Scope ceiling.
func MatchIntentNamespaces(
	filter kollectdevv1alpha1.CollectionFilterSpec,
	namespaceSelector *metav1.LabelSelector,
	nsMeta map[string]NamespaceMeta,
	defaults NamespaceDefaults,
) []string {
	included := filter.IncludedNamespaces
	if len(included) == 0 {
		included = defaults.Included
	}

	excluded := unionStrings(filter.ExcludedNamespaces, defaults.Excluded)
	excludeSel := filter.NamespaceExcludeSelector

	internalMeta := namespaceMetaMapFromFilter(nsMeta)

	var matched []string
	for nsName, meta := range internalMeta {
		if len(included) > 0 && !containsString(included, nsName) {
			continue
		}

		if !namespaceMatchesSelector(namespaceSelector, meta.Labels) {
			continue
		}

		if containsString(excluded, nsName) {
			continue
		}

		if excludeSel != nil && namespaceMatchesSelector(excludeSel, meta.Labels) {
			continue
		}

		matched = append(matched, nsName)
	}

	return sortedUniqueStrings(matched)
}

// EffectiveNamespaces applies Scope ceiling and deny lists to matched intent namespaces.
func EffectiveNamespaces(
	matched []string,
	ceiling ScopeCeiling,
	filter kollectdevv1alpha1.CollectionFilterSpec,
	defaults NamespaceDefaults,
) []string {
	excluded := unionStrings(filter.ExcludedNamespaces, defaults.Excluded)
	excluded = unionStrings(excluded, ceiling.DeniedNamespaces)

	allowed := ceiling.AllowedNamespaces
	effective := make([]string, 0, len(matched))

	for _, ns := range matched {
		if containsString(excluded, ns) {
			continue
		}

		if len(allowed) > 0 && !containsString(allowed, ns) {
			continue
		}

		effective = append(effective, ns)
	}

	return sortedUniqueStrings(effective)
}

// CompileResourceRules validates and compiles resource rules for runtime matching.
func CompileResourceRules(rules []kollectdevv1alpha1.ResourceRule, celEnv *cel.Env) ([]CompiledResourceRule, error) {
	if len(rules) == 0 {
		return nil, nil
	}

	compiled := make([]CompiledResourceRule, 0, len(rules))
	for i, rule := range rules {
		if rule.GVK.Version == "" || rule.GVK.Kind == "" {
			return nil, fmt.Errorf("resourceRules[%d].gvk: version and kind are required", i)
		}

		nsSel, err := selectorFromOptional(rule.NamespaceSelector)
		if err != nil {
			return nil, fmt.Errorf("resourceRules[%d].namespaceSelector: %w", i, err)
		}

		resSel, err := resourceLabelSelector(rule.MatchLabels, rule.MatchExpressions)
		if err != nil {
			return nil, fmt.Errorf("resourceRules[%d]: %w", i, err)
		}

		var prog cel.Program
		if strings.TrimSpace(rule.MatchPolicy) != "" {
			prog, err = compileMatchPolicy(celEnv, rule.MatchPolicy)
			if err != nil {
				return nil, fmt.Errorf("resourceRules[%d].matchPolicy: %w", i, err)
			}
		}

		compiled = append(compiled, CompiledResourceRule{
			Rule:            rule,
			GVR:             gvrFromProfile(rule.GVK),
			NamespaceSel:    nsSel,
			ResourceSel:     resSel,
			MatchPolicyProg: prog,
		})
	}

	return compiled, nil
}

// ResourceMatchesRules reports whether obj matches any compiled rule (OR union) or legacy fallback.
func ResourceMatchesRules(
	u *unstructured.Unstructured,
	gvr schema.GroupVersionResource,
	target *kollectdevv1alpha1.KollectTarget,
	profile *kollectdevv1alpha1.KollectProfile,
	rules []CompiledResourceRule,
	nsMeta map[string]NamespaceMeta,
) bool {
	if len(rules) == 0 {
		return resourceMatchesLegacy(u, target, profile, gvr)
	}

	resourceNS := u.GetNamespace()
	if resourceNS == "" {
		resourceNS = corev1.NamespaceDefault
	}

	internalMeta := namespaceMetaMapFromFilter(nsMeta)
	meta, ok := internalMeta[resourceNS]
	if !ok {
		meta = namespaceMeta{}
	}

	for _, rule := range rules {
		if rule.GVR != gvr {
			continue
		}

		if rule.NamespaceSel != nil && !rule.NamespaceSel.Matches(meta.Labels) {
			continue
		}

		if rule.ResourceSel != nil && !rule.ResourceSel.Matches(labels.Set(u.GetLabels())) {
			continue
		}

		if rule.MatchPolicyProg != nil {
			ok, err := evalMatchPolicy(rule.MatchPolicyProg, u)
			if err != nil || !ok {
				continue
			}
		}

		return true
	}

	return false
}

func resourceMatchesLegacy(
	u *unstructured.Unstructured,
	target *kollectdevv1alpha1.KollectTarget,
	profile *kollectdevv1alpha1.KollectProfile,
	gvr schema.GroupVersionResource,
) bool {
	if gvrFromProfile(profile.Spec.TargetGVK) != gvr {
		return false
	}

	if len(target.Spec.Names) > 0 {
		found := false
		for _, n := range target.Spec.Names {
			if n == u.GetName() {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	if target.Spec.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(target.Spec.LabelSelector)
		if err != nil {
			return false
		}

		if !selector.Matches(labels.Set(u.GetLabels())) {
			return false
		}
	}

	return true
}

func compileMatchPolicy(env *cel.Env, expr string) (cel.Program, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty CEL expression")
	}

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	prog, err := env.Program(ast)
	if err != nil {
		return nil, err
	}

	return prog, nil
}

func evalMatchPolicy(prog cel.Program, obj *unstructured.Unstructured) (bool, error) {
	out, _, err := prog.Eval(map[string]any{"object": obj.Object})
	if err != nil {
		return false, err
	}

	switch v := out.(type) {
	case types.Bool:
		return bool(v), nil
	case ref.Val:
		b, ok := v.ConvertToType(types.BoolType).(types.Bool)
		if !ok {
			return false, fmt.Errorf("matchPolicy result is not bool")
		}

		return bool(b), nil
	default:
		return false, fmt.Errorf("matchPolicy result is not bool")
	}
}

func selectorFromOptional(sel *metav1.LabelSelector) (labels.Selector, error) {
	if sel == nil {
		return labels.Everything(), nil
	}

	return metav1.LabelSelectorAsSelector(sel)
}

func resourceLabelSelector(
	matchLabels map[string]string,
	matchExpressions []metav1.LabelSelectorRequirement,
) (labels.Selector, error) {
	if len(matchLabels) == 0 && len(matchExpressions) == 0 {
		return labels.Everything(), nil
	}

	return metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels:      matchLabels,
		MatchExpressions: matchExpressions,
	})
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}

	return false
}

func unionStrings(a, b []string) []string {
	if len(a) == 0 {
		return append([]string(nil), b...)
	}

	if len(b) == 0 {
		return append([]string(nil), a...)
	}

	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, v := range append(append([]string(nil), a...), b...) {
		if v == "" {
			continue
		}

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}

func sortedUniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		out = append(out, v)
	}

	// Simple sort for stable status output.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}

	return out
}

// ScopeCeilingFromScope extracts namespace ceiling fields from a KollectScope.
func ScopeCeilingFromScope(scope *kollectdevv1alpha1.KollectScope) ScopeCeiling {
	if scope == nil {
		return ScopeCeiling{}
	}

	return ScopeCeiling{
		AllowedNamespaces: append([]string(nil), scope.Spec.AllowedNamespaces...),
		DeniedNamespaces:  append([]string(nil), scope.Spec.DeniedNamespaces...),
	}
}

// ScopeCeilingFromClusterScope extracts namespace ceiling fields from a KollectClusterScope.
func ScopeCeilingFromClusterScope(scope *kollectdevv1alpha1.KollectClusterScope) ScopeCeiling {
	if scope == nil {
		return ScopeCeiling{}
	}

	return ScopeCeiling{
		AllowedNamespaces: append([]string(nil), scope.Spec.AllowedNamespaces...),
		DeniedNamespaces:  append([]string(nil), scope.Spec.DeniedNamespaces...),
	}
}

// ComputeFilterStatus derives matched/effective namespaces and active rule count for status.
func ComputeFilterStatus(
	filter kollectdevv1alpha1.CollectionFilterSpec,
	namespaceSelector *metav1.LabelSelector,
	nsMeta map[string]NamespaceMeta,
	ceiling ScopeCeiling,
	defaults NamespaceDefaults,
) (matched, effective []string, activeRules int) {
	matched = MatchIntentNamespaces(filter, namespaceSelector, nsMeta, defaults)
	effective = EffectiveNamespaces(matched, ceiling, filter, defaults)
	activeRules = len(filter.ResourceRules)

	return matched, effective, activeRules
}

func EffectiveNamespaceSet(namespaces []string) map[string]struct{} {
	if len(namespaces) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		set[ns] = struct{}{}
	}

	return set
}
