// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	sigsyaml "sigs.k8s.io/yaml"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
)

// Namespace-scope and filter option labels shown by the init wizard.
const (
	InitScopeAll            = "All accessible namespaces"
	InitScopeExplicit       = "Explicit namespace list"
	InitScopeLabelSelector  = "Label namespaceSelector"
	InitScopeNamePattern    = "Discovery-time name pattern (snapshot)"
	InitFilterNone          = "No resource filter"
	InitFilterLabelSelector = "Label selector"
	InitFilterExplicitNames = "Explicit resource name list"
	InitSamplesPointer      = "config/samples/pipeline"
	initProfileFileName     = "profile.yaml"
	initTargetFileName      = "target.yaml"
)

// ErrNonInteractive is returned when stdin is not a TTY.
var ErrNonInteractive = fmt.Errorf(
	"init requires an interactive terminal (TTY); copy a starter from %s instead",
	InitSamplesPointer,
)

// IsInitCanceled reports whether err is (or wraps) ErrCanceled.
func IsInitCanceled(err error) bool {
	return errors.Is(err, ErrCanceled)
}

// InitResourceInfo is one discoverable API resource.
type InitResourceInfo struct {
	Group      string
	Version    string
	Kind       string
	Resource   string
	Namespaced bool
	Verbs      []string
}

// Label returns a human-readable select option for this resource.
func (r InitResourceInfo) Label() string {
	if r.Group == "" {
		return fmt.Sprintf("%s (%s)", r.Kind, r.Version)
	}
	return fmt.Sprintf("%s (%s/%s)", r.Kind, r.Group, r.Version)
}

// InitDiscoverer lists API resources and namespaces using the caller's RBAC.
type InitDiscoverer interface {
	ListResources(ctx context.Context) ([]InitResourceInfo, error)
	ListNamespaces(ctx context.Context) ([]string, error)
}

// FakeInitDiscoverer is an injectable InitDiscoverer for tests.
type FakeInitDiscoverer struct {
	Resources  []InitResourceInfo
	Namespaces []string
	Err        error
}

// ListResources implements InitDiscoverer.
func (f *FakeInitDiscoverer) ListResources(context.Context) ([]InitResourceInfo, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	out := make([]InitResourceInfo, len(f.Resources))
	copy(out, f.Resources)
	return out, nil
}

// ListNamespaces implements InitDiscoverer.
func (f *FakeInitDiscoverer) ListNamespaces(context.Context) ([]string, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	out := make([]string, len(f.Namespaces))
	copy(out, f.Namespaces)
	return out, nil
}

// InitOptions configures a wizard run.
type InitOptions struct {
	Kubeconfig string
	Context    string
	OutputDir  string
	Prompter   Prompter
	Discoverer InitDiscoverer
	Stdout     io.Writer
	Stderr     io.Writer
	IsTerminal func() bool
	Color      bool
}

// InitResult holds paths written by a successful run.
type InitResult struct {
	ProfilePath string
	TargetPath  string
}

// RunInit executes the init wizard end-to-end.
func RunInit(opts InitOptions) (InitResult, error) {
	if err := validateInitOptions(&opts); err != nil {
		return InitResult{}, err
	}
	if err := confirmKubecontext(opts); err != nil {
		return InitResult{}, err
	}
	chosen, err := selectInitResource(opts)
	if err != nil {
		return InitResult{}, err
	}
	intent := initDraft{Resource: chosen}
	if err := collectInitScope(opts, &intent); err != nil {
		return InitResult{}, err
	}
	if err := collectInitFilter(opts, &intent); err != nil {
		return InitResult{}, err
	}
	if err := collectInitAttributesAndName(opts, &intent); err != nil {
		return InitResult{}, err
	}
	if err := confirmInitReview(opts, &intent); err != nil {
		return InitResult{}, err
	}
	return writeInitIntent(opts, &intent)
}

func validateInitOptions(opts *InitOptions) error {
	if opts.Prompter == nil {
		return fmt.Errorf("prompter is required")
	}
	if opts.OutputDir == "" {
		return fmt.Errorf("--output-dir is required")
	}
	if opts.Discoverer == nil {
		return fmt.Errorf("discoverer is required")
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.IsTerminal != nil && !opts.IsTerminal() {
		return ErrNonInteractive
	}
	return nil
}

func confirmKubecontext(opts InitOptions) error {
	ctxInfo, err := resolveInitContextInfo(opts.Kubeconfig, opts.Context)
	if err != nil {
		return err
	}
	printInitPlain(opts, "Kubecontext: %s\nServer:      %s\n", ctxInfo.Name, ctxInfo.Server)
	ok, err := opts.Prompter.Confirm(
		fmt.Sprintf("Use kubecontext %q (%s)?", ctxInfo.Name, ctxInfo.Server),
		true,
	)
	if err != nil {
		return mapInitPromptErr(err)
	}
	if !ok {
		return ErrCanceled
	}
	return nil
}

func selectInitResource(opts InitOptions) (InitResourceInfo, error) {
	resources, err := opts.Discoverer.ListResources(context.Background())
	if err != nil {
		return InitResourceInfo{}, fmt.Errorf("API discovery failed (check RBAC / connectivity): %w", err)
	}
	resources = preferListableInit(resources)
	if len(resources) == 0 {
		return InitResourceInfo{}, fmt.Errorf(
			"no listable API resources discovered; check RBAC list/get permissions")
	}

	labels := make([]string, 0, len(resources))
	byLabel := make(map[string]InitResourceInfo, len(resources))
	for _, r := range resources {
		l := r.Label()
		labels = append(labels, l)
		byLabel[l] = r
	}
	sort.Strings(labels)

	chosenLabel, err := opts.Prompter.Select("Select a resource kind to collect", labels)
	if err != nil {
		return InitResourceInfo{}, mapInitPromptErr(err)
	}
	chosen, ok := byLabel[chosenLabel]
	if !ok {
		return InitResourceInfo{}, fmt.Errorf("unknown resource selection %q", chosenLabel)
	}
	return chosen, nil
}

func collectInitScope(opts InitOptions, intent *initDraft) error {
	if !intent.Resource.Namespaced {
		return nil
	}
	scope, scopeErr := opts.Prompter.Select("Namespace scope", []string{
		InitScopeAll, InitScopeExplicit, InitScopeLabelSelector, InitScopeNamePattern,
	})
	if scopeErr != nil {
		return mapInitPromptErr(scopeErr)
	}
	intent.ScopeMode = scope
	switch scope {
	case InitScopeExplicit:
		return collectInitExplicitNamespaces(opts, intent)
	case InitScopeLabelSelector:
		raw, inputErr := opts.Prompter.Input(
			"Namespace label selector (e.g. team=platform)", "", requireInitNonEmpty)
		if inputErr != nil {
			return mapInitPromptErr(inputErr)
		}
		intent.NamespaceSelector = raw
	case InitScopeNamePattern:
		return collectInitNamePattern(opts, intent)
	}
	return nil
}

func collectInitExplicitNamespaces(opts InitOptions, intent *initDraft) error {
	nsList, listErr := opts.Discoverer.ListNamespaces(context.Background())
	if listErr != nil {
		return fmt.Errorf("list namespaces: %w", listErr)
	}
	if len(nsList) == 0 {
		return fmt.Errorf("no namespaces visible with current RBAC")
	}
	sort.Strings(nsList)
	picked, pickErr := opts.Prompter.MultiSelect("Select namespaces", nsList, nil)
	if pickErr != nil {
		return mapInitPromptErr(pickErr)
	}
	intent.IncludedNamespaces = append([]string(nil), picked...)
	return nil
}

func collectInitNamePattern(opts InitOptions, intent *initDraft) error {
	raw, inputErr := opts.Prompter.Input(
		"Namespace name pattern (discovery snapshot, e.g. team-*)", "", requireInitNonEmpty)
	if inputErr != nil {
		return mapInitPromptErr(inputErr)
	}
	nsList, listErr := opts.Discoverer.ListNamespaces(context.Background())
	if listErr != nil {
		return fmt.Errorf("list namespaces: %w", listErr)
	}
	matched := matchInitPattern(nsList, raw)
	printInitPlain(opts, "Matched namespaces (snapshot): %s\n", strings.Join(matched, ", "))
	intent.IncludedNamespaces = matched
	intent.ScopeWarning = "name pattern expanded to a snapshot includedNamespaces list (not a durable glob)"
	return nil
}

func collectInitFilter(opts InitOptions, intent *initDraft) error {
	filterMode, filterErr := opts.Prompter.Select("Resource filter", []string{
		InitFilterNone, InitFilterLabelSelector, InitFilterExplicitNames,
	})
	if filterErr != nil {
		return mapInitPromptErr(filterErr)
	}
	intent.FilterMode = filterMode
	switch filterMode {
	case InitFilterLabelSelector:
		raw, inputErr := opts.Prompter.Input(
			"Resource label selector (e.g. app.kubernetes.io/name=api)", "", requireInitNonEmpty)
		if inputErr != nil {
			return mapInitPromptErr(inputErr)
		}
		intent.LabelSelector = raw
	case InitFilterExplicitNames:
		raw, inputErr := opts.Prompter.Input(
			"Comma-separated resource names", "", requireInitNonEmpty)
		if inputErr != nil {
			return mapInitPromptErr(inputErr)
		}
		intent.Names = splitInitCSV(raw)
	}
	return nil
}

func collectInitAttributesAndName(opts InitOptions, intent *initDraft) error {
	attrOpts := safeInitAttributeOptions(intent.Resource.Namespaced)
	attrDefaults := make([]string, 0, len(attrOpts))
	for _, a := range attrOpts {
		attrDefaults = append(attrDefaults, a.Name)
	}
	pickedAttrs, multiErr := opts.Prompter.MultiSelect(
		"Attributes to extract (safe metadata defaults)", attrDefaults, attrDefaults)
	if multiErr != nil {
		return mapInitPromptErr(multiErr)
	}
	intent.Attributes = resolveInitAttributes(attrOpts, pickedAttrs)

	defaultName := defaultInitResourceName(intent.Resource.Kind)
	name, inputErr := opts.Prompter.Input(
		"Profile and Target name", defaultName, validateInitResourceName)
	if inputErr != nil {
		return mapInitPromptErr(inputErr)
	}
	intent.Name = name
	return nil
}

func confirmInitReview(opts InitOptions, intent *initDraft) error {
	printInitPlain(opts, "\n%s\n", intent.ReviewSummary())
	ok, confirmErr := opts.Prompter.Confirm("Write KollectProfile + KollectTarget YAML?", true)
	if confirmErr != nil {
		return mapInitPromptErr(confirmErr)
	}
	if !ok {
		return ErrCanceled
	}
	return nil
}

func writeInitIntent(opts InitOptions, intent *initDraft) (InitResult, error) {
	profileYAML, targetYAML, err := intent.RenderYAML()
	if err != nil {
		return InitResult{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o750); err != nil {
		return InitResult{}, fmt.Errorf("create output dir: %w", err)
	}

	profilePath := filepath.Join(opts.OutputDir, initProfileFileName)
	targetPath := filepath.Join(opts.OutputDir, initTargetFileName)

	if err := confirmInitOverwrite(opts, profilePath, profileYAML); err != nil {
		return InitResult{}, err
	}
	if err := confirmInitOverwrite(opts, targetPath, targetYAML); err != nil {
		return InitResult{}, err
	}
	if err := os.WriteFile(profilePath, profileYAML, 0o600); err != nil {
		return InitResult{}, fmt.Errorf("write profile: %w", err)
	}
	if err := os.WriteFile(targetPath, targetYAML, 0o600); err != nil {
		return InitResult{}, fmt.Errorf("write target: %w", err)
	}

	printInitPlain(opts, "Wrote %s\nWrote %s\n", profilePath, targetPath)
	return InitResult{ProfilePath: profilePath, TargetPath: targetPath}, nil
}

type initContextInfo struct {
	Name   string
	Server string
}

func resolveInitContextInfo(kubeconfig, contextName string) (initContextInfo, error) {
	cfg, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return initContextInfo{}, fmt.Errorf("load kubeconfig %q: %w", kubeconfig, err)
	}
	name := contextName
	if name == "" {
		name = cfg.CurrentContext
	}
	if name == "" {
		return initContextInfo{}, fmt.Errorf(
			"kubeconfig %q has no current-context and no --context was given", kubeconfig)
	}
	ctx, ok := cfg.Contexts[name]
	if !ok {
		return initContextInfo{}, fmt.Errorf("context %q not found in kubeconfig %q", name, kubeconfig)
	}
	server := ""
	if cluster, ok := cfg.Clusters[ctx.Cluster]; ok && cluster != nil {
		server = cluster.Server
	}
	if server == "" {
		server = "(unknown server)"
	}
	return initContextInfo{Name: name, Server: server}, nil
}

func preferListableInit(in []InitResourceInfo) []InitResourceInfo {
	var out []InitResourceInfo
	for _, r := range in {
		if r.Kind == "" || strings.Contains(r.Kind, ".") {
			continue
		}
		if hasInitVerb(r.Verbs, "list") || hasInitVerb(r.Verbs, "get") {
			out = append(out, r)
		}
	}
	return out
}

func hasInitVerb(verbs []string, want string) bool {
	for _, v := range verbs {
		if v == want {
			return true
		}
	}
	return false
}

type initAttributeOpt struct {
	Name string
	Path string
	Type string
}

func safeInitAttributeOptions(namespaced bool) []initAttributeOpt {
	opts := []initAttributeOpt{
		{Name: "name", Path: "$.metadata.name", Type: "string"},
		{Name: "creationTimestamp", Path: "$.metadata.creationTimestamp", Type: "string"},
		{Name: "labels", Path: "$.metadata.labels", Type: "object"},
	}
	if namespaced {
		opts = append([]initAttributeOpt{
			{Name: "namespace", Path: "$.metadata.namespace", Type: "string"},
		}, opts...)
	}
	return opts
}

func resolveInitAttributes(opts []initAttributeOpt, names []string) []initAttributeOpt {
	byName := make(map[string]initAttributeOpt, len(opts))
	for _, o := range opts {
		byName[o.Name] = o
	}
	var out []initAttributeOpt
	for _, n := range names {
		if a, ok := byName[n]; ok {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return opts[:1]
	}
	return out
}

type initDraft struct {
	Name               string
	Resource           InitResourceInfo
	ScopeMode          string
	IncludedNamespaces []string
	NamespaceSelector  string
	ScopeWarning       string
	FilterMode         string
	LabelSelector      string
	Names              []string
	Attributes         []initAttributeOpt
}

func (d initDraft) ReviewSummary() string {
	var b strings.Builder
	b.WriteString("Review generated intent\n")
	b.WriteString("-----------------------\n")
	fmt.Fprintf(&b, "Name:       %s\n", d.Name)
	fmt.Fprintf(&b, "GVK:        %s\n", d.Resource.Label())
	fmt.Fprintf(&b, "Namespaced: %v\n", d.Resource.Namespaced)
	if d.Resource.Namespaced {
		fmt.Fprintf(&b, "Scope:      %s\n", d.ScopeMode)
		if len(d.IncludedNamespaces) > 0 {
			fmt.Fprintf(&b, "Namespaces: %s\n", strings.Join(d.IncludedNamespaces, ", "))
		}
		if d.NamespaceSelector != "" {
			fmt.Fprintf(&b, "NS selector:%s\n", d.NamespaceSelector)
		}
	}
	fmt.Fprintf(&b, "Filter:     %s\n", d.FilterMode)
	if d.LabelSelector != "" {
		fmt.Fprintf(&b, "Labels:     %s\n", d.LabelSelector)
	}
	if len(d.Names) > 0 {
		fmt.Fprintf(&b, "Names:      %s\n", strings.Join(d.Names, ", "))
	}
	attrs := make([]string, 0, len(d.Attributes))
	for _, a := range d.Attributes {
		attrs = append(attrs, a.Name)
	}
	fmt.Fprintf(&b, "Attributes: %s\n", strings.Join(attrs, ", "))
	if d.ScopeWarning != "" {
		fmt.Fprintf(&b, "Warning:    %s\n", d.ScopeWarning)
	}
	return b.String()
}

func (d initDraft) RenderYAML() (profileYAML, targetYAML []byte, err error) {
	profile := kollectdevv1alpha1.KollectProfile{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kollectdevv1alpha1.GroupVersion.String(),
			Kind:       "KollectProfile",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.Name,
			Namespace: "kollect-system",
		},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{
				Group:   d.Resource.Group,
				Version: d.Resource.Version,
				Kind:    d.Resource.Kind,
			},
		},
	}
	for _, a := range d.Attributes {
		profile.Spec.Attributes = append(profile.Spec.Attributes, kollectdevv1alpha1.AttributeSpec{
			Name: a.Name,
			Path: a.Path,
			Type: a.Type,
		})
	}

	target := kollectdevv1alpha1.KollectTarget{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kollectdevv1alpha1.GroupVersion.String(),
			Kind:       "KollectTarget",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.Name,
			Namespace: "kollect-system",
		},
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			ProfileRef: d.Name,
		},
	}
	if len(d.IncludedNamespaces) > 0 {
		target.Spec.IncludedNamespaces = append([]string(nil), d.IncludedNamespaces...)
	}
	if d.NamespaceSelector != "" {
		sel, parseErr := parseInitMatchLabels(d.NamespaceSelector)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("namespaceSelector: %w", parseErr)
		}
		target.Spec.NamespaceSelector = &metav1.LabelSelector{MatchLabels: sel}
	}
	if d.LabelSelector != "" {
		sel, parseErr := parseInitMatchLabels(d.LabelSelector)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("labelSelector: %w", parseErr)
		}
		target.Spec.LabelSelector = &metav1.LabelSelector{MatchLabels: sel}
	}
	if len(d.Names) > 0 {
		target.Spec.Names = append([]string(nil), d.Names...)
	}

	profileYAML, err = sigsyaml.Marshal(&profile)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal profile: %w", err)
	}
	targetYAML, err = sigsyaml.Marshal(&target)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal target: %w", err)
	}
	return profileYAML, targetYAML, nil
}

func confirmInitOverwrite(opts InitOptions, path string, newContent []byte) error {
	existing, err := os.ReadFile(path) //nolint:gosec // path is under OutputDir chosen by the operator
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	printInitPlain(opts,
		"\nFile already exists: %s\n--- existing (%d bytes) ---\n%s\n--- new (%d bytes) ---\n%s\n",
		path, len(existing), truncateInit(string(existing), 800),
		len(newContent), truncateInit(string(newContent), 800))
	ok, confirmErr := opts.Prompter.Confirm(
		fmt.Sprintf("Overwrite %s?", filepath.Base(path)), false)
	if confirmErr != nil {
		return mapInitPromptErr(confirmErr)
	}
	if !ok {
		return fmt.Errorf("%w: overwrite of %s declined", ErrCanceled, path)
	}
	return nil
}

func printInitPlain(opts InitOptions, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !opts.Color {
		msg = stripInitANSI(msg)
	}
	_, _ = io.WriteString(opts.Stderr, msg)
}

func stripInitANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func truncateInit(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(truncated)"
}

func mapInitPromptErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrCanceled) {
		return err
	}
	if err.Error() == "interrupt" || strings.Contains(strings.ToLower(err.Error()), "interrupt") {
		return ErrCanceled
	}
	return err
}

func requireInitNonEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("value must not be empty")
	}
	return nil
}

func validateInitResourceName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("name must not be empty")
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return fmt.Errorf("name must be lowercase DNS-1123 (a-z, 0-9, -)")
	}
	return nil
}

func defaultInitResourceName(kind string) string {
	s := strings.ToLower(kind)
	return strings.ReplaceAll(s, " ", "-")
}

func splitInitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseInitMatchLabels(raw string) (map[string]string, error) {
	out := map[string]string{}
	for _, part := range splitInitCSV(raw) {
		for _, token := range strings.Fields(part) {
			kv := strings.SplitN(token, "=", 2)
			if len(kv) != 2 || kv[0] == "" {
				return nil, fmt.Errorf("expected key=value, got %q", token)
			}
			out[kv[0]] = kv[1]
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no key=value pairs")
	}
	return out, nil
}

func matchInitPattern(names []string, pattern string) []string {
	var out []string
	for _, n := range names {
		ok, err := filepath.Match(pattern, n)
		if err == nil && ok {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}
