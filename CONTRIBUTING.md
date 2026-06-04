# Contributing to kollect

Thank you for helping improve kollect. This project uses **Conventional Commits** with an
optional **gitmoji** prefix in the subject line.

## Commit messages

Format:

```text
:gitmoji: type(scope): short summary

Optional body with motivation and breaking-change notes.
```

Examples:

- `:sparkles: feat(api): add KollectScope CRD skeleton`
- `:wrench: build: pin controller-tools for codegen`
- `:green_heart: ci: add preflight workflow`
- `:memo: docs: expand README quickstart`

Common gitmoji: `:sparkles:` feat, `:bug:` fix, `:wrench:` build/chore, `:green_heart:` ci,
`:memo:` docs, `:white_check_mark:` test, `:recycle:` refactor, `:tada:` initial/bootstrap.

Types follow [Conventional Commits](https://www.conventionalcommits.org/): `feat`, `fix`,
`docs`, `test`, `ci`, `build`, `chore`, `refactor`, `perf`.

Release notes are generated with [git-cliff](https://git-cliff.org/) (`cliff.toml`); gitmoji
tokens are stripped from changelog headings automatically.

## Pull request process

1. Fork or branch from `main`.
2. Run locally:
   - `task lint`
   - `task test`
   - `task verify`
   - `task scrub` (after staging) and `gitleaks protect --staged --no-banner` before commit
3. Keep changes focused; update ADRs in `docs/adr/` when making architectural decisions.
4. Ensure CI is green (`preflight` + `CI` workflows).
5. Request review; address feedback with additional commits (avoid force-push to `main`).

## Code guidelines

See [GUIDELINES.md](GUIDELINES.md) for error handling, robustness, security, and testing
expectations.

## License

By contributing, you agree that your contributions are licensed under the project MIT
license.
