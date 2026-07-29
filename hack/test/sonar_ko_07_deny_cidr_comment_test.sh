#!/usr/bin/env bash
# Meta test for SEC-04g (KO-07, SonarCloud go:S1313 "hardcoded IP address").
#
# SonarCloud flags the literal CIDR/hostname strings in
# internal/validation/endpoint_guard.go as hardcoded IP addresses. This is a
# SAFE accept: the literals ARE the intended SSRF deny-list security control
# (loopback/link-local/cloud-metadata/carrier-NAT/benchmark ranges plus known
# metadata hostnames), not accidental config that should be externalized.
# Externalizing/making this list overridable would weaken the control, so
# this test locks in both (a) the documenting SAFE comment staying present,
# and (b) the known-critical deny entries staying verbatim.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SRC="${ROOT}/internal/validation/endpoint_guard.go"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

[[ -f "${SRC}" ]] || fail "expected ${SRC} to exist"

# --- a reviewed SAFE marker referencing S1313 must be present near the deny-list ---
if ! grep -qi 'safe' "${SRC}" || ! grep -qi 's1313' "${SRC}"; then
  fail "expected a SAFE comment referencing S1313 near the deny-list declarations in ${SRC}"
fi
pass "SAFE S1313 comment present"

# --- the comment must call out that this is a deliberate SSRF security control ---
if ! grep -qi 'ssrf' "${SRC}"; then
  fail "expected the SAFE comment to reference SSRF (the security control being documented)"
fi
pass "comment references SSRF"

# --- the comment must warn against externalizing/making the list overridable ---
# (guards against a future "helpful" refactor turning this into configurable,
# attacker-overridable input, which would weaken the control).
if ! grep -Eqi 'externaliz|configurab|overrid' "${SRC}"; then
  fail "expected the SAFE comment to warn against externalizing/making the deny-list configurable/overridable"
fi
pass "comment warns against externalizing/making the deny-list configurable"

# --- the SAFE comment must live near the actual declarations, not floating elsewhere ---
# Find the line number of the denyCIDRs declaration and require the SAFE/S1313
# marker to appear within a few lines above it.
DECL_LINE="$(grep -n 'denyCIDRs[[:space:]]*=' "${SRC}" | head -1 | cut -d: -f1)"
[[ -n "${DECL_LINE}" ]] || fail "could not find denyCIDRs declaration in ${SRC}"

CONTEXT_START=$(( DECL_LINE > 25 ? DECL_LINE - 25 : 1 ))
CONTEXT="$(sed -n "${CONTEXT_START},$((DECL_LINE))p" "${SRC}")"
if ! echo "${CONTEXT}" | grep -qi 'safe'; then
  fail "SAFE comment must appear directly above/near the denyCIDRs declaration (within 25 lines)"
fi
pass "SAFE comment is located near the denyCIDRs declaration"

# --- known-critical CIDR entries must remain verbatim (nobody weakens the deny-list) ---
for entry in '127.0.0.0/8' '169.254.0.0/16' '100.64.0.0/10' '198.18.0.0/15' '::1/128' 'fe80::/10'; do
  if ! grep -qF "${entry}" "${SRC}"; then
    fail "known-critical deny CIDR ${entry} is missing from ${SRC}; deny-list membership must not change in this SAFE-accept lane"
  fi
done
pass "all known-critical deny CIDRs present verbatim"

# --- known-critical deny hostnames must remain verbatim ---
for entry in 'metadata' 'metadata.google.internal' 'instance-data' 'instance-data.ec2.internal' 'localhost' 'localhost.localdomain'; do
  if ! grep -qF "\"${entry}\"" "${SRC}"; then
    fail "known-critical deny hostname ${entry} is missing from ${SRC}; deny-list membership must not change in this SAFE-accept lane"
  fi
done
pass "all known-critical deny hostnames present verbatim"

echo "All sonar_ko_07 deny CIDR comment tests passed."
