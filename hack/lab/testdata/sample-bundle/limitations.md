# Limitations / not claimed

Every publishable summary must list explicit limitations (or "not claimed") items:

- DR-4.1 SKIPPED — Wave-4 Tier-S synthetic load not run (fixture schedule)
- DR-4.3 LIMIT_REACHED — idle pprof only; no churn baseline
- Metrics-server absent (`kubectl top` N/A) in this synthetic sample
- Full Ubuntu D-suite / every sink / NetPol deny-path / 100k cloud gate not in scope
- Emulator fidelity caveats: fixture backends only; not managed SaaS parity

> Synthetic sample for LAB-H06. Treat as bounded fixture evidence, not a product pin claim.
