//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package netguard

// Integration backends intentionally run on loopback or private container
// bridges. This compile-time-only policy cannot be enabled in production.
func init() {
	DefaultDialer.allowPrivate = true
}
