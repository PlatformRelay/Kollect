"""G3 probe: compare (apiGroup, resource, verb) triples of config/rbac/role.yaml
against the chart's manager roles rendered by `helm template`.

usage: python3 -I probe.py <repo> [helm --set args...]
"""
import subprocess
import sys

import yaml

repo = sys.argv[1]
sets = sys.argv[2:]


def triples(rules):
    out = set()
    for r in rules or []:
        for g in r.get("apiGroups") or []:
            for res in r.get("resources") or []:
                for v in r.get("verbs") or []:
                    out.add((g, res, v))
    return out


want = triples(yaml.safe_load(open(f"{repo}/config/rbac/role.yaml"))["rules"])

cmd = ["helm", "template", "kollect", f"{repo}/charts/kollect", "--namespace", "kollect-system"]
for s in sets:
    cmd += ["--set", s]
# Not --show-only: it errors when a template renders nothing (clusterrole.yaml in
# tenant mode), so render everything and filter by the "# Source:" comment.
rendered = subprocess.run(cmd, check=True, capture_output=True, text=True).stdout

have = set()
sources = []
src = None
for chunk in rendered.split("\n---"):
    src = None
    for line in chunk.splitlines():
        if line.startswith("# Source: "):
            src = line[len("# Source: "):]
    if not src or not any(src.endswith("/templates/" + t) for t in ("clusterrole.yaml", "role.yaml", "role-leader-election.yaml")):
        continue
    doc = yaml.safe_load(chunk)
    if not doc:
        continue
    sources.append(f"{src} ({doc['kind']} {doc['metadata']['name']})")
    have |= triples(doc.get("rules"))

print("mode:", " ".join(sets) or "defaults")
print("manager roles:", *sources, sep="\n  ")
print(f"role.yaml triples: {len(want)}; rendered triples: {len(have)}")
missing = sorted(want - have)
print(f"missing from chart: {len(missing)}")
for g, r, v in missing:
    print(f"  {g or 'core'} {r} {v}")
extra = sorted(have - want)
print(f"chart-only (superset, not a failure): {len(extra)}")
for g, r, v in extra:
    print(f"  {g or 'core'} {r} {v}")
