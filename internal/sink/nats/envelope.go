// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package nats

import (
	"encoding/json"
	"time"

	"github.com/konih/kollect/internal/export"
)

func marshalEventEnvelope(cluster, namespace string, payload []byte, at time.Time) ([]byte, error) {
	envelope := EventEnvelope{
		SchemaVersion: export.SchemaVersion,
		Timestamp:     at.UTC().Format(time.RFC3339Nano),
		Cluster:       cluster,
		Namespace:     namespace,
		Payload:       json.RawMessage(payload),
	}

	return json.Marshal(envelope)
}
