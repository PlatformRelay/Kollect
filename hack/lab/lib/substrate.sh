#!/usr/bin/env bash
# Lab substrate allowlist + image-delivery policy (LAB-DEKIND). Source this file; do not execute.
# SPDX-License-Identifier: MIT
# shellcheck shell=bash
#
# WHY THIS EXISTS
#   Lab tooling port-forwards, installs Helm values and applies load. Pointing it at the
#   wrong cluster is destructive, and a maintainer's ambient kube context is frequently a
#   production cluster. The gate is therefore an ALLOWLIST with DEFAULT-DENY semantics:
#   a context is refused unless hack/lab/substrates.conf (or an explicitly validated
#   KOLLECT_LAB_ALLOWED_CONTEXTS addition) names it. There is no "allow everything" value.
#
# Exit/return contract:
#   lab_substrate_assert_context  0 = allowed, 2 = refused (callers exit 2)
#   lab_substrate_resolve         0 = allowed (prints substrate kind), 1 = refused
#   lab_substrate_load            0 = allowlist parsed, 1 = unusable/unsafe allowlist (fail closed)

_LAB_SUBSTRATE_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

: "${LAB_SUBSTRATE_LOG_PREFIX:=[lab-substrate]}"

lab_substrate_log() { printf '%s %s\n' "${LAB_SUBSTRATE_LOG_PREFIX}" "$*"; }
lab_substrate_warn() { printf '%s WARN: %s\n' "${LAB_SUBSTRATE_LOG_PREFIX}" "$*" >&2; }
lab_substrate_err() { printf '%s FAIL: %s\n' "${LAB_SUBSTRATE_LOG_PREFIX}" "$*" >&2; }

LAB_SUBSTRATE_PATTERNS=()
LAB_SUBSTRATE_KINDS=()
LAB_SUBSTRATE_CLUSTERS=()
LAB_SUBSTRATE_LOADED=0
LAB_SUBSTRATE_MATCHED_KIND=""
LAB_SUBSTRATE_MATCHED_CLUSTER=""

lab_substrate_config_path() {
  if [[ -n "${KOLLECT_LAB_SUBSTRATES_FILE:-}" ]]; then
    printf '%s' "${KOLLECT_LAB_SUBSTRATES_FILE}"
    return 0
  fi
  printf '%s' "${_LAB_SUBSTRATE_LIB_DIR}/../substrates.conf"
}

# A pattern is safe only if it is an exact context name or a single TRAILING '*' with at
# least 4 literal characters. '*', '**', '*-prod' and 'k*' are all rejected: an over-broad
# pattern turns the gate into a hole.
lab_substrate_valid_pattern() {
  local pat="${1:-}"
  [[ -n "${pat}" ]] || return 1
  local stars="${pat//[^*]/}"
  ((${#stars} <= 1)) || return 1
  if [[ "${pat}" == *'*'* && "${pat}" != *'*' ]]; then
    return 1
  fi
  local literal="${pat%\*}"
  ((${#literal} >= 4)) || return 1
  [[ "${literal}" =~ ^[A-Za-z0-9][A-Za-z0-9._:@/-]*$ ]] || return 1
  return 0
}

lab_substrate_valid_kind() {
  case "${1:-}" in
    kind | talos | generic) return 0 ;;
    *) return 1 ;;
  esac
}

_lab_substrate_add() {
  local pat="$1" kind="$2" cluster="$3" src="$4"
  if ! lab_substrate_valid_pattern "${pat}"; then
    lab_substrate_err "refusing unsafe context pattern '${pat}' from ${src}: want an exact context name or one trailing '*' after >= 4 literal characters"
    return 1
  fi
  if ! lab_substrate_valid_kind "${kind}"; then
    lab_substrate_err "invalid substrate '${kind}' for pattern '${pat}' from ${src} (want kind|talos|generic)"
    return 1
  fi
  LAB_SUBSTRATE_PATTERNS+=("${pat}")
  LAB_SUBSTRATE_KINDS+=("${kind}")
  LAB_SUBSTRATE_CLUSTERS+=("${cluster}")
  return 0
}

# Parse the checked-in allowlist plus KOLLECT_LAB_ALLOWED_CONTEXTS. Any unsafe or malformed
# entry fails the whole load — a partially-parsed allowlist is not a safety boundary.
#
# The parser runs with pathname expansion DISABLED. Every individual expansion below is
# already quoted or read-split, but this makes the property structural: no future edit inside
# the parser can turn an allowlist pattern into a list of filenames from the caller's CWD.
lab_substrate_load() {
  local rc=0 restore=0
  if [[ "$-" != *f* ]]; then
    set -f
    restore=1
  fi
  _lab_substrate_load_impl || rc=$?
  if [[ "${restore}" -eq 1 ]]; then
    set +f
  fi
  return "${rc}"
}

_lab_substrate_load_impl() {
  LAB_SUBSTRATE_PATTERNS=()
  LAB_SUBSTRATE_KINDS=()
  LAB_SUBSTRATE_CLUSTERS=()
  LAB_SUBSTRATE_LOADED=0

  local file
  file="$(lab_substrate_config_path)"
  if [[ ! -f "${file}" ]]; then
    lab_substrate_err "substrate allowlist not found: ${file} (refusing every context)"
    return 1
  fi

  local line pat kind cluster rest
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%%#*}"
    pat=""
    kind=""
    cluster=""
    rest=""
    read -r pat kind cluster rest <<<"${line}" || true
    [[ -n "${pat}" ]] || continue
    if [[ -n "${rest}" ]]; then
      lab_substrate_err "malformed allowlist entry in ${file}: '${line}' (want '<pattern> <substrate> [cluster]')"
      return 1
    fi
    _lab_substrate_add "${pat}" "${kind:-generic}" "${cluster}" "${file}" || return 1
  done <"${file}"

  local extra="${KOLLECT_LAB_ALLOWED_CONTEXTS:-}"
  if [[ -n "${extra}" ]]; then
    # `for item in ${extra}` would be subject to PATHNAME EXPANSION, not just word
    # splitting: from a directory containing a file named after a production context, the
    # shell would rewrite `*` into that filename BEFORE the validator ever saw it and the
    # allowlist would fail OPEN. Split with `read -a`, which never globs, and iterate the
    # array quoted. Never reintroduce an unquoted expansion here.
    local -a items=()
    IFS=', ' read -r -a items <<<"${extra}"
    local item
    for item in "${items[@]}"; do
      [[ -n "${item}" ]] || continue
      IFS='=' read -r pat kind cluster rest <<<"${item}"
      if [[ -n "${rest:-}" ]]; then
        lab_substrate_err "malformed KOLLECT_LAB_ALLOWED_CONTEXTS entry '${item}' (want 'pattern[=substrate[=cluster]]')"
        return 1
      fi
      _lab_substrate_add "${pat}" "${kind:-generic}" "${cluster:-}" "KOLLECT_LAB_ALLOWED_CONTEXTS" || return 1
    done
  fi

  LAB_SUBSTRATE_LOADED=1
  return 0
}

lab_substrate_allowlist_summary() {
  if [[ "${LAB_SUBSTRATE_LOADED}" -ne 1 ]]; then
    # Never present a partially-parsed list as if it were the allowlist: a load that failed
    # closed admits NOTHING, and saying so is the honest answer inside a refusal message.
    if ! lab_substrate_load >/dev/null 2>&1; then
      printf '<allowlist failed to load — nothing is permitted>'
      return 0
    fi
  fi
  local i out=""
  for ((i = 0; i < ${#LAB_SUBSTRATE_PATTERNS[@]}; i++)); do
    out+="${out:+, }${LAB_SUBSTRATE_PATTERNS[i]}(${LAB_SUBSTRATE_KINDS[i]})"
  done
  printf '%s' "${out:-<empty>}"
}

# Print the substrate kind for an allowlisted context; return 1 when the context is refused.
lab_substrate_resolve() {
  local ctx="${1:-}"
  LAB_SUBSTRATE_MATCHED_KIND=""
  LAB_SUBSTRATE_MATCHED_CLUSTER=""
  if [[ "${LAB_SUBSTRATE_LOADED}" -ne 1 ]]; then
    lab_substrate_load || return 1
  fi
  [[ -n "${ctx}" ]] || return 1

  local i pat
  for ((i = 0; i < ${#LAB_SUBSTRATE_PATTERNS[@]}; i++)); do
    pat="${LAB_SUBSTRATE_PATTERNS[i]}"
    # The allowlist entry IS the glob, so the right-hand side must stay unquoted. Quoting it
    # would make 'kind-*' a literal context name and refuse every real Kind cluster. The
    # pattern is validated above (exact name, or one trailing '*' after >= 4 literal chars),
    # so this cannot widen into a match-everything wildcard.
    # shellcheck disable=SC2053
    if [[ "${ctx}" == $pat ]]; then
      LAB_SUBSTRATE_MATCHED_KIND="${LAB_SUBSTRATE_KINDS[i]}"
      LAB_SUBSTRATE_MATCHED_CLUSTER="${LAB_SUBSTRATE_CLUSTERS[i]}"
      printf '%s' "${LAB_SUBSTRATE_MATCHED_KIND}"
      return 0
    fi
  done
  return 1
}

# Optional confirmation only: a cluster-name mismatch REFUSES, it never admits.
_lab_substrate_confirm_cluster() {
  local ctx="$1" expected="$2"
  [[ -n "${expected}" ]] || return 0
  command -v kubectl >/dev/null 2>&1 || {
    lab_substrate_warn "kubectl unavailable; cannot confirm cluster '${expected}' for context ${ctx}"
    return 0
  }
  local actual
  actual="$(kubectl config view -o "jsonpath={.contexts[?(@.name==\"${ctx}\")].context.cluster}" 2>/dev/null || true)"
  if [[ -z "${actual}" ]]; then
    lab_substrate_warn "could not read the cluster of context ${ctx}; proceeding on the context-name match"
    return 0
  fi
  if [[ "${actual}" != "${expected}" ]]; then
    lab_substrate_err "context '${ctx}' points at cluster '${actual}', not the expected lab cluster '${expected}' — refusing"
    return 1
  fi
  return 0
}

# Gate entry point. Returns 0 when allowed, 2 when refused.
# Pass --offline to skip the live kubectl cluster confirmation (fixtures / meta-tests).
lab_substrate_assert_context() {
  local ctx="${1:-}"
  local offline=0
  [[ "${2:-}" == "--offline" ]] && offline=1

  if [[ -z "${ctx}" ]]; then
    lab_substrate_err "ambiguous kube context: no current context (refusing; set one explicitly)"
    return 2
  fi

  local kind
  if ! kind="$(lab_substrate_resolve "${ctx}")"; then
    lab_substrate_err "refusing kube context '${ctx}': not on the lab substrate allowlist [$(lab_substrate_allowlist_summary)]"
    lab_substrate_err "default-deny — add the context to $(lab_substrate_config_path) or KOLLECT_LAB_ALLOWED_CONTEXTS if it really is a lab cluster"
    return 2
  fi

  if [[ "${offline}" -eq 0 ]] && ! _lab_substrate_confirm_cluster "${ctx}" "${LAB_SUBSTRATE_MATCHED_CLUSTER}"; then
    return 2
  fi

  lab_substrate_log "kube context ok: ${ctx} (substrate=${kind})"
  return 0
}

# How images reach the nodes on a given substrate.
lab_substrate_image_delivery() {
  case "${1:-}" in
    kind) printf 'sideload' ;;
    *) printf 'registry' ;;
  esac
  return 0
}

# Non-Kind substrates have no `kind load docker-image` equivalent: the image MUST come from
# a registry at an immutable reference. Refusing loudly here is the whole point — a silent
# fallback to whatever tag the nodes already cached is how a stale-image run gets published
# as evidence.
lab_substrate_require_registry_image() {
  local image="${1:-}" substrate="${2:-generic}"
  # Only recommend a form the install path can actually use. The chart renders
  # `repository:tag` (charts/kollect/templates/_helpers.tpl "kollect.image"), so a digest
  # reference is NOT installable — do not suggest one here and then reject it in
  # kollect_helm_install two calls later.
  local hint="set KOLLECT_IMAGE=ghcr.io/platformrelay/kollect:v<semver> and push it before running"

  if [[ -z "${image}" ]]; then
    lab_substrate_err "no image configured for a ${substrate} substrate; ${hint}"
    return 1
  fi

  local repo tag="" digest=""
  if [[ "${image}" == *"@"* ]]; then
    repo="${image%%@*}"
    digest="${image#*@}"
  elif [[ "${image##*/}" == *:* ]]; then
    repo="${image%:*}"
    tag="${image##*:}"
  else
    repo="${image}"
  fi

  local host="${repo%%/*}"
  if [[ "${repo}" != */* ]] || { [[ "${host}" != *.* ]] && [[ "${host}" != *:* ]] && [[ "${host}" != "localhost" ]]; }; then
    lab_substrate_err "image '${image}' is not registry-qualified: a ${substrate} cluster cannot side-load a local image (no 'kind load' equivalent); ${hint}"
    return 1
  fi

  if [[ -n "${digest}" ]]; then
    # A digest is the strongest pin, but the kollect chart cannot render one, so accepting it
    # here would only defer the failure to helm. Refuse it where the operator can act on it.
    lab_substrate_err "image '${image}' is digest-pinned: the kollect chart renders repository:tag and cannot install a digest; ${hint}"
    return 1
  fi

  if [[ -z "${tag}" ]]; then
    lab_substrate_err "image '${image}' has no tag: an untagged image resolves to a mutable 'latest' on a ${substrate} cluster; ${hint}"
    return 1
  fi

  if [[ ! "${tag}" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([-+.][0-9A-Za-z.-]+)?$ ]]; then
    lab_substrate_err "image tag '${tag}' is not an immutable release tag (want a pinned v<semver> or a @sha256 digest); ${hint}"
    return 1
  fi
  return 0
}
