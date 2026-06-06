# Git sink — third-party attribution

kollect's git sink (`internal/sink/git/`) incorporates patterns adapted from
[Argo CD Image Updater](https://github.com/argoproj-labs/argocd-image-updater),
Copyright The Argo Project Authors, licensed under the
[Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0).

Adapted areas include:

- SSH host key verification and configurable known_hosts handling
- HTTPS token username convention for GitHub-style personal access tokens
- Exponential backoff retry for transient git transport failures
- Heuristics for classifying transient vs terminal push errors

Source references in the local-only `references/argocd-image-updater/` tree (not
shipped with kollect):

| kollect file | Reference path |
| --- | --- |
| `internal/sink/git/ssh_auth.go` | `ext/git/ssh.go`, `ext/git/client.go` |
| `internal/sink/git/retry.go` | `ext/git/client.go` (`LsRemote` retry loop) |
| `internal/sink/git/errors.go` | `ext/git/client.go` (transient error signals) |
| `internal/sink/git/auth.go` | `ext/git/client.go`, `ext/git/creds.go` |

Apache 2.0 requires retaining copyright and license notices in derivative
works. Adapted Go source files include an inline attribution comment pointing
to the upstream path.

No Argo CD Image Updater code is copied verbatim; implementations are rewritten
to fit kollect's go-git export path and CRD model.
