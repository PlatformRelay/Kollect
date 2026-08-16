# lab-evidence

Preserved evidence from lab runs whose source substrate no longer exists.

This directory is **tracked** but deliberately **not published** — `mkdocs.yml` sets `docs_dir: docs`,
so nothing here reaches the documentation site. It is not `agent-context/` (gitignored, local-only,
for working notes) and not `artifacts/` (gitignored build output, never committed).

## Why it exists

Lab protocols are local-only by convention. That convention assumes the lab can be re-measured. When
a substrate is destroyed and a measurement becomes **unrepeatable**, a local-only copy is one disk
failure away from unverifiable claims in the changelog and the roadmap.

Contents are committed only when all three hold:

1. the measurement backs a published claim or a closed gate;
2. the substrate that produced it is gone, so it cannot be re-taken;
3. it has been scrubbed (see below).

## Scrubbing rules

Everything here is captured off real infrastructure, so it is redacted before it is committed:

- **Real LAN addresses are replaced with RFC 5737 TEST-NET-1 (`192.0.2.0/24`)**, preserving the last
  octet so node identity stays readable. This repository is public and the operator's SC-007 rule
  forbids real LAN addresses in tracked files.
- No credentials, tokens, JWTs, or private keys. Verified by grep before commit, in addition to
  `hack/scrub.sh`.
- Kubernetes ServiceAccount names, namespaces and image references are kept — they are meaningful to
  the evidence and are not secrets.

## Index

| Directory | What it preserves |
| --- | --- |
| [`2026-08-16-L10k-LT-S14/`](./2026-08-16-L10k-LT-S14/) | The L-10k / LT-S14 gate closure — 10,000/10,000 inventory rows and a numeric `export_p99` on a 3-node bare-metal Talos cluster. The cluster was destroyed hours after capture; this is the only surviving copy. |
