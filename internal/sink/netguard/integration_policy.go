//go:build integration

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package netguard

// Integration backends intentionally run on loopback or private container
// bridges (often addressed as hostname "localhost"). This compile-time-only
// policy cannot be enabled in production; it permits private resolved IPs and
// the localhost hostname family while metadata hostnames stay denied.
func init() {
	DefaultDialer.allowPrivate = true
}
