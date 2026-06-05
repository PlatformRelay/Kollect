// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package transport

import (
	"fmt"
	"os"
	"strings"
)

// ACLSettings configures optional broker-side subject/stream ACL hints for production wiring.
type ACLSettings struct {
	AllowedClusterIDs []string
	PublishSubjects   []string
	SubscribeSubjects []string
}

// ACLSettingsFromEnv reads KOLLECT_TRANSPORT_ACL_* variables.
func ACLSettingsFromEnv() ACLSettings {
	return ACLSettings{
		AllowedClusterIDs: splitCSV(os.Getenv("KOLLECT_TRANSPORT_ACL_ALLOWED_CLUSTERS")),
		PublishSubjects:   splitCSV(os.Getenv("KOLLECT_TRANSPORT_ACL_PUBLISH_SUBJECTS")),
		SubscribeSubjects: splitCSV(os.Getenv("KOLLECT_TRANSPORT_ACL_SUBSCRIBE_SUBJECTS")),
	}
}

// Enabled reports whether any ACL hint is configured.
func (a ACLSettings) Enabled() bool {
	return len(a.AllowedClusterIDs) > 0 ||
		len(a.PublishSubjects) > 0 ||
		len(a.SubscribeSubjects) > 0
}

// ValidateClusterID returns an error when allowlist is set and clusterID is not listed.
func (a ACLSettings) ValidateClusterID(clusterID string) error {
	if len(a.AllowedClusterIDs) == 0 {
		return nil
	}

	clusterID = strings.TrimSpace(clusterID)
	for _, allowed := range a.AllowedClusterIDs {
		if clusterID == allowed {
			return nil
		}
	}

	return fmt.Errorf("transport acl: cluster %q not in allowlist", clusterID)
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}

	return out
}
