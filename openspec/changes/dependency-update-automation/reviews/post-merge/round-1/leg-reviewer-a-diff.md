| ID | Severity | File:line | Finding | Fix |
|---|---|---|---|---|
| R1 | WARNING | `.github/workflows/e2e-extended.yaml:18`, `:85` | Case handling differs: GitHub expressions compare strings case-insensitively, while Bash compares case-sensitively. Adding `E2E/FULL` therefore shares the original run’s group and cancels it, but the gate returns `false`. Cancelled matrix checks can remain without successful replacements, blocking Renovate. [GitHub expression semantics](https://docs.github.com/en/enterprise-server%403.22/actions/reference/workflows-and-actions/expressions). | Use consistent label matching in concurrency and the gate, including the existing-label check. Add a regression case for mixed-case labels. |

For the reported `dependencies` event, the fix works: it isolates the labeled run from the original. Exact `e2e/full` labels and PR pushes retain their original group and supersede earlier runs. Relevant-file detection remains unchanged for opened, reopened, and synchronize events.

The expression is valid: `actionlint` passed, and workflow concurrency permits the `github` context. Label text selects a fixed suffix and enters Bash through a quoted environment variable; there is no new injection path. [GitHub concurrency syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#concurrency).

Two qualifications:

- The labeled run is not entirely a no-op: three meta jobs and the gate’s checkout still execute and can fail. Multiple unrelated labels also share `-other-label`, so they can cancel each other; ordinarily, the surviving successful run replaces those same-name checks under the supplied API semantics.
- Main’s running workflows remain protected. “Main never cancels” is broader than the existing guarantee: multiple main dispatches can still replace pending runs in the shared group. This predates the patch.

No guard incompatibility found by inspection. `e2e_mt_repro_harden_test.sh` passed. The finalizer and webhook guards were inspected but not executed because they create temporary files under this read-only review.

## Verdict

CONCERNS
