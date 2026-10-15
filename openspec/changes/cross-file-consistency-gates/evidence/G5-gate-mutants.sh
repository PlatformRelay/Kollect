#!/usr/bin/env bash
# Hand mutants of hack/test/consistency_metrics_test.sh (G5, task 5.2): each must make --self-test
# exit non-zero. Restores the gate from the first saved copy on exit.
# usage: bash G5-gate-mutants.sh   (from anywhere; edits the gate in place, then restores it)
set -uo pipefail
cd "$(dirname "$0")/../../../.." || exit 1
G=hack/test/consistency_metrics_test.sh
SAVE="$(mktemp)"
cp "$G" "$SAVE"
trap 'cp "$SAVE" "$G"; rm -f "$SAVE"' EXIT
killed=0 survived=0 notlanded=0
run() {
  local label="$1" old="$2" new="$3" out rc
  cp "$SAVE" "$G"
  if ! python3 -c 'import sys; p,o,n=sys.argv[1:]; s=open(p).read(); assert o in s, o; open(p,"w").write(s.replace(o,n,1))' "$G" "$old" "$new"; then
    echo "MUTANT-NOT-LANDED $label"
    notlanded=$((notlanded + 1))
    return
  fi
  out="$(bash "$G" --self-test 2>&1)"
  rc=$?
  if [[ "$rc" -ne 0 ]]; then killed=$((killed + 1)); else survived=$((survived + 1)); fi
  echo "$label exit=$rc :: $(grep FAIL <<<"$out" | head -1)"
}
run "catalog counted as registration" 'and not p.endswith("/metrics_catalog.go")' ''
run "test files counted" 'if not p.endswith("_test.go") and' 'if'
run "comments counted" '        if src.startswith("//", i):' '        if False:'
run "rules file not checked" 'check_names(RULES)' 'pass'
run "docs completeness not checked" '    errors.append(f"registered metric {n} is missing from {DOCS}")' '    pass'
run "any suffix on any registered base" '        if base in histograms:' '        if base in registered:'
run "suffix list widened" 'SUFFIXES = ("_bucket", "_sum", "_count")' 'SUFFIXES = ("_bucket", "_sum", "_count", "_created")'
run "mirror missing names not checked" '    for n in sorted(registered - mirror):' '    for n in []:'
run "mirror extra names not checked" '    for n in sorted(mirror - registered):' '    for n in []:'
run "empty source scan passes" 'if not registered:' 'if False:'
run "empty docs/rules scan passes" '    if not names:' '    if False:'
run "Namespace split allowed" 'm = re.search(r"\b(Namespace|Subsystem)\s*:", masked)' 'm = None'
run "outside registration allowed" '                    if CLIENT_PKGS & set(mask_go(fh.read())[1]):' '                    if False:'
run "missing file read as empty" '        print(f"FAIL: {rel} is missing", file=sys.stderr)
        sys.exit(1)' '        return ""'
run "names read as their lowercase prefix" '    names = set(TOKEN.findall(read(rel)))' '    names = set(NAME.findall(read(rel)))'
run "non-literal mirror entries dropped" '    if rest:' '    if False:'
run "imports matched double-quoted only" '                    if CLIENT_PKGS & set(mask_go(fh.read())[1]):' '                    if '"'"'"github.com/prometheus/client_golang/prometheus"'"'"' in fh.read():'
cp "$SAVE" "$G"
echo "killed=$killed survived=$survived not-landed=$notlanded"
bash "$G" >/dev/null || { echo "FAIL: the restored gate does not pass"; exit 1; }
echo "restored gate passes"
[[ "$survived" -eq 0 && "$notlanded" -eq 0 ]] || { echo "FAIL: survived or not-landed mutants"; exit 1; }
