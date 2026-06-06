// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package v1alpha1

// ConnectionTestSinkRef identifies a family sink for KollectConnectionTest (ADR-0414).
type ConnectionTestSinkRef struct {
	// snapshotSinkRef names a KollectSnapshotSink or KollectClusterSnapshotSink.
	// +optional
	SnapshotSinkRef string `json:"snapshotSinkRef,omitempty"`

	// databaseSinkRef names a KollectDatabaseSink or KollectClusterDatabaseSink.
	// +optional
	DatabaseSinkRef string `json:"databaseSinkRef,omitempty"`

	// eventSinkRef names a KollectEventSink or KollectClusterEventSink.
	// +optional
	EventSinkRef string `json:"eventSinkRef,omitempty"`
}

// Family returns the sink family and name when exactly one ref field is set.
func (r ConnectionTestSinkRef) Family() (family, name string, ok bool) {
	set := 0
	if r.SnapshotSinkRef != "" {
		set++
		family, name = SinkFamilySnapshot, r.SnapshotSinkRef
	}
	if r.DatabaseSinkRef != "" {
		set++
		family, name = SinkFamilyDatabase, r.DatabaseSinkRef
	}
	if r.EventSinkRef != "" {
		set++
		family, name = SinkFamilyEvent, r.EventSinkRef
	}
	return family, name, set == 1 && name != ""
}
