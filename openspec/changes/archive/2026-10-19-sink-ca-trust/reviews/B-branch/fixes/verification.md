# Stage-B fixes — review round 1, verification

Diff `09541cb7c..387ef99bf` (fixes 8.1-8.8), brief `brief.md`. Legs, run in parallel and waited on
by PID: astra (`codex exec --sandbox read-only`, exit 0): **REQUEST_CHANGES**; claude/opus:diff
(`fanout.sh --no-unify --leg-timeout 1800`, exit 0): **CONCERNS**. astra ran the focused unit and
S3 integration tests (pass; `internal/sink/git` failed two mirror tests under its sandbox umask, the
known ENVIRONMENT case). opus ran nothing.

| Id | Finding | Leg, sev | Verified? (evidence) | Disposition |
|---|---|---|---|---|
| FX1 | 8.2 covers only a refused downgrade: the redirect limit, an unparsable `Location` and a failed followed hop still name the server-sent target | astra WARNING, opus WARNING | yes: Go `net/http/client.go:662-665` quotes an unparsable `Location`; `:731-738` sets `URL = loc` for any `CheckRedirect` error (the limit); a failed hop's `uerr` names `req.URL`, the hop. Red: three subtests of the new test named `127.0.0.1:<port>/sentinel-redirect` | fix 8.9: `ErrRedirectFailed` for those, killed by two mutants |
| FX2 | The 8.5 comment says `applyCA` judges a DSN mode the reader missed, which is the case it cannot judge; suggests pinning last for URLs | opus WARNING (60) | partly: the comment overclaims. The behaviour claim is rejected: a missed weaker mode is either plaintext (rejected by `applyCA`), or `prefer`+`direct`, which gets the same chain check `require` and `verify-ca` have, and under verify-full `applyCA` rejects any config without the hostname check (`ca_trust.go:357-373`) | fix 8.10: comment and design note state this; no behaviour change, so no URL-only pin |
| FX3 | The pin test is a property test; no case where reader and pgx split | opus NOTE | yes | fix 8.10: tasks 8.5 says so; no splitting DSN is known (`round-1/verification.md` D2) |
| FX4 | `errURIRefused` is retryable even for a permanent bad port | opus NOTE | yes | rejected: same class as before this change (`mongo.Connect`'s error), and the SRV case can be transient; telling them apart means parsing driver text again |
| FX5 | `ResolveSinkSecretData` doc still says `spec.secretRef` first | opus NOTE | yes (`wire.go:72`) | fix 8.10 |
| — | 8.2 `nil, err` keeps SDK retries (`RequestSendError` is retryable) | astra note | yes, and unchanged by the fix (the `*url.Error` was retried the same way, `exceeded maximum number of attempts, 3` in the red run) | no change |

Both legs found 8.1 and 8.3-8.8 sound (8.5: no downgrade or broken DSN form found).

FX1 was a behaviour defect, so its fix needs another look: branch round 2 (`round-2/`) runs fresh
legs over all of section 8, which is also the second and last review round of these fixes.
