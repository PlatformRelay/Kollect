// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink"
)

// SinkCleanupReport classifies per-sink cleanup outcomes so the finalizer path
// can announce the loud ones (K-28 retention, K-29 vanished sinks) while
// deletion proceeds.
type SinkCleanupReport struct {
	// SinkGone lists "<family>/<name>" of bound sinks whose CR was already deleted
	// when cleanup ran (namespace cascade ordering). Nothing can be retracted
	// through a sink whose spec and credentials are gone, but this is NOT an
	// error: refusing here wedged the whole namespace in Terminating (K-29).
	SinkGone []string
	// Retained lists "<family>/<name> (path)" entries whose exported data the
	// backend physically cannot retract (event emitters, layout trees).
	Retained []string
	// RetainedByPolicy lists "<family>/<name>" of snapshot sinks whose
	// deletionPolicy is Retain (the default, ADR-0421): exports deliberately kept.
	RetainedByPolicy []string
	// SharedIdentity lists "<family>/<name> (path)" entries whose Delete-policy
	// retraction was skipped because another live inventory renders the same
	// export identity.
	SharedIdentity []string
}

func (r SinkCleanupReport) empty() bool {
	return len(r.SinkGone) == 0 && len(r.Retained) == 0 &&
		len(r.RetainedByPolicy) == 0 && len(r.SharedIdentity) == 0
}

// sinkExportEvidence is what the deleting inventory's status says about its
// last export to one sink: the recorded paths, and whether it exported at all.
type sinkExportEvidence struct {
	paths    []string
	exported bool
}

// cleanupTarget identifies the deleting inventory for cleanupSinkExports.
type cleanupTarget struct {
	objectPath string
	generation int64
	// evidence is keyed by sink export key ("<family>/<name>").
	evidence map[string]sinkExportEvidence
	// sharedIdentity reports whether another live inventory renders the same
	// export identity. It is consulted only for sinks that would retract, at
	// most once per cleanup; nil means the identity cannot be shared.
	sharedIdentity func(context.Context) (bool, error)
}

// sharedIdentityOnce memoizes target.sharedIdentity for one cleanup pass.
func (t cleanupTarget) sharedIdentityOnce() func(context.Context) (bool, error) {
	var (
		done   bool
		shared bool
		err    error
	)

	return func(ctx context.Context) (bool, error) {
		if t.sharedIdentity == nil {
			return false, nil
		}
		if !done {
			shared, err = t.sharedIdentity(ctx)
			done = err == nil
		}

		return shared, err
	}
}

func cleanupSinkExports(
	ctx context.Context,
	c client.Client,
	registry *sink.Registry,
	sinkNamespace string,
	bindings []kollectdevv1alpha1.InventorySinkBinding,
	clusterScoped bool,
	target cleanupTarget,
) (SinkCleanupReport, error) {
	report := SinkCleanupReport{}
	if registry == nil || len(bindings) == 0 {
		return report, nil
	}

	var errs []error

	sharedIdentity := target.sharedIdentityOnce()

	for _, binding := range bindings {
		var (
			resolved *sink.ResolvedSink
			err      error
		)
		if clusterScoped {
			resolved, err = loadClusterInventorySink(ctx, c, sinkNamespace, binding)
		} else {
			resolved, err = loadResolvedSink(ctx, c, sinkNamespace, binding)
		}
		if err != nil {
			// K-29: the sink CR vanished (deleted before the inventory, e.g. by
			// namespace cascade). There is no backend to reach through it —
			// treat as "nothing to clean" and announce the possible retention
			// instead of classifying NotFound terminal and wedging deletion.
			if apierrors.IsNotFound(err) {
				report.SinkGone = append(report.SinkGone, sinkExportKey(binding))

				continue
			}

			errs = append(errs, err)

			continue
		}

		shared := false
		if sink.RetractsOnDeletion(resolved.Spec) {
			var serr error
			if shared, serr = sharedIdentity(ctx); serr != nil {
				errs = append(errs, serr)

				continue
			}
		}

		evidence := target.evidence[sinkExportKey(binding)]
		outcome, cerr := sink.RunCleanupExport(sink.CleanupExportRequest{
			Ctx:                   ctx,
			Client:                c,
			Registry:              registry,
			SinkNamespace:         sink.SinkNamespaceForResolved(resolved, sinkNamespace),
			SinkName:              binding.Name,
			SinkUID:               resolved.UID,
			SinkSpec:              resolved.Spec,
			ObjectPath:            target.objectPath,
			Generation:            target.generation,
			LastExportedPaths:     evidence.paths,
			ExportPathsUnrecorded: evidence.exported && len(evidence.paths) == 0,
			SharedExportIdentity:  shared,
		})
		if cerr != nil {
			errs = append(errs, cerr)

			continue
		}

		report.add(outcome, sinkExportKey(binding), target.objectPath)
	}

	return report, errors.Join(errs...)
}

// add files one sink's cleanup outcome under the matching announcement.
// objectPath is the inventory's canonical export identity, not necessarily a
// literal file (git YAML sinks write .yaml siblings).
func (r *SinkCleanupReport) add(outcome sink.CleanupExportOutcome, key, objectPath string) {
	switch outcome {
	case sink.CleanupRetained:
		r.Retained = append(r.Retained, fmt.Sprintf("%s (inventory export identity %s)", key, objectPath))
	case sink.CleanupRetainedByPolicy:
		r.RetainedByPolicy = append(r.RetainedByPolicy, key)
	case sink.CleanupRetainedSharedIdentity:
		r.SharedIdentity = append(r.SharedIdentity, fmt.Sprintf("%s (inventory export identity %s)", key, objectPath))
	case sink.CleanupCleaned, sink.CleanupPruned:
	}
}

// exportEvidenceBySink indexes the deleting inventory's sink export status by
// sink export key ("<family>/<name>") so the cleanup path can consult what the
// last successful export actually wrote (K-28 evidence), and whether a past
// export left no recording to consult.
func exportEvidenceBySink(
	exports []kollectdevv1alpha1.InventorySinkExportStatus,
) map[string]sinkExportEvidence {
	out := make(map[string]sinkExportEvidence, len(exports))
	for i := range exports {
		out[exports[i].Name] = sinkExportEvidence{
			paths:    exports[i].LastExportPaths,
			exported: exports[i].LastExportTime != nil,
		}
	}

	return out
}

// clusterExportNamespace is the namespace segment a KollectClusterInventory
// renders into its export identity (inventory/cluster/<name>), which a
// KollectInventory in a namespace of that name renders too.
const clusterExportNamespace = "cluster"

// exportIdentityShared reports whether another live object of kind obj, at key,
// renders the same export identity as the deleting inventory.
//
//   - Found: shared — the retraction would delete the other object's export.
//   - NotFound: not shared.
//   - Forbidden, or the kind is not served: the operator cannot read that scope,
//     so it cannot have reconciled — or exported — such an object. Not shared.
//   - Anything else is an unknown: a transient error, so cleanup retries with
//     the finalizer kept instead of either skipping the retraction for good or
//     risking the other export.
func exportIdentityShared(ctx context.Context, c client.Client, key client.ObjectKey, obj client.Object) (bool, error) {
	err := c.Get(ctx, key, obj)
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err), apierrors.IsForbidden(err), apimeta.IsNoMatchError(err):
		return false, nil
	default:
		return false, kollecterrors.Transient(fmt.Errorf("check shared export identity %s: %w", key, err))
	}
}

// recordCleanupAnnouncements turns a cleanup report's loud outcomes into Warning
// Events. Neither outcome blocks finalizer removal; both say so in the message.
func recordCleanupAnnouncements(recorder record.EventRecorder, inv runtime.Object, report SinkCleanupReport) {
	if report.empty() || recorder == nil {
		return
	}

	for _, gone := range report.SinkGone {
		recordWarning(recorder, inv, reasonCleanupSinkGone, fmt.Sprintf(
			"sink %q was already deleted before inventory cleanup: nothing to clean through it, "+
				"but objects it had exported may be retained in its backend (deletion proceeds)",
			gone))
	}

	for _, kept := range report.RetainedByPolicy {
		recordNormal(recorder, inv, reasonCleanupRetainedByPolicy, fmt.Sprintf(
			"sink %q has deletionPolicy %s: the objects it exported for this inventory are left in place "+
				"(set deletionPolicy: %s on the sink to retract them on deletion)",
			kept, kollectdevv1alpha1.DeletionPolicyRetain, kollectdevv1alpha1.DeletionPolicyDelete))
	}

	for _, shared := range report.SharedIdentity {
		recordWarning(recorder, inv, reasonCleanupSharedIdentity, fmt.Sprintf(
			"retraction skipped for %s: a KollectClusterInventory and a KollectInventory in namespace %q "+
				"with the same name share this export identity, so deleting it would remove the other's export "+
				"(deletion proceeds; retract manually if required)", shared, clusterExportNamespace))
	}

	for _, retained := range report.Retained {
		recordWarning(recorder, inv, reasonCleanupRetained, fmt.Sprintf(
			"backend cleanup could not retract everything for %s: previously exported data "+
				"may be retained — unsupported backend, layout-tree files, past generations, "+
				"or a merge request awaiting merge "+
				"(deletion proceeds; retract manually if required)", retained))
	}
}
