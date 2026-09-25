// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kollectdevv1alpha1 "github.com/platformrelay/kollect/api/v1alpha1"
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
}

func (r SinkCleanupReport) empty() bool {
	return len(r.SinkGone) == 0 && len(r.Retained) == 0
}

func cleanupSinkExports(
	ctx context.Context,
	c client.Client,
	registry *sink.Registry,
	sinkNamespace string,
	bindings []kollectdevv1alpha1.InventorySinkBinding,
	clusterScoped bool,
	objectPath string,
	generation int64,
) (SinkCleanupReport, error) {
	report := SinkCleanupReport{}
	if registry == nil || len(bindings) == 0 {
		return report, nil
	}

	var errs []error

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

		outcome, cerr := sink.RunCleanupExport(sink.CleanupExportRequest{
			Ctx:           ctx,
			Client:        c,
			Registry:      registry,
			SinkNamespace: sink.SinkNamespaceForResolved(resolved, sinkNamespace),
			SinkName:      binding.Name,
			SinkUID:       resolved.UID,
			SinkSpec:      resolved.Spec,
			ObjectPath:    objectPath,
			Generation:    generation,
		})
		if cerr != nil {
			errs = append(errs, cerr)

			continue
		}

		if outcome == sink.CleanupRetained {
			// objectPath is the inventory's canonical export identity, not
			// necessarily a literal file (git YAML sinks write .yaml siblings).
			report.Retained = append(report.Retained, fmt.Sprintf(
				"%s (inventory export identity %s)", sinkExportKey(binding), objectPath))
		}
	}

	return report, errors.Join(errs...)
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

	for _, retained := range report.Retained {
		recordWarning(recorder, inv, reasonCleanupRetained, fmt.Sprintf(
			"backend cleanup could not retract everything for %s: previously exported data "+
				"may be retained — unsupported backend, layout-tree files, past generations, "+
				"or a merge request awaiting merge "+
				"(deletion proceeds; retract manually if required)", retained))
	}
}
