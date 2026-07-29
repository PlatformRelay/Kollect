#!/usr/bin/env bash
# Meta test for SEC-04a (KO-01, SonarCloud pythonsecurity:S2083).
#
# SonarCloud flags hack/gen-glossary.py for writing to a "computed path"
# (path traversal / insecure file write path). This is a SAFE accept: the
# destination is a module-level constant derived only from the script's own
# location (ROOT = Path(__file__).resolve().parents[1]), never from argv or
# the environment. This test proves that property holds so the finding can
# stay marked SAFE/false-positive in SonarCloud without silently regressing
# if someone "fixes" it into something that actually reads argv/env later.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT}/hack/gen-glossary.py"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

[[ -f "${SCRIPT}" ]] || fail "expected ${SCRIPT} to exist"

# --- the GLOSSARY constant must be defined exactly as the reviewed-safe form ---
GLOSSARY_LINE="$(grep -E '^GLOSSARY[[:space:]]*=' "${SCRIPT}" || true)"
[[ -n "${GLOSSARY_LINE}" ]] || fail "no module-level GLOSSARY constant found in ${SCRIPT}"

EXPECTED_LINE='GLOSSARY = ROOT / "docs" / "GLOSSARY.md"'
if [[ "${GLOSSARY_LINE}" != "${EXPECTED_LINE}" ]]; then
  fail "GLOSSARY constant changed from the reviewed-safe form; got: ${GLOSSARY_LINE}"
fi
pass "GLOSSARY constant is exactly \`${EXPECTED_LINE}\`"

# --- the constant definition line itself must not reference argv/env ---
if echo "${GLOSSARY_LINE}" | grep -Eq 'sys\.argv|os\.environ|os\.getenv'; then
  fail "GLOSSARY constant line references argv/env; no longer a SAFE accept"
fi
pass "GLOSSARY constant line does not reference argv/env"

# --- the whole script must not derive any write target from argv/env ---
# (guards against a future edit reassigning GLOSSARY elsewhere from user
# input). Matches actual usage syntax (subscript/call), not prose mentions
# of the names in comments (e.g. this file's own SAFE-accept comment).
if grep -Eq 'sys\.argv\[|os\.environ\[|os\.environ\.get\(|os\.getenv\(' "${SCRIPT}"; then
  fail "${SCRIPT} uses sys.argv/os.environ/os.getenv; SAFE disposition no longer holds"
fi
pass "script has no argv/env usage"

# --- a reviewed SAFE/false-positive marker must be present near the constant ---
if ! grep -qi 'safe' "${SCRIPT}" || ! grep -qi 's2083' "${SCRIPT}"; then
  fail "expected a SAFE/false-positive comment referencing S2083 near the GLOSSARY constant"
fi
pass "SAFE/false-positive S2083 comment present"

echo "All sonar_ko_01 glossary path constant tests passed."
