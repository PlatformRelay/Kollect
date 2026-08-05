# Scenario matrix

| ID | Verdict | Evidence / notes |
| --- | --- | --- |
| DR-0.1 | PASS | Synthetic topology recorded (fixture) |
| DR-1.1 | PASS | Two manager replicas; single lease holder (fixture counts) |
| DR-2.4 | PASS | Inventory export count=3 (fixture) |
| DR-3.1 | PASS_WITH_LIMITATION | Certificate scrape collecting 1 (partial vs live; fixture) |
| DR-4.1 | SKIPPED | Wave-4 Tier-S load not in quick+sinks fixture schedule |
| DR-4.3 | LIMIT_REACHED | Idle pprof only; churn baseline not captured (fixture bound) |

> Allowed verdicts per LAB-DOC-02. Non-pass rows state why. Synthetic only.
