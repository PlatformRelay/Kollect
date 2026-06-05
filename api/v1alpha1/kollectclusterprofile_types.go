// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// KollectClusterProfile is reserved for platform-wide shared extraction schemas (ADR-0031).
// Cluster-scoped counterpart to namespaced KollectProfile — not registered in the scheme until
// platform rollup needs shared GVK definitions. Short name kcprof when implemented.
//
// TODO(platform): add kubebuilder markers, CRD, and webhook when KollectClusterProfile ships:
//
//	// +kubebuilder:object:root=true
//	// +kubebuilder:resource:scope=Cluster,shortName=kcprof
//	type KollectClusterProfile struct { ... same Spec/Status shape as KollectProfile ... }
