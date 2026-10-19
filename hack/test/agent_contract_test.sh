#!/usr/bin/env bash
# The public agent contract (openspec change public-agent-contract, PAC-1 to PAC-6).
#
# A fresh clone gives AI coding assistants and new contributors one tracked AGENTS.md. This guard
# keeps it present, true and safe to publish:
#   PAC-1  AGENTS.md is tracked; every `task <name>` in its code spans and fences is defined
#          (`task --list-all`); the gate list between <!-- gates:start --> and <!-- gates:end -->
#          names exactly the gates hack/check.sh passes to run_gate; the required commands are
#          there; it links CONTRIBUTING.md and docs/development/spec-workflow.md.
#   PAC-2  .github/copilot-instructions.md is tracked, at most 3 lines, points to AGENTS.md and
#          names no missing path; CLAUDE.md is NOT tracked (it is the maintainer's ignored local
#          loader, and git replaces an ignored file with a tracked one on pull).
#   PAC-3  the three checklists exist under their headings; every path in them matches a tracked
#          file (a <placeholder> segment reads as *); every Go identifier on a line with .go paths
#          is declared in one of them; the metric checklist names every place a metric touches.
#   PAC-4  the agent files, on disk AND in the index, match no line of hack/scrub-patterns.txt
#          (`rg -i -f`, the semantics of hack/scrub.sh; an rg error or a missing rg fails), name
#          no home path and no gitignored harness path, carry no private-file header and no
#          OpenSpec marker; .gitignore does not ignore AGENTS.md and does ignore the private files
#          (checked with `git -c core.excludesFile=/dev/null check-ignore --no-index` in a probe
#          repository that holds only .gitignore); no gitleaks allowlist path regex exempts an
#          agent file.
#   PAC-6  the agent files name no internal host or private address (a generic pattern set) and
#          not the short name of the machine running the check (`hostname -s`, or
#          AGENT_CONTRACT_HOSTNAME; skipped with a printed reason when it cannot be checked
#          without false reds).
#   PAC-5  --self-test runs one mutant per clause, each on its own throwaway copy of every tracked
#          file (a mktemp -d directory that is its own git repository), and requires a red that
#          carries the diagnostic of the clause it targets; an unmodified copy must pass.
#
# Every failure is collected before the exit, so a mutant that breaks two clauses still reds with
# the message of the clause it names. No scrub pattern and no hostname is written in this file:
# the mutants read hack/scrub-patterns.txt and build host names at run time.
#
# Usage:
#   hack/test/agent_contract_test.sh              # check the repository tree
#   hack/test/agent_contract_test.sh --self-test  # mutate copies, assert each clause can fail
# Environment:
#   AGENT_CONTRACT_HOSTNAME  the running-host name to refuse (set empty to skip that check)
#   AGENT_CONTRACT_NESTED=1  set by the self-test for the nested self-tests it starts; skips the
#                            mutants that would start another one
#   AGENT_CONTRACT_ONLY      a self-test runs only the mutants whose label contains this text
#                            (the nested self-tests use it to stay fast)
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="check"
if [[ "${1:-}" == "--self-test" ]]; then
  MODE="self-test"
fi

FAILURES=0
bad() {
  echo "FAIL: $*" >&2
  FAILURES=$((FAILURES + 1))
}
ok() {
  echo "ok - $*"
}

SCRATCH=()
cleanup() {
  local d
  for d in "${SCRATCH[@]}"; do
    rm -rf "${d:?}"
  done
}
trap cleanup EXIT

AGENTS="AGENTS.md"
COPILOT=".github/copilot-instructions.md"
PATTERNS_FILE="hack/scrub-patterns.txt"
REQUIRED_TASKS=(build test lint verify check scrub lint:markdown spec:validate)
CHECKLISTS=("### When adding a CRD field" "### When adding a metric" "### When adding a sink")
METRIC_ENTRIES=(internal/metrics/metrics.go Catalog internal/metrics/metrics_catalog.go
  registeredMetricNames internal/metrics/metrics_catalog_test.go docs/operator-manual/metrics.md)
HARNESS_PATHS=(agent-context/ .claude/ .agents/ .cursor/ AGENTS.local.md worktrees/)
MUST_IGNORE=(CLAUDE.md CLAUDE.local.md AGENTS.local.md agent-context/x .claude/x .agents/x .cursor/x)
LEAK_PATHS=(AGENTS.md CLAUDE.md .github/copilot-instructions.md)

# The PAC-6 generic pattern set: name=perl regex, matched case-insensitively per line.
OCTET='(?:25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])'
GENERIC_PATTERNS=(
  'internal DNS name=(?<![a-z0-9.-])(?!(?:[a-z0-9-]+\.)*cluster\.local(?![a-z0-9-]|\.[a-z0-9]))(?:[a-z0-9-]+\.)+(?:internal|local|lan|home\.arpa)(?![a-z0-9-]|\.[a-z0-9])'
  "private IPv4 address=(?<![0-9.])(?:10\.${OCTET}\.${OCTET}\.${OCTET}|172\.(?:1[6-9]|2[0-9]|3[01])\.${OCTET}\.${OCTET}|192\.168\.${OCTET}\.${OCTET}|169\.254\.${OCTET}\.${OCTET}|100\.(?:6[4-9]|[7-9][0-9]|1[01][0-9]|12[0-7])\.${OCTET}\.${OCTET})(?![0-9]|\.[0-9])"
  'unique-local IPv6 address=(?<![0-9a-z:])f[cd][0-9a-f]{2}:[0-9a-f]{0,4}:[0-9a-f:]*'
  'SSH Host directive=^\s*host(?:\s*=\s*|\s+)[a-z0-9*?._-]+(?:\s+[a-z0-9*?._-]+){0,2}\s*(?:#.*)?$'
  'scp-style address=(?:^|(?<=[\s(<\x22\x27\x60]))(?!git@github\.com:)[a-z0-9._-]+@(?!sha(?:256|384|512):)[a-z0-9-]+(?:\.[a-z0-9-]+)*:(?=[^\s0-9])'
)

need() {
  command -v "$1" >/dev/null 2>&1
}

# --- helpers -------------------------------------------------------------------------------------

# scrub_match <file>: 0 = a scrub pattern matches, 1 = none, 2 = rg error. The same rg flags as
# hack/scrub.sh (case-insensitive regular expressions, substring match).
scrub_match() {
  local rc
  rg -i -q -f "${ROOT}/${PATTERNS_FILE}" -- "$1" 2>/dev/null # scrub-rg
  rc=$?
  return "${rc}"
}

# code_task_names <file>: every `task <name>` in a code span or fence, one name per line.
code_task_names() {
  perl -e '
    local $/; my $t = <>; my @code;
    my $fence = 0;
    for my $l (split /\n/, $t) {
      if ($l =~ /^\s*(```|~~~)/) { $fence = !$fence; next; }
      if ($fence) { push @code, $l; next; }
      while ($l =~ /(`+)(.+?)\1/g) { push @code, $2; }
    }
    for my $c (@code) {
      for my $cmd (split /&&|\|\||;|\||\$\(|\(/, $c) {
        $cmd =~ s/^\s+//; $cmd =~ s/^\$\s+//;
        while ($cmd =~ s/^[A-Za-z_][A-Za-z0-9_]*=\S*\s+//) {}
        my @w = split /\s+/, $cmd;
        next unless @w && $w[0] eq "task";
        shift @w;
        while (@w && ($w[0] =~ /^-/ || $w[0] =~ /^[A-Za-z_][A-Za-z0-9_]*=/)) { shift @w; }
        print "$w[0]\n" if @w && $w[0] =~ /^[A-Za-z0-9][\w:.-]*$/;
      }
    }' "$1"
}

# gate_list <file>: the code spans between the gate markers; prints MISSING-START / MISSING-END.
gate_list() {
  perl -e '
    my ($in, $s, $e) = (0, 0, 0);
    while (my $l = <>) {
      chomp $l; (my $x = $l) =~ s/^\s+|\s+$//g;
      if ($x eq "<!-- gates:start -->") { $in = 1; $s = 1; next; }
      if ($x eq "<!-- gates:end -->") { $in = 0; $e = 1; next; }
      next unless $in;
      while ($l =~ /(`+)(.+?)\1/g) { (my $g = $2) =~ s/^\s+|\s+$//g; print "$g\n"; }
    }
    print "MISSING-START\n" unless $s; print "MISSING-END\n" unless $e;' "$1"
}

# check_gates: the run_gate names in hack/check.sh, comment lines aside.
check_gates() {
  grep -v '^[[:space:]]*#' "${ROOT}/hack/check.sh" | awk '$1 == "run_gate" { print $2 }'
}

# checklist_findings <agents> <tracked-list>: PAC-3 findings, one per line.
checklist_findings() {
  perl -e '
    my ($agents, $tracked, $root, $metric) = @ARGV;
    my @want = ("### When adding a CRD field", "### When adding a metric", "### When adding a sink");
    my @files;
    open my $tf, "<", $tracked or die; while (<$tf>) { chomp; push @files, $_ if -e "$root/$_"; } close $tf;
    my (%sec, %seen, $cur); my $fence = 0;
    open my $af, "<", $agents or die;
    while (my $l = <$af>) {
      chomp $l;
      if ($l =~ /^\s*(```|~~~)/) { $fence = !$fence; next; }
      next if $fence;
      if ($l =~ /^(#{1,6})\s+(.*?)\s*$/ && length($1) <= 3) {
        my $h = "$1 $2"; $cur = undef;
        for my $w (@want) { if ($h eq $w) { $cur = $w; $seen{$w} = 1; } }
        next;
      }
      push @{$sec{$cur}}, $l if defined $cur;
    }
    for my $w (@want) { print "heading\t$w\n" unless $seen{$w}; }
    my %decl;
    sub decls { my $f = shift; return $decl{$f} if $decl{$f}; my %d; my $blk = 0;
      open my $g, "<", "$root/$f" or return {};
      while (my $x = <$g>) {
        if ($x =~ /^(?:var|const|type)\s*\(\s*$/) { $blk = 1; next; }
        if ($blk && $x =~ /^\)/) { $blk = 0; next; }
        if ($blk && $x =~ /^\s+([A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)/) { $d{$_} = 1 for split /\s*,\s*/, $1; next; }
        if ($x =~ /^func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)/) { $d{$1} = 1; }
        if ($x =~ /^type\s+([A-Za-z_]\w*)/) { $d{$1} = 1; }
        if ($x =~ /^(?:var|const)\s+([A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)/) { $d{$_} = 1 for split /\s*,\s*/, $1; }
      }
      close $g; return $decl{$f} = \%d; }
    my %metric_spans;
    for my $w (@want) {
      for my $l (@{$sec{$w} || []}) {
        my @spans; while ($l =~ /(`+)(.+?)\1/g) { (my $s = $2) =~ s/^\s+|\s+$//g; push @spans, $s; }
        my (@gofiles, @syms);
        for my $s (@spans) {
          $metric_spans{$s} = 1 if $w eq $metric;
          if ($s =~ /^([A-Za-z_]\w*)(?:\(\))?$/) { push @syms, $1; next; }
          for my $tok (split /\s+/, $s) {
            next unless $tok =~ m{/} && $tok !~ m{://};
            (my $p = $tok) =~ s/[#:].*$//; $p =~ s{^\./}{}; $p =~ s{/\.\.\.$}{}; $p =~ s{/+$}{};
            next if $p eq "";
            my $re = join "/", map { my $seg = $_; $seg =~ s/<[^>]*>/*/g;
              join "", map { $_ eq "*" ? "[^/]*" : $_ eq "?" ? "[^/]" : quotemeta $_ } split /(\*|\?)/, $seg } split m{/}, $p;
            my @m = grep { m{^$re(?:/.*)?$} } @files;
            if (!@m) { print "path\t$tok\n"; next; }
            push @gofiles, grep { /\.go$/ } @m if $p =~ /\.go$/;
          }
        }
        next unless @gofiles;
        for my $y (@syms) {
          next if grep { decls($_)->{$y} } @gofiles;
          print "symbol\t$y\t" . join(" ", @gofiles) . "\n";
        }
      }
    }
    for my $m (split / /, $ENV{METRIC_ENTRIES}) { print "metric\t$m\n" unless $metric_spans{$m}; }
  ' "$1" "$2" "${ROOT}" "### When adding a metric"
}

# generic_findings <file>: "<name> line <n>" for every PAC-6 generic match.
generic_findings() {
  local p
  for p in "${GENERIC_PATTERNS[@]}"; do
    printf '%s\n' "${p}"
  done | perl -e '
    my @p = map { chomp; [split /=/, $_, 2] } grep { /=/ } <STDIN>;
    open my $f, "<", $ARGV[0] or exit 0; my $n = 0;
    while (my $l = <$f>) { $n++; chomp $l;
      for my $q (@p) { print "$q->[0] line $n\n" if $l =~ /$q->[1]/i; } }' "$1"
}

# --- PAC-6 running host: decide once whether the check is active ---------------------------------
HOST_NAME=""
host_check_setup() {
  local name reason=""
  if [[ -n "${AGENT_CONTRACT_HOSTNAME+set}" ]]; then
    name="${AGENT_CONTRACT_HOSTNAME}"
  else
    name="$(hostname -s 2>/dev/null || true)"
  fi
  if [[ -z "${name}" ]]; then
    reason="the host name is empty or cannot be read"
  elif [[ "${#name}" -lt 4 ]]; then
    reason="the host name is shorter than 4 characters"
  elif [[ "${name,,}" == "localhost" ]]; then
    reason="the host name is localhost"
  elif printf '%s\n' "${TASK_NAMES}" | grep -ixF -- "${name}" >/dev/null; then
    reason="the host name is a task name"
  elif need rg && [[ -n "$(git -C "${ROOT}" ls-files -z -- '*.md' |
    grep -zvxF -e "${AGENTS}" -e "${COPILOT}" |
    (cd "${ROOT}" && xargs -0 rg -i -w -F -l -- "${name}") 2>/dev/null)" ]]; then
    reason="the host name is an ordinary word in tracked Markdown"
  fi
  if [[ -n "${reason}" ]]; then
    echo "PAC-6: skip running-host check: ${reason}"
    return
  fi
  HOST_NAME="${name}"
  echo "PAC-6: running-host check active"
}

# content_checks <label> <file>: PAC-4 and PAC-6 checks on one version of one agent file.
content_checks() {
  local label="$1" f="$2" rc h finding
  if ((HAVE_RG && HAVE_PATTERNS)); then
    scrub_match "${f}"
    rc=$?
    case "${rc}" in
    0) bad "PAC-4: ${label} matches a scrub pattern (hack/scrub-patterns.txt)" ;;
    1) : ;;
    *) bad "PAC-4: rg failed (exit ${rc}) while scrubbing ${label}" ;;
    esac
  fi
  if grep -E '(^|[^A-Za-z0-9_.~-])/(home|Users)/[A-Za-z0-9._-]+' "${f}" >/dev/null; then
    bad "PAC-4: ${label} names a home path"
  fi
  if grep -E '(^|[^A-Za-z0-9_])~/' "${f}" >/dev/null; then
    bad "PAC-4: ${label} names a ~/ path"
  fi
  for h in "${HARNESS_PATHS[@]}"; do
    if grep -F -- "${h}" "${f}" >/dev/null; then
      bad "PAC-4: ${label} names the gitignored harness path '${h}'"
    fi
  done
  if head -n 10 "${f}" | grep -Ei 'local[ -]only|never[ -]commit' >/dev/null; then
    bad "PAC-4: ${label} carries a private-file header (first 10 lines)"
  fi
  if grep -Ei '<!--[[:space:]]*openspec:(start|end)[[:space:]]*-->' "${f}" >/dev/null; then
    bad "PAC-4: ${label} carries an OpenSpec marker"
  fi
  while IFS= read -r finding; do
    [[ -n "${finding}" ]] && bad "PAC-6: ${label} names an internal host or address (${finding})"
  done < <(generic_findings "${f}")
  if [[ -n "${HOST_NAME}" ]] && grep -iwF -- "${HOST_NAME}" "${f}" >/dev/null; then
    bad "PAC-6: ${label} contains the running host's short name"
  fi
}

# pointer_path_ok <path relative to the root>: the normalised path stays inside the repository
# and is a tracked file (an ignored local file that exists on one machine does not count).
pointer_path_ok() {
  local norm
  norm="$(perl -e 'my @o; for (split m{/}, $ARGV[0]) { next if $_ eq "" || $_ eq "."; if ($_ eq "..") { exit 1 unless @o; pop @o; next } push @o, $_ } print join "/", @o' "$1")" || return 1
  [[ -n "${norm}" ]] && git ls-files --error-unmatch -- "${norm}" >/dev/null 2>&1
}

# --- the plain check -----------------------------------------------------------------------------
check_tree() {
  local tmp f name g line kind a b rc
  for name in git perl; do
    if ! need "${name}"; then
      bad "${name} not found"
      return
    fi
  done
  tmp="$(mktemp -d)"
  SCRATCH+=("${tmp}")
  cd "${ROOT}" || return

  HAVE_RG=1
  if ! need rg; then
    bad "PAC-4: rg not found (the scrub check needs it; install ripgrep)"
    HAVE_RG=0
  fi
  HAVE_PATTERNS=0
  if [[ ! -f "${PATTERNS_FILE}" ]]; then
    bad "PAC-4: scrub pattern list ${PATTERNS_FILE} is missing"
  elif ! grep -q '[^[:space:]]' "${PATTERNS_FILE}"; then
    bad "PAC-4: scrub pattern list ${PATTERNS_FILE} is empty"
  else
    HAVE_PATTERNS=1
  fi

  TASK_NAMES=""
  if ! need task; then
    bad "PAC-1: task not found (needed to read task --list-all)"
  elif ! TASK_NAMES="$(task --list-all --silent 2>/dev/null)" || [[ -z "${TASK_NAMES}" ]]; then
    bad "PAC-1: Taskfile could not be loaded (task --list-all failed)"
    TASK_NAMES=""
  fi
  host_check_setup

  git ls-files >"${tmp}/tracked"

  # PAC-1
  if ! git ls-files --error-unmatch -- "${AGENTS}" >/dev/null 2>&1; then
    bad "PAC-1: AGENTS.md is not tracked (git ls-files AGENTS.md is empty)"
  fi
  if [[ -f "${AGENTS}" ]]; then
    if [[ -n "${TASK_NAMES}" ]]; then
      while IFS= read -r name; do
        printf '%s\n' "${TASK_NAMES}" | grep -xF -- "${name}" >/dev/null ||
          bad "PAC-1: AGENTS.md names an undefined task '${name}' (not in task --list-all)"
      done < <(code_task_names "${AGENTS}" | sort -u)
    fi
    code_task_names "${AGENTS}" | sort -u >"${tmp}/named"
    for name in "${REQUIRED_TASKS[@]}"; do
      grep -xF -- "${name}" "${tmp}/named" >/dev/null ||
        bad "PAC-1: AGENTS.md does not name the required command 'task ${name}'"
    done
    gate_list "${AGENTS}" | sort -u >"${tmp}/listed"
    grep -qx 'MISSING-START' "${tmp}/listed" && bad "PAC-1: gate list: marker '<!-- gates:start -->' missing from AGENTS.md"
    grep -qx 'MISSING-END' "${tmp}/listed" && bad "PAC-1: gate list: marker '<!-- gates:end -->' missing from AGENTS.md"
    check_gates | sort -u >"${tmp}/gates"
    while IFS= read -r g; do
      grep -xF -- "${g}" "${tmp}/listed" >/dev/null ||
        bad "PAC-1: gate list: hack/check.sh runs gate '${g}' but AGENTS.md does not list it"
    done <"${tmp}/gates"
    while IFS= read -r g; do
      [[ "${g}" == MISSING-START || "${g}" == MISSING-END ]] && continue
      grep -xF -- "${g}" "${tmp}/gates" >/dev/null ||
        bad "PAC-1: gate list: AGENTS.md lists gate '${g}' that hack/check.sh does not run"
    done <"${tmp}/listed"
    grep -E '\]\((\./)?CONTRIBUTING\.md(#[^)]*)?\)' "${AGENTS}" >/dev/null ||
      bad "PAC-1: AGENTS.md does not link CONTRIBUTING.md"
    grep -E '\]\((\./)?docs/development/spec-workflow\.md(#[^)]*)?\)' "${AGENTS}" >/dev/null ||
      bad "PAC-1: AGENTS.md does not link docs/development/spec-workflow.md"

    # PAC-3
    while IFS=$'\t' read -r kind a b; do
      case "${kind}" in
      heading) bad "PAC-3: missing checklist heading '${a}'" ;;
      path) bad "PAC-3: checklist path '${a}' matches no tracked file" ;;
      symbol) bad "PAC-3: checklist symbol '${a}' is not declared in ${b}" ;;
      metric) bad "PAC-3: metric checklist does not name '${a}'" ;;
      esac
    done < <(METRIC_ENTRIES="${METRIC_ENTRIES[*]}" checklist_findings "${AGENTS}" "${tmp}/tracked")
  fi

  # PAC-2
  if [[ ! -f "${COPILOT}" ]]; then
    bad "PAC-2: ${COPILOT} is missing"
  else
    git ls-files --error-unmatch -- "${COPILOT}" >/dev/null 2>&1 || bad "PAC-2: ${COPILOT} is not tracked"
    line="$(awk 'END { print NR }' "${COPILOT}")"
    [[ "${line}" -le 3 ]] || bad "PAC-2: ${COPILOT} has ${line} lines (at most 3)"
    grep -E '(^|[^A-Za-z0-9_.-])AGENTS\.md' "${COPILOT}" >/dev/null || bad "PAC-2: ${COPILOT} does not point to AGENTS.md"
    while IFS= read -r a; do
      [[ -z "${a}" ]] && continue
      case "${a}" in
      *://* | mailto:* | '#'*) continue ;;
      /*) b="${a#/}" ;;
      *) b=".github/${a%%#*}" ;;
      esac
      pointer_path_ok "${b}" || bad "PAC-2: ${COPILOT} names a missing path '${a}' (not a tracked file in this repository)"
    done < <(perl -ne 'print "$1\n" while /\]\(([^)\s]+)\)/g' "${COPILOT}")
    while IFS= read -r a; do
      pointer_path_ok "${a}" || bad "PAC-2: ${COPILOT} names a missing path '${a}' (not a tracked file in this repository)"
    done < <(perl -ne 'while (/`([^`\s]+)`/g) { my $s = $1; print "$s\n" if $s =~ m{/|\.md$} && $s !~ m{://}; }' "${COPILOT}")
  fi
  if git ls-files --error-unmatch -- CLAUDE.md >/dev/null 2>&1; then
    bad "PAC-2: CLAUDE.md is tracked (it must stay the maintainer's ignored local loader)"
  fi

  # PAC-4 / PAC-6 content, disk and index
  for f in "${AGENTS}" "${COPILOT}"; do
    if [[ -f "${f}" ]]; then
      content_checks "${f} (disk)" "${f}"
    fi
    if git cat-file -e ":${f}" 2>/dev/null; then
      git show ":${f}" >"${tmp}/index-content"
      content_checks "${f} (index)" "${tmp}/index-content"
    fi
  done

  # PAC-4 ignore rules, against .gitignore only
  mkdir "${tmp}/probe"
  if git -C "${tmp}/probe" init -q 2>/dev/null; then
    : >"${tmp}/probe/.git/info/exclude"
    [[ -f .gitignore ]] && cp .gitignore "${tmp}/probe/.gitignore"
    git -C "${tmp}/probe" -c core.excludesFile=/dev/null check-ignore --no-index -q -- "${AGENTS}"
    rc=$?
    case "${rc}" in
    0) bad "PAC-4: .gitignore ignores the tracked AGENTS.md" ;;
    1) : ;;
    *) bad "PAC-4: git check-ignore failed (exit ${rc})" ;;
    esac
    for name in "${MUST_IGNORE[@]}"; do
      git -C "${tmp}/probe" -c core.excludesFile=/dev/null check-ignore --no-index -q -- "${name}"
      rc=$?
      case "${rc}" in
      0) : ;;
      1) bad "PAC-4: .gitignore does not ignore '${name%/x}'" ;;
      *) bad "PAC-4: git check-ignore failed (exit ${rc})" ;;
      esac
    done
  else
    bad "PAC-4: could not create the ignore-rule probe repository"
  fi

  # PAC-4 gitleaks allowlists
  if [[ ! -f .github/gitleaks.toml ]]; then
    bad "PAC-4: .github/gitleaks.toml is missing"
  else
    # A real TOML parser, so quoted keys, escapes, inline tables and dotted keys all count: every
    # "paths" list under any key named allowlist or allowlists, at any depth (tables, rules,
    # inline tables).
    python3 -I -c '
import sys, tomllib
try:
    with open(sys.argv[1], "rb") as f:
        doc = tomllib.load(f)
except Exception as e:
    print("parse error:", type(e).__name__, file=sys.stderr)
    sys.exit(3)
out = []
def walk(node, under):
    if isinstance(node, dict):
        for k, v in node.items():
            if under and k == "paths" and isinstance(v, list):
                out.extend(x for x in v if isinstance(x, str))
            walk(v, under or k in ("allowlist", "allowlists"))
    elif isinstance(node, list):
        for v in node:
            walk(v, under)
walk(doc, False)
for x in out:
    print(x.replace("\n", " "))
' .github/gitleaks.toml >"${tmp}/allow" 2>/dev/null
    rc=$?
    case "${rc}" in
    0) : ;;
    3) bad "PAC-4: .github/gitleaks.toml could not be parsed as TOML" ;;
    *) bad "PAC-4: python3 with tomllib not found or failed (exit ${rc}); it reads .github/gitleaks.toml" ;;
    esac
    while IFS= read -r a; do
      [[ -z "${a}" ]] && continue
      for name in "${LEAK_PATHS[@]}"; do
        printf '%s\n' "${name}" | grep -P -- "${a}" >/dev/null 2>&1
        rc=$?
        case "${rc}" in
        0) bad "PAC-4: .github/gitleaks.toml allowlist path regex '${a}' exempts ${name} from secret scanning" ;;
        1) : ;;
        *)
          bad "PAC-4: .github/gitleaks.toml allowlist path regex '${a}' could not be compiled"
          break
          ;;
        esac
      done
    done <"${tmp}/allow"
  fi
}

if [[ "${MODE}" == "check" ]]; then
  check_tree
  if ((FAILURES > 0)); then
    echo "agent contract: FAILED (${FAILURES} problem(s))" >&2
    exit 1
  fi
  ok "agent contract holds (PAC-1 to PAC-4, PAC-6)"
  exit 0
fi

# --- PAC-5: the self-test ------------------------------------------------------------------------

for name in git perl rg; do
  need "${name}" || {
    echo "FAIL: self-test needs ${name}" >&2
    exit 1
  }
done
BASE="$(mktemp -d)"
SCRATCH+=("${BASE}")
copy_git() {
  GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 git -c core.excludesFile=/dev/null \
    -c user.name=agent-contract -c user.email=agent-contract@example.invalid -c commit.gpgsign=false "$@"
}
(cd "${ROOT}" && git ls-files -z | tar --null -T - -cf -) | tar -C "${BASE}" -xf - || {
  echo "FAIL: self-test could not copy the tracked tree" >&2
  exit 1
}
(cd "${BASE}" && copy_git init -q && : >.git/info/exclude && copy_git add -A -f &&
  copy_git commit -q --no-verify -m base) || {
  echo "FAIL: self-test could not create the base repository" >&2
  exit 1
}

MUTLOG="$(mktemp)"
SCRATCH+=("${MUTLOG}")
MUT_ENV=()
MUTANT_RESULT=""
MUTANT_OUT=""
# run_mutant <label> <target> <command...>: runs the command in a fresh copy, then the copy's
# own guard in plain mode (MUT_ENV applied). Sets MUTANT_RESULT to killed / survived /
# wrong-reason / not-applied.
run_mutant() {
  local label="$1" target="$2" copy rc
  shift 2
  copy="$(mktemp -d)"
  SCRATCH+=("${copy}")
  cp -a "${BASE}/." "${copy}/"
  if ! (cd "${copy}" && "$@") >"${MUTLOG}" 2>&1; then
    MUTANT_RESULT="not-applied"
    MUTANT_OUT="$(head -3 "${MUTLOG}")"
    rm -rf "${copy:?}"
    return
  fi
  MUTANT_OUT="$(cd "${copy}" && env -u AGENT_CONTRACT_ONLY -u AGENT_CONTRACT_NESTED "${MUT_ENV[@]}" bash hack/test/agent_contract_test.sh 2>&1)"
  rc=$?
  rm -rf "${copy:?}"
  if [[ "${rc}" -eq 0 ]]; then
    MUTANT_RESULT="survived"
  elif grep -F -- "${target}" <<<"${MUTANT_OUT}" >/dev/null; then
    MUTANT_RESULT="killed"
  else
    MUTANT_RESULT="wrong-reason"
  fi
}

# failure_lines <output>: its FAIL lines, or its first lines when it has none
failure_lines() {
  grep FAIL <<<"$1" || head -3 <<<"$1"
}

selected() {
  [[ -z "${AGENT_CONTRACT_ONLY:-}" || "$1" == *"${AGENT_CONTRACT_ONLY}"* ]]
}

# expect_red <label> <target> <command...>: the mutant must red with its target diagnostic.
expect_red() {
  local label="$1" target="$2"
  selected "${label}" || {
    MUT_ENV=()
    return 0
  }
  run_mutant "$@"
  case "${MUTANT_RESULT}" in
  killed) ok "self-test: ${label} rejected with '${target}'" ;;
  survived) bad "self-test: mutant '${label}' survived (the plain run stayed green)" ;;
  wrong-reason) bad "self-test: mutant '${label}' red without its target diagnostic '${target}': $(head -2 <<<"${MUTANT_OUT}" | tr '\n' ' ')" ;;
  *) bad "self-test: mutant '${label}' was not applied, nothing was tested: ${MUTANT_OUT}" ;;
  esac
  MUT_ENV=()
}

# expect_green <label> <expected-output> <command...>: the control must pass, printing the text.
expect_green() {
  local label="$1" want="$2"
  selected "${label}" || {
    MUT_ENV=()
    return 0
  }
  run_mutant "$@"
  if [[ "${MUTANT_RESULT}" != "survived" ]]; then
    bad "self-test: control '${label}' did not pass (${MUTANT_RESULT}): $(failure_lines "${MUTANT_OUT}" | head -3 | tr '\n' ' ')"
  elif [[ -n "${want}" ]] && ! grep -F -- "${want}" <<<"${MUTANT_OUT}" >/dev/null; then
    bad "self-test: control '${label}' passed without printing '${want}'"
  else
    ok "self-test: control ${label} passes"
  fi
  MUT_ENV=()
}

# expect_selftest_red <label> <only> <want> <command...>: a mutant of the guard itself. The
# copy's own --self-test (nested, limited to the mutants whose label contains <only>) must fail
# and print <want>. Skipped inside a nested self-test, so the nesting stops at one level.
expect_selftest_red() {
  local label="$1" only="$2" want="$3" copy out
  shift 3
  selected "${label}" || return 0
  if [[ "${AGENT_CONTRACT_NESTED:-}" == 1 ]]; then
    echo "skip - self-test: ${label} (nested run)"
    return 0
  fi
  copy="$(mktemp -d)"
  SCRATCH+=("${copy}")
  cp -a "${BASE}/." "${copy}/"
  if ! (cd "${copy}" && "$@") >"${MUTLOG}" 2>&1; then
    bad "self-test: mutant '${label}' was not applied, nothing was tested: $(head -3 "${MUTLOG}")"
    rm -rf "${copy:?}"
    return
  fi
  if out="$(cd "${copy}" && env AGENT_CONTRACT_NESTED=1 AGENT_CONTRACT_ONLY="${only}" bash hack/test/agent_contract_test.sh --self-test 2>&1)"; then
    bad "self-test: mutant '${label}' survived (the copy's own self-test passed)"
  elif grep -F -- "${want}" <<<"${out}" >/dev/null; then
    ok "self-test: ${label} fails the copy's self-test with '${want}'"
  else
    bad "self-test: mutant '${label}' failed the copy's self-test without '${want}': $(failure_lines "${out}" | head -2 | tr '\n' ' ')"
  fi
  rm -rf "${copy:?}"
}

# --- mutation steps (run inside the copy) --------------------------------------------------------
append() { printf '\n%s\n' "$2" >>"$1"; }
plant() { append "${AGENTS}" "$1"; }
plant_code() { append "${AGENTS}" "Run \`$1\`."; }
# first_line <file> <text>: insert the text as the file's first line
first_line() { L="$2" perl -i -pe 'print "$ENV{L}\n" if $. == 1' "$1"; }
header_committed() { first_line "${AGENTS}" "$1" && copy_git add -- "${AGENTS}"; }
header_pointer() { first_line "${COPILOT}" "$1"; }
# stage_only: stage the mutated AGENTS.md, then restore the working tree from HEAD, so only the
# index carries the mutation
stage_only() {
  copy_git add -- "${AGENTS}" && git show "HEAD:${AGENTS}" >"${AGENTS}" || return 1
  git diff --quiet HEAD -- "${AGENTS}" && ! git diff --quiet --cached -- "${AGENTS}"
}
stage_plant() { plant "$1" && stage_only; }
stage_header() { first_line "${AGENTS}" "$1" && stage_only; }
must_change() {
  # must_change <file> <perl-program>: edit in place and require that the file changed
  local before
  before="$(cksum <"$1")"
  perl -i -pe "$2" "$1" || return 1
  [[ "$(cksum <"$1")" != "${before}" ]]
}
untrack_agents() { copy_git rm -q --cached -- "${AGENTS}" && append .gitignore "${AGENTS}"; }
remove_agents() { copy_git rm -q -- "${AGENTS}"; }
drop_gate() {
  G="$1" must_change "${AGENTS}" 'BEGIN { $g = $ENV{G} } $in = 1 if /^\s*<!-- gates:start -->\s*$/; $in = 0 if /^\s*<!-- gates:end -->\s*$/; s/`\Q$g\E`//g if $in'
}
extra_gate() { must_change "${AGENTS}" 's/^(\s*<!-- gates:end -->)/- `frobnicate-gate`\n$1/'; }
no_marker() { must_change "${AGENTS}" '$_ = "" if /^\s*<!-- gates:start -->\s*$/'; }
drop_task() {
  T="$1" must_change "${AGENTS}" 'BEGIN { $t = $ENV{T} } s/\btask((?:\s+-\S+)*\s+)\Q$t\E(?![\w:.-])/removed$1$t/g'
}
unlink_to() { L="$1" must_change "${AGENTS}" 'BEGIN { $l = $ENV{L} } s/\]\((?:\.\/)?\Q$l\E(?:#[^)]*)?\)/](README.md)/g'; }
copilot_set() { printf '%s\n' "$@" >"${COPILOT}"; }
plant_lines() {
  local l
  for l in "$@"; do append "${AGENTS}" "${l}" || return 1; done
}
ignored_pointer() { printf 'x\n' >CLAUDE.md && copilot_set "See [AGENTS.md](../CLAUDE.md)."; }
track_claude() { printf 'x\n' >CLAUDE.md && copy_git add -f -- CLAUDE.md; }
rename_heading() { H="$1" must_change "${AGENTS}" 'BEGIN { $h = $ENV{H} } s/^\Q$h\E\s*$/### Renamed checklist/'; }
move_registry() {
  grep -F '`internal/sink/registry.go`' "${AGENTS}" >/dev/null || return 1
  copy_git mv internal/sink/registry.go internal/sink/registry_moved.go
}
rename_symbol() {
  grep -F '`NewRegistry`' "${AGENTS}" >/dev/null || return 1
  must_change internal/sink/registry.go 's/\bNewRegistry\b/NewRegistryRenamed/g'
}
drop_span() { S="$1" must_change "${AGENTS}" 'BEGIN { $s = $ENV{S} } s/`\Q$s\E`/the place/g'; }
plant_in_section() {
  # plant_in_section <heading> <line>: insert the line right after the heading
  H="$1" X="$2" must_change "${AGENTS}" 'BEGIN { $h = $ENV{H}; $x = $ENV{X} } $_ .= "\n$x\n" if /^\Q$h\E\s*$/'
}
grouped_var() {
  perl -ne 'if (/^var\s*\(\s*$/) { $b = 1; next } if ($b && /^\)/) { $b = 0 } if ($b && /^\s+([A-Za-z_]\w*)/) { print "$1\n"; exit }' internal/metrics/metrics.go
}
plant_grouped_var() {
  local v
  v="$(grouped_var)"
  [[ -n "${v}" ]] || return 1
  plant_in_section "### When adding a metric" "- \`internal/metrics/metrics.go\`: \`${v}\`"
}
set_patterns() { printf '%s' "$1" >"${PATTERNS_FILE}"; }
fixed_string_guard() { must_change hack/test/agent_contract_test.sh 's/rg -i -q -f/rg -F -i -q -f/ if /# scrub-rg\s*\z/'; }
empty_generic() {
  must_change hack/test/agent_contract_test.sh 'if (/^GENERIC_PATTERNS=\($/) { $skip = 1; $_ = "GENERIC_PATTERNS=()\n"; next } if ($skip) { $skip = 0 if /^\)$/; $_ = "" }'
}
add_ignore() { append .gitignore "$1"; }
drop_ignore() {
  E="$1" must_change .gitignore 'BEGIN { $e = $ENV{E}; $e =~ s{/x$}{/} } $_ = "" if m{^/?\Q$e\E\s*$}'
}
# allow_basic <toml-escaped text>: a [[allowlists]] table whose path is a TOML basic string
# append_toml <line...>: append the lines to .github/gitleaks.toml
append_toml() { printf '\n' >>.github/gitleaks.toml && printf '%s\n' "$@" >>.github/gitleaks.toml; }
allow_basic() { printf '\n[[allowlists]]\npaths = ["%s"]\n' "$1" >>.github/gitleaks.toml; }
allow_path() {
  # allow_path <table|inline> <regex>
  if [[ "$1" == table ]]; then
    printf "\n[[allowlists]]\npaths = ['''%s''']\n" "$2" >>.github/gitleaks.toml
  else
    R="$2" must_change .github/gitleaks.toml 'BEGIN { $r = $ENV{R} } if (/^\[allowlist\]/) { $a = 1 } elsif (/^\[/) { $a = 0 } $_ .= "  \x27\x27\x27$r\x27\x27\x27,\n" if $a && /^paths\s*=\s*\[\s*$/'
  fi
}
# These two helpers set HELPER_OUT instead of printing it: a $(...) subshell would lose the
# SCRATCH entries they add, and the cleanup trap would miss their directories.
fake_rg_dir() {
  local d
  d="$(mktemp -d)"
  SCRATCH+=("${d}")
  printf '#!/bin/sh\nexit 2\n' >"${d}/rg"
  chmod +x "${d}/rg"
  HELPER_OUT="${d}"
}
# A PATH with every directory that holds rg replaced by a shadow of links to all its other entries.
path_without_rg() {
  local out="" dir shadow
  IFS=: read -ra dirs <<<"${PATH}"
  for dir in "${dirs[@]}"; do
    if [[ -x "${dir}/rg" ]]; then
      shadow="$(mktemp -d)"
      SCRATCH+=("${shadow}")
      find "${dir}/" -mindepth 1 -maxdepth 1 ! -name rg -exec ln -s -t "${shadow}" {} + 2>/dev/null
      dir="${shadow}"
    fi
    out="${out:+${out}:}${dir}"
  done
  HELPER_OUT="${out}"
}

# --- PAC-5 harness probes (in-process) -----------------------------------------------------------
if selected "PAC-5"; then
  copy="$(mktemp -d)"
  SCRATCH+=("${copy}")
  cp -a "${BASE}/." "${copy}/"
  if out="$(cd "${copy}" && env -u AGENT_CONTRACT_ONLY -u AGENT_CONTRACT_NESTED bash hack/test/agent_contract_test.sh 2>&1)"; then
    ok "self-test: PAC-5 no-op copy passes"
  else
    bad "self-test: PAC-5 no-op copy failed -- the checks are broken on an unmodified tree: $(failure_lines "${out}" | head -3 | tr '\n' ' ')"
  fi
  rm -rf "${copy:?}"

  saved_failures="${FAILURES}"
  expect_red "PAC-5 probe: surviving mutant" "PAC-4: AGENTS.md (disk) matches a scrub pattern" true 2>/dev/null
  expect_red "PAC-5 probe: wrong clause" "PAC-4: AGENTS.md (disk) matches a scrub pattern" rm -f Taskfile.yml 2>/dev/null
  probe_failures=$((FAILURES - saved_failures))
  FAILURES="${saved_failures}"
  if [[ "${probe_failures}" -eq 2 ]]; then
    ok "self-test: PAC-5 a surviving mutant and a wrong-clause red both fail the self-test"
  else
    bad "self-test: PAC-5 harness probes: expected 2 self-test failures (surviving, wrong clause), got ${probe_failures}"
  fi
fi

# --- PAC-1 ---------------------------------------------------------------------------------------
expect_red "PAC-1 missing" "PAC-1: AGENTS.md is not tracked" remove_agents
expect_red "PAC-1 untracked" "PAC-1: AGENTS.md is not tracked" untrack_agents
expect_red "PAC-1 unknown task" "PAC-1: AGENTS.md names an undefined task 'frobnicate'" plant_code "task frobnicate"
first_gate="$(check_gates | head -1)"
expect_red "PAC-1 gate dropped" "PAC-1: gate list: hack/check.sh runs gate '${first_gate}' but AGENTS.md does not list it" drop_gate "${first_gate}"
expect_red "PAC-1 gate extra" "PAC-1: gate list: AGENTS.md lists gate 'frobnicate-gate' that hack/check.sh does not run" extra_gate
expect_red "PAC-1 gate marker" "PAC-1: gate list: marker '<!-- gates:start -->' missing" no_marker
expect_green "PAC-1 gate name in prose" "" plant "Prose may mention the \`zz-not-a-gate\` gate outside the markers."
expect_green "PAC-1 task-name parsing" "" plant "the task that runs first, \`task --list-all\` and \`FOO=1 task build\`"
for t in "${REQUIRED_TASKS[@]}"; do
  expect_red "PAC-1 required task ${t}" "PAC-1: AGENTS.md does not name the required command 'task ${t}'" drop_task "${t}"
done
expect_red "PAC-1 link CONTRIBUTING.md" "PAC-1: AGENTS.md does not link CONTRIBUTING.md" unlink_to CONTRIBUTING.md
expect_red "PAC-1 link spec-workflow" "PAC-1: AGENTS.md does not link docs/development/spec-workflow.md" unlink_to docs/development/spec-workflow.md

# --- PAC-2 ---------------------------------------------------------------------------------------
expect_red "PAC-2 long pointer" "PAC-2: ${COPILOT} has 4 lines (at most 3)" copilot_set "See AGENTS.md." "a" "b" "c"
expect_red "PAC-2 pointer target" "PAC-2: ${COPILOT} does not point to AGENTS.md" copilot_set "See the contributing guide."
expect_red "PAC-2 pointer missing path" "PAC-2: ${COPILOT} names a missing path '../MISSING-AGENTS.md'" copilot_set "See [AGENTS.md](../MISSING-AGENTS.md)."
expect_red "PAC-2 pointer to an ignored local file" "PAC-2: ${COPILOT} names a missing path '../CLAUDE.md'" ignored_pointer
expect_red "PAC-2 pointer leaving the repository" "PAC-2: ${COPILOT} names a missing path '../../x/AGENTS.md'" copilot_set "See [AGENTS.md](../../x/AGENTS.md)."
expect_red "PAC-2 CLAUDE.md tracked" "PAC-2: CLAUDE.md is tracked" track_claude

# --- PAC-3 ---------------------------------------------------------------------------------------
for h in "${CHECKLISTS[@]}"; do
  expect_red "PAC-3 heading ${h#\#\#\# }" "PAC-3: missing checklist heading '${h}'" rename_heading "${h}"
done
expect_red "PAC-3 stale path" "PAC-3: checklist path 'internal/sink/registry.go' matches no tracked file" move_registry
expect_red "PAC-3 stale symbol" "PAC-3: checklist symbol 'NewRegistry' is not declared in" rename_symbol
expect_green "PAC-3 method symbol" "" plant_in_section "### When adding a sink" '- `internal/sink/registry.go`: `Register`'
expect_green "PAC-3 grouped var symbol" "" plant_grouped_var
for m in "${METRIC_ENTRIES[@]}"; do
  expect_red "PAC-3 metric entry ${m}" "PAC-3: metric checklist does not name '${m}'" drop_span "${m}"
done
expect_green "PAC-3 placeholder and glob" "" plant_in_section "### When adding a sink" '- `internal/sink/<name>` and `api/v1alpha1/*_types.go`'
expect_red "PAC-3 glob matches nothing" "PAC-3: checklist path 'internal/nothing/*_zz.go' matches no tracked file" plant_in_section "### When adding a sink" '- `internal/nothing/*_zz.go`'

# --- PAC-4 scrub ---------------------------------------------------------------------------------
n=0
while IFS= read -r pat || [[ -n "${pat}" ]]; do
  [[ -z "${pat//[[:space:]]/}" ]] && continue
  n=$((n + 1))
  expect_red "PAC-4 scrub line ${n}" "PAC-4: AGENTS.md (disk) matches a scrub pattern" plant "${pat}"
  expect_red "PAC-4 scrub line ${n} upper-cased" "PAC-4: AGENTS.md (disk) matches a scrub pattern" plant "${pat^^}"
  if [[ "${pat}" == *.* ]]; then
    witness="${pat//./x}"
    if selected "PAC-4 scrub line ${n} regex witness" && ! printf '%s\n' "${witness}" | rg -i -q -f <(printf '%s\n' "${pat}"); then
      bad "self-test: PAC-4 scrub line ${n} has no usable regex witness"
    else
      expect_red "PAC-4 scrub line ${n} regex witness" "PAC-4: AGENTS.md (disk) matches a scrub pattern" plant "${witness}"
    fi
  fi
  if [[ "${n}" -eq 1 ]]; then
    expect_red "PAC-4 staged scrub string" "PAC-4: AGENTS.md (index) matches a scrub pattern" stage_plant "${pat}"
  fi
done <"${ROOT}/${PATTERNS_FILE}"
[[ "${n}" -gt 0 ]] || bad "self-test: ${PATTERNS_FILE} holds no pattern to plant"
expect_red "PAC-4 staged private header" "PAC-4: AGENTS.md (index) carries a private-file header" stage_header "<!-- LOCAL ONLY -->"
# A guard whose scrub rg lost regex semantics (-F) still reds on the literal and upper-cased
# plants; only the regex witnesses survive, so the copy's own self-test must name one.
expect_selftest_red "PAC-4 fixed-string implementation" "regex witness" "regex witness' survived" fixed_string_guard
expect_red "PAC-4 pattern list empty" "PAC-4: scrub pattern list ${PATTERNS_FILE} is empty" set_patterns ""
expect_red "PAC-4 pattern list missing" "PAC-4: scrub pattern list ${PATTERNS_FILE} is missing" rm -f "${PATTERNS_FILE}"
fake_rg_dir
MUT_ENV=(PATH="${HELPER_OUT}:${PATH}")
expect_red "PAC-4 rg error" "PAC-4: rg failed (exit 2)" true
path_without_rg
MUT_ENV=(PATH="${HELPER_OUT}")
expect_red "PAC-4 rg missing" "PAC-4: rg not found" true

# --- PAC-4 paths, ignore rules, headers, markers, gitleaks ---------------------------------------
expect_red "PAC-4 home path /home" "PAC-4: AGENTS.md (disk) names a home path" plant "see /home/someone/x"
expect_red "PAC-4 home path /Users" "PAC-4: AGENTS.md (disk) names a home path" plant "see /Users/someone/x"
expect_red "PAC-4 home path ~/" "PAC-4: AGENTS.md (disk) names a ~/ path" plant "see ~/x"
for h in "${HARNESS_PATHS[@]}"; do
  expect_red "PAC-4 harness path ${h}" "PAC-4: AGENTS.md (disk) names the gitignored harness path '${h}'" plant "see ${h}GUIDE"
done
expect_red "PAC-4 ignore AGENTS.md" "PAC-4: .gitignore ignores the tracked AGENTS.md" add_ignore AGENTS.md
for e in "${MUST_IGNORE[@]}"; do
  expect_red "PAC-4 un-ignore ${e%/x}" "PAC-4: .gitignore does not ignore '${e%/x}'" drop_ignore "${e}"
done
fakehome="$(mktemp -d)"
SCRATCH+=("${fakehome}")
mkdir -p "${fakehome}/.config/git"
printf '.claude/\n' >"${fakehome}/.config/git/ignore"
printf '[core]\n\texcludesFile = %s/.config/git/ignore\n' "${fakehome}" >"${fakehome}/.gitconfig"
MUT_ENV=(HOME="${fakehome}" XDG_CONFIG_HOME="${fakehome}/.config")
expect_red "PAC-4 un-ignore .claude with a global ignore" "PAC-4: .gitignore does not ignore '.claude'" drop_ignore .claude/x
for hdr in "<!-- LOCAL ONLY -->" "<!-- never commit -->" "<!-- Local Only: Never Commit -->"; do
  expect_red "PAC-4 private header '${hdr}'" "PAC-4: AGENTS.md (disk) carries a private-file header" header_committed "${hdr}"
  expect_red "PAC-4 private header in pointer '${hdr}'" "PAC-4: ${COPILOT} (disk) carries a private-file header" header_pointer "${hdr}"
done
expect_red "PAC-4 OpenSpec start marker" "PAC-4: AGENTS.md (disk) carries an OpenSpec marker" plant "<!-- OPENSPEC:START -->"
expect_red "PAC-4 OpenSpec end marker" "PAC-4: AGENTS.md (disk) carries an OpenSpec marker" plant "<!-- openspec:end -->"
for form in inline table; do
  expect_red "PAC-4 gitleaks ${form} AGENTS.md" "exempts AGENTS.md from secret scanning" allow_path "${form}" '(?:^|/)AGENTS\.md$'
  expect_red "PAC-4 gitleaks ${form} any .md" "exempts AGENTS.md from secret scanning" allow_path "${form}" '.*\.md$'
  expect_red "PAC-4 gitleaks ${form} CLAUDE.md" "exempts CLAUDE.md from secret scanning" allow_path "${form}" '(?:^|/)CLAUDE\.md$'
  expect_red "PAC-4 gitleaks ${form} copilot" "exempts ${COPILOT} from secret scanning" allow_path "${form}" '^\.github/copilot-instructions\.md$'
done
expect_red "PAC-4 gitleaks escaped basic string" "exempts AGENTS.md from secret scanning" allow_basic '\u0041GENTS[.]md$'
expect_red "PAC-4 gitleaks quoted table and key" "exempts AGENTS.md from secret scanning" append_toml '[[ "allowlists" ]]' "\"paths\" = ['''(?:^|/)AGENTS\\.md\$''']"
expect_red "PAC-4 gitleaks rule inline allowlist" "exempts AGENTS.md from secret scanning" append_toml '[[rules]]' 'id = "zz-rule"' "regex = '''zz'''" "allowlist = { paths = ['''AGENTS\\.md'''] }"
expect_red "PAC-4 gitleaks rule dotted allowlist" "exempts CLAUDE.md from secret scanning" append_toml '[[rules]]' 'id = "zz-rule2"' "regex = '''zz'''" "allowlist.paths = ['''CLAUDE\\.md''']"
expect_red "PAC-4 gitleaks unparseable" "could not be parsed as TOML" append_toml '[allowlist'
expect_red "PAC-4 gitleaks invalid regex" "could not be compiled" allow_path table '(unclosed'

# --- PAC-6 ---------------------------------------------------------------------------------------
stem="zq$((RANDOM % 900 + 100))x"
for suffix in Internal LOCAL Lan Home.Arpa; do
  expect_red "PAC-6 host .${suffix,,}" "PAC-6: AGENTS.md (disk) names an internal host or address (internal DNS name" plant "ssh to Db.${stem}.${suffix} first"
done
edges=(10.0.0.1 10.255.255.254 172.16.0.1 172.31.255.254 192.168.0.1 192.168.255.254
  169.254.0.1 169.254.255.254 100.64.0.1 100.127.255.254)
for ip in "${edges[@]}"; do
  IFS=. read -r a b c d <<<"${ip}"
  expect_red "PAC-6 address ${ip}" "PAC-6: AGENTS.md (disk) names an internal host or address (private IPv4 address" plant "$(printf 'connect to %d.%d.%d.%d:5432' "${a}" "${b}" "${c}" "${d}")"
done
expect_red "PAC-6 ULA" "(unique-local IPv6 address" plant "listen on fd00::1"
expect_red "PAC-6 Host line" "(SSH Host directive" plant "Host ${stem}"
expect_red "PAC-6 Host line with a comment" "(SSH Host directive" plant "Host ${stem} # build box"
expect_red "PAC-6 scp address at github.com, not git" "(scp-style address" plant "copy ${stem}@github.com:org/repo here"
expect_red "PAC-6 scp address" "(scp-style address" plant "copy ${stem}@${stem}box:src/x here"
expect_green "PAC-6 prose look-alikes" "" plant_lines "host the chart in your own registry before you install it" "Report it to security@example.com: we answer within a week."
expect_green "PAC-6 allowed look-alikes" "" plant "kollect.kollect-system.svc.cluster.local localhost mise.local.toml git@github.com:org/repo ghcr.io/org/kollect@sha256:abc postgres://user:pass@db.example:5432/x 172.32.0.1 192.0.2.1 100.63.255.255 100.128.0.1"
MUT_ENV=(AGENT_CONTRACT_HOSTNAME="${stem}host")
expect_red "PAC-6 running host" "PAC-6: AGENTS.md (disk) contains the running host's short name" plant "built on ${stem^^}HOST today"
for skip in ab localhost build operator ""; do
  MUT_ENV=(AGENT_CONTRACT_HOSTNAME="${skip}")
  expect_green "PAC-6 running-host skip '${skip}'" "PAC-6: skip running-host check" plant "built on ${skip:-nothing}"
done
# An emptied generic pattern set lets every PAC-6 generic mutant survive in the copy's self-test.
expect_selftest_red "PAC-6 empty pattern set" "PAC-6 host" "PAC-6 host .internal' survived" empty_generic

if ((FAILURES > 0)); then
  echo "agent contract self-test: FAILED (${FAILURES} problem(s))" >&2
  exit 1
fi
ok "agent contract self-test: every mutant red with its target diagnostic, the no-op copy green"
