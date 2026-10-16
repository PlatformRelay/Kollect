## Verdict: CONCERNS

## Findings
- [WARNING] git-cliff and helm-docs installs on stock macOS now fail. Both installers advertise Darwin, but the new checks call `sha512sum` and `sha256sum`, which are GNU coreutils commands — `hack/install-git-cliff.sh:50`, `hack/lib/verify-sha256.sh:21`
  Failure: a macOS developer without coreutils runs the Taskfile target (`Taskfile.yml:376`) or `hack/verify-changelog.sh:11`. Under `set -euo pipefail` the run aborts with `sha512sum: command not found`, and nothing is installed. Before this commit the same install worked unverified. It fails closed, so this is not a security problem, but it is a regression the commit message does not mention. Evidence item 7 defers it to BACKLOG DTP-SHA512-MACOS.
  Fix: fall back to `shasum -a 512` / `shasum -a 256` when the GNU command is missing, or state in the commit or PR that Darwin is unsupported until DTP-SHA512-MACOS lands.
  Confidence: 70
- [NOTE] The digest comes from the same GitHub release as the tarball, so a compromised release or maintainer account defeats the check — `hack/install-git-cliff.sh:43`, `hack/install-helm-docs.sh:62`
  Failure: an attacker who can replace release assets replaces the `.sha512` or `checksums.txt` too, and the installer accepts the result. The check covers corruption or tampering of the tarball alone, not a compromised upstream. This matches the gitleaks pattern the evidence cites, so it is not a regression, but "nothing is ever installed unverified" (`hack/install-git-cliff.sh:42`) promises more than the check delivers.
  Fix: no code change needed for this task. Optionally pin the expected digest in-repo next to the Taskfile version, or reword the comment to "verified against the upstream-published checksum".
  Confidence: 85
- [NOTE] The emptied array is expanded unguarded under `set -u` — `hack/test/ci_fetch_lib_hardening_test.sh:771,789,798`
  Failure: under bash < 4.4 (macOS's stock bash 3.2), `"${UNVERIFIED_BY_DESIGN[@]}"` on `()` aborts with "unbound variable". The toolchain-pins guard uses the safe `${X[@]+"${X[@]}"}` form for the same list (`consistency_toolchain_pins_test.sh:419,421`). The repo already assumes bash 5 for tests (`hack/test/dist_ci_wiring_test.sh:140`), so the impact is low. Line 798 also now prints `()` in the pass message.
  Fix: use `${UNVERIFIED_BY_DESIGN[@]+"${UNVERIFIED_BY_DESIGN[@]}"}` at lines 771 and 789 to match the guard.
  Confidence: 60
- [NOTE] helm-docs only checks that the resolved digest is non-empty, while git-cliff requires a 128-hex value — `hack/install-helm-docs.sh:64`
  Failure: a duplicated or malformed `checksums.txt` line yields a multi-line or non-hex `expected`. That still fails closed with a "checksum mismatch" error instead of the clearer "failed to resolve", so only the diagnostic suffers.
  Fix: `[[ "${expected}" =~ ^[0-9a-f]{64}$ ]]`, to match the git-cliff check.
  Confidence: 50

What I checked and found sound:
- **Order and parsing:** the checksum is fetched before the tarball, and the digest is taken only from the line naming this asset (the `*asset` binary-mode form is also accepted). A failed checksum fetch stops the install before the tarball is fetched.
- **Fake `fetch_to`:** its failure message matches the real one (`hack/lib/fetch.sh:141`).
- **Comment can't satisfy the scan:** `logical_lines` drops comments, so the `sha512sum` mention in the installer's comment at line 41 does not count toward the hardening scan (`ci_fetch_lib_hardening_test.sh:93`).
- **Mutants run against the copy:** with a relative `GATE`, `run_gate` runs the copy's own hardening gate, whose `ROOT` resolves to the copy (`consistency_toolchain_pins_test.sh:1138-1141`, `ci_fetch_lib_hardening_test.sh:49`).
- **Tampered case:** flipping the gzip MTIME byte leaves an archive that still extracts, and the test asserts that. The sed that removes verification is confirmed to change the file.
- **"(empty)" sentinel:** it matches `${UNVERIFIED_LIMIT[*]:-empty}` (`consistency_toolchain_pins_test.sh:425`).
- **Allow-lists only shrank:** `UNVERIFIED_BY_DESIGN` and `UNVERIFIED_LIMIT` both went to `()`. Accepting `sha512sum` in the scan adds a verifier and exempts no installer.
- **Commit message:** it matches the diff and contains no agent attribution.

## Could not check
- `/tmp/g4red/brief.md` is outside the repository, so I did not read it.
- I ran no tests, sensors or upstream probes. The gate exits, self-test counts, red-before runs, hand mutants and the real installs of v2.13.1 / v1.14.2 come from `evidence/G4.md`, not from my own runs.
- I did not verify that BACKLOG DTP-SHA512-MACOS exists, or that stock macOS lacks `sha256sum` on current releases.
- I did not read `unverified_other_uses` or the full hardening gate outside lines 47–109 and 758–798.
