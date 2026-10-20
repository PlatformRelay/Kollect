# Branch review brief — sink-credential-rotation, stage B round 1, spec lens

Run through the fan-out script's standard leg brief (lens `spec`, `--no-unify`, leg timeout
1800 s), with `SPEC` = `openspec/changes/sink-credential-rotation/specs/sink-secret-rotation/spec.md`
and this `TARGET`:

> whole branch feat/sink-credential-rotation: git diff ae98ef106..HEAD (commits d307e2ec8 G1,
> f08fb2b12 G2, b6fca7d1e G3, df5170e57 G4 docs; they land together, judge the tip). Produce an
> SCR compliance table, one row per requirement SCR-1..SCR-7 and per scenario under it: holds /
> partial / contradicted / absent / stronger-than-spec, with the deciding production file:line
> and the test that pins it. Then list behaviour the branch adds that no requirement mentions.
> Must read: observeAndLookup in internal/sink/backend_pool.go (the failed re-read path, staleKey
> and retireEntryWithKey, never reviewed) and say whether SCR-2/SCR-3 hold there; the accepted
> residual that a forced second read retires an entry stored during it (claimed: one extra
> build, never stale use); any cleanup path that still reads a Secret it should not, and any
> path that closes a backend while an export uses it; the SCR-7 terminal-no-requeue rule in
> both reconcilers. Inputs: openspec/changes/sink-credential-rotation/{proposal.md,design.md,tasks.md}
> and evidence/{T0.1,G1,G2,G3,G4}.md (verification matrices; trust recorded exit codes). Output
> at most 60 lines as the brief says; the table may use compact rows.
