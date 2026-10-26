## Verdict
verdict: APPROVE
findings:
  - [P2] openspec/changes/mutation-testing-signal/evidence/G1.md:354 references nonexistent “Sensors (final)”; markdown lint and evidence-guard results, including --self-test, are not recorded.
matrix_disputes:
  - Rows 11–12 claim independent-review pass, but G1.md:391 says review pending; the recorded classes overstate completed verification, although this review supports both conclusions.
could_not_check:
  - Live #469 merge metadata without network; its recorded commit exists in branch ancestry.
  - Mutation probes and Go sensors were not rerun under read-only constraints; Go/gofmt shims failed attempting toolchain installation. Gremlins --help executed successfully.
summary: All probe clauses are covered; six reports byte-match scratch outputs, counts and control coordinates agree, and survivor classifications hold. F1 is supported and does not contradict the spec; exporting GOFLAGS=-count=1 preserves MUT-1/MUT-2 and R1 argv.
