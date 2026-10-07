// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func gitInWorkdir(ctx context.Context, workdir string, cli *cliEnv, args ...string) *exec.Cmd {
	gitPath, resolveErr := resolveGitExecutable()

	argv := make([]string, 0, 4+len(args))
	if resolveErr != nil {
		// Preserve prior error taxonomy: bare "git" missing yields *exec.Error at Run.
		// Still avoid ambient PATH lookup by stashing the resolve error on Cmd.Err.
		argv = append(argv, "git")
	} else {
		argv = append(argv, gitPath)
	}
	if cli != nil {
		argv = append(argv, cli.prependGitArgs("-C", workdir)...)
	} else {
		argv = append(argv, "-C", workdir)
	}
	argv = append(argv, args...)
	//nolint:gosec // G204: workdir validated by validateGitWorkdir before call; gitPath pinned via resolveGitExecutable
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if resolveErr != nil {
		cmd.Err = resolveErr
	}
	applyCLIEnv(cmd, cli)

	return cmd
}

func gitCloneCmd(ctx context.Context, cli *cliEnv, args ...string) *exec.Cmd {
	gitPath, resolveErr := resolveGitExecutable()

	bin := "git"
	if resolveErr == nil {
		bin = gitPath
	}

	// Global options (--config-env for the auth header) MUST precede the
	// subcommand: `git clone --config-env ...` is rejected by git as an unknown
	// clone option. This ordering is what makes the default extraHeader path work
	// for clone (K-16).
	argv := make([]string, 0, 2+len(args))
	argv = append(argv, bin)
	if cli != nil {
		argv = append(argv, cli.configEnvArgs...)
	}
	argv = append(argv, "clone")
	argv = append(argv, args...)

	//nolint:gosec // G204: cloneURL, workdir, and branch validated before call; gitPath pinned via resolveGitExecutable
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if resolveErr != nil {
		cmd.Err = resolveErr
	}
	applyCLIEnv(cmd, cli)

	return cmd
}

func gitClone(ctx context.Context, workdir, cloneURL, branch string, depth int, cli *cliEnv) (cloned bool, err error) {
	if validateErr := ValidateGitRef(branch); validateErr != nil {
		return false, fmt.Errorf("git export: invalid branch: %w", validateErr)
	}

	safeURL, err := canonicalCloneURL(cloneURL)
	if err != nil {
		return false, fmt.Errorf("git export: %w", err)
	}

	workdir, err = validateGitWorkdir(workdir)
	if err != nil {
		return false, fmt.Errorf("git export: %w", err)
	}

	var cloneArgs []string
	if depth > 0 {
		cloneArgs = []string{"--branch", branch, "--single-branch", "--depth", strconv.Itoa(depth), "--", safeURL, workdir}
	} else {
		cloneArgs = []string{"--branch", branch, "--single-branch", "--", safeURL, workdir}
	}

	var out []byte
	retryErr := withTransportRetry(ctx, defaultTransportRetry(), func() error {
		cmd := gitCloneCmd(ctx, cli, cloneArgs...)
		out, err = cmd.CombinedOutput()
		if err == nil {
			return nil
		}

		if isCLIEmptyRemote(string(out), err) {
			return nil
		}

		return fmt.Errorf("git clone: %s: %w", cli.redact(strings.TrimSpace(string(out))), err)
	})
	if retryErr != nil {
		return false, retryErr
	}

	if err == nil {
		return true, nil
	}

	if isCLIEmptyRemote(string(out), err) {
		return false, nil
	}

	return false, fmt.Errorf("git clone: %s: %w", cli.redact(strings.TrimSpace(string(out))), err)
}

func gitInit(ctx context.Context, workdir string, cli *cliEnv) error {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "init")
	return runGitOutput(cmd, "init", cli)
}

func gitCheckoutNewBranch(ctx context.Context, workdir, branch string, cli *cliEnv) error {
	if err := ValidateGitRef(branch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "checkout", "-B", branch)
	return runGitOutput(cmd, "checkout -B "+branch, cli)
}

// gitRefExists reports whether the given revision resolves in workdir. A
// missing ref, or a workdir that is not yet a repository, is (false, nil).
func gitRefExists(ctx context.Context, workdir, ref string, cli *cliEnv) (bool, error) {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return false, fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "rev-parse", "--verify", "--quiet", ref)

	return cmd.Run() == nil, nil
}

// gitCheckoutPushBranch checks out pushBranch without resetting an existing
// branch: `git checkout -B` would rebase this inventory's work onto the
// mirror's current HEAD, which may be another inventory's feature tip. A branch
// that does not exist locally is created from the just-fetched clone-branch tip
// (origin/<cloneBranch>) so a synthesized branch can never carry another
// inventory's unmerged export; an empty remote with no origin/<cloneBranch>
// falls back to creating at the current HEAD.
func gitCheckoutPushBranch(ctx context.Context, workdir, cloneBranch, pushBranch string, cli *cliEnv) error {
	if err := ValidateGitRef(pushBranch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}
	if err := ValidateGitRef(cloneBranch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	if _, existed, err := gitRefHash(ctx, workdir, pushBranch, cli); err != nil {
		return err
	} else if existed {
		cmd := gitInWorkdir(ctx, workdir, cli, "checkout", pushBranch)
		return runGitOutput(cmd, "checkout "+pushBranch, cli)
	}

	base := ""
	if exists, err := gitRefExists(ctx, workdir, "refs/remotes/origin/"+cloneBranch, cli); err != nil {
		return err
	} else if exists {
		base = "origin/" + cloneBranch
	}

	args := []string{"checkout", "-B", pushBranch}
	if base != "" {
		args = append(args, base)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, args...)
	return runGitOutput(cmd, "checkout -B "+pushBranch, cli)
}

func gitRemoteAddOrigin(ctx context.Context, workdir, cloneURL string, cli *cliEnv) error {
	safeURL, err := canonicalCloneURL(cloneURL)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	workdir, err = validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "remote", "add", defaultRemote, safeURL)
	return runGitOutput(cmd, "remote add origin", cli)
}

func gitAddPath(ctx context.Context, workdir, objectPath string, cli *cliEnv) error {
	validatedPath, err := validateObjectPath(objectPath)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	workdir, err = validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "add", validatedPath)
	return runGitOutput(cmd, "add "+validatedPath, cli)
}

func gitAddAll(ctx context.Context, workdir string, cli *cliEnv) error {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "add", "-A")
	return runGitOutput(cmd, "add -A", cli)
}

// gitAddPaths stages exactly the given paths (deletions included, via -A pathspec
// semantics) so a cleanup commit never sweeps unrelated worktree dirt out of a
// shared warm mirror into the deletion commit.
func gitAddPaths(ctx context.Context, workdir string, paths []string, cli *cliEnv) error {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	args := append([]string{"add", "-A", "--"}, paths...)
	cmd := gitInWorkdir(ctx, workdir, cli, args...)

	return runGitOutput(cmd, "add -A -- <cleanup paths>", cli)
}

func gitCommit(ctx context.Context, workdir, authorName, authorEmail string, commit renderedCommit, cli *cliEnv) error {
	return gitCommitScoped(ctx, workdir, authorName, authorEmail, commit, nil, cli)
}

// gitCommitScoped commits only the given paths (git commit -- <paths>): the
// commit is built from HEAD plus the current worktree state of those paths, so
// unrelated staged or unstaged state in a shared warm mirror can never ride
// along under the commit's subject. A nil/empty paths list commits the index
// (the export path's existing semantics).
func gitCommitScoped(
	ctx context.Context,
	workdir, authorName, authorEmail string,
	commit renderedCommit,
	paths []string,
	cli *cliEnv,
) error {
	if err := validateGitConfigValue(authorName); err != nil {
		return fmt.Errorf("git export: invalid author name: %w", err)
	}

	if err := validateGitConfigValue(authorEmail); err != nil {
		return fmt.Errorf("git export: invalid author email: %w", err)
	}

	if err := validateGitCommitMessage(commit.Subject); err != nil {
		return fmt.Errorf("git export: invalid commit message: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	args := []string{
		"-c", "user.name=" + authorName,
		"-c", "user.email=" + authorEmail,
		"commit",
		"-m", commit.Subject,
	}
	if commit.Body != "" {
		args = append(args, "-m", commit.Body)
	}

	for _, line := range commit.Trailers {
		args = append(args, "-m", line)
	}

	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, args...)
	return runGitOutput(cmd, "commit", cli)
}

func gitPushOrigin(ctx context.Context, workdir string, force bool, branch string, cli *cliEnv) error {
	if err := ValidateGitRef(branch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	var cmd *exec.Cmd
	if force {
		cmd = gitInWorkdir(ctx, workdir, cli, "push", "--force", "-u", defaultRemote, branch)
	} else {
		cmd = gitInWorkdir(ctx, workdir, cli, "push", "-u", defaultRemote, branch)
	}

	return runGitOutput(cmd, "push", cli)
}

func gitFetchShallow(ctx context.Context, workdir, branch string, depth int, cli *cliEnv) error {
	if err := ValidateGitRef(branch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	args := []string{"fetch", defaultRemote, branch}
	if depth > 0 {
		args = append(args, "--depth", strconv.Itoa(depth))
	}

	cmd := gitInWorkdir(ctx, workdir, cli, args...)
	return runGitOutput(cmd, "fetch", cli)
}

func gitPullRebase(ctx context.Context, workdir string, branch string, cli *cliEnv) error {
	if err := ValidateGitRef(branch); err != nil {
		return fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "pull", "--rebase", defaultRemote, branch)
	return runGitOutput(cmd, "pull --rebase", cli)
}

// gitResetHard clears every staged and unstaged tracked change in the mirror
// worktree (git reset --hard). Staged deletions a crashed cleanup left behind
// are restored by the reset, so the retry re-deletes them; a deletion commit
// already stranded at HEAD is kept (the reset never moves HEAD). No-op on an
// unborn HEAD (nothing committed: the index cannot hold tracked dirt).
func gitResetHard(ctx context.Context, workdir string, cli *cliEnv) error {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	head := gitInWorkdir(ctx, workdir, cli, "rev-parse", "--verify", "-q", "HEAD")
	if _, headErr := head.CombinedOutput(); headErr != nil {
		// Unborn HEAD: no commit exists, so reset has nothing to reset to and
		// any staged state is protected from the commit by scoped commits.
		return nil
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "reset", "--hard")
	return runGitOutput(cmd, "reset --hard", cli)
}

// gitResetHardTo moves the checked-out branch and worktree to commit (a commit
// hash; never a user-supplied ref).
func gitResetHardTo(ctx context.Context, workdir, commit string, cli *cliEnv) error {
	if !isCommitHash(commit) {
		return fmt.Errorf("git export: reset target %q is not a commit hash", commit)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "reset", "--hard", commit)
	return runGitOutput(cmd, "reset --hard "+commit, cli)
}

// gitRevParse resolves a fully qualified ref to its commit hash. A ref that
// does not resolve is ("", false, nil).
func gitRevParse(ctx context.Context, workdir, ref string, cli *cliEnv) (string, bool, error) {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return "", false, fmt.Errorf("git export: %w", err)
	}

	out, err := gitInWorkdir(ctx, workdir, cli, "rev-parse", "--verify", "--quiet", ref+"^{commit}").Output()
	if err != nil {
		return "", false, nil
	}

	hash := strings.TrimSpace(string(out))

	return hash, hash != "", nil
}

// gitCleanFd removes untracked files and directories from the mirror worktree
// (git clean -fd): leftover writes of a crashed export, which the mirror — a
// cache, not user state — must not accumulate. Nested git repositories are
// deliberately left alone (no -ff): nothing kollect writes creates them.
func gitCleanFd(ctx context.Context, workdir string, cli *cliEnv) error {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "clean", "-fd")
	return runGitOutput(cmd, "clean -fd", cli)
}

func gitStatusPorcelain(ctx context.Context, workdir string, cli *cliEnv) (string, error) {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return "", fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git status: %s: %w", cli.redact(strings.TrimSpace(string(out))), err)
	}

	return string(out), nil
}

// remoteHasLocalHead reports whether origin already holds the local HEAD commit on pushBranch.
// It compares `git rev-parse HEAD` against the remote-side value of refs/heads/<pushBranch> from
// `git ls-remote`, so it reflects what the remote actually has -- not a possibly-unfetched local
// remote-tracking ref. A branch absent on the remote (empty ls-remote output) is "not synced",
// so a first push of a new branch still happens. This underpins REL-06: it lets the export push a
// clean-tree-but-stranded snapshot instead of silently reporting success.
func remoteHasLocalHead(ctx context.Context, workdir, pushBranch string, cli *cliEnv) (bool, error) {
	if err := ValidateGitRef(pushBranch); err != nil {
		return false, fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return false, fmt.Errorf("git export: %w", err)
	}

	headCmd := gitInWorkdir(ctx, workdir, cli, "rev-parse", "HEAD")
	headOut, err := headCmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git rev-parse HEAD: %s: %w", cli.redact(strings.TrimSpace(string(headOut))), err)
	}
	localHead := strings.TrimSpace(string(headOut))

	ref := "refs/heads/" + pushBranch
	lsCmd := gitInWorkdir(ctx, workdir, cli, "ls-remote", defaultRemote, ref)
	lsOut, err := lsCmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git ls-remote origin %s: %s: %w", ref, cli.redact(strings.TrimSpace(string(lsOut))), err)
	}

	return remoteSHAFromLsRemote(string(lsOut)) == localHead && localHead != "", nil
}

// remoteSHAFromLsRemote extracts the commit SHA from the first line of `git ls-remote` output
// (tab-delimited "<sha>\t<ref>"). It returns "" when the remote has no such ref.
func remoteSHAFromLsRemote(out string) string {
	line := strings.TrimSpace(out)
	if line == "" {
		return ""
	}

	if idx := strings.IndexAny(line, " \t"); idx >= 0 {
		return line[:idx]
	}

	return line
}

// gitRefHash resolves the local branch ref to its commit hash and reports
// whether it exists. A missing ref (or a workdir that is not yet a repository)
// is ("", false, nil): callers use existence only, never an error, to decide
// whether a tip pre-existed an operation.
func gitRefHash(ctx context.Context, workdir, branch string, cli *cliEnv) (string, bool, error) {
	if err := ValidateGitRef(branch); err != nil {
		return "", false, fmt.Errorf("git export: invalid branch: %w", err)
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return "", false, fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", false, nil
	}

	hash := strings.TrimSpace(string(out))
	if hash == "" {
		return "", false, nil
	}

	return hash, true, nil
}

// gitHeadHash returns the worktree HEAD commit hash.
func gitHeadHash(ctx context.Context, workdir string, cli *cliEnv) (string, error) {
	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return "", fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "rev-parse", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %s: %w", cli.redact(strings.TrimSpace(string(out))), err)
	}

	return strings.TrimSpace(string(out)), nil
}

// gitIsAncestorOfHead reports whether ancestor is an ancestor of HEAD, i.e.
// whether HEAD can be fast-forwarded from ancestor. Anything that is not a
// plain commit hash, or an ancestry that cannot be established, is reported as
// false so a delivery is never forced over unverifiable remote state.
func gitIsAncestorOfHead(ctx context.Context, workdir, ancestor string, cli *cliEnv) (bool, error) {
	if !isCommitHash(ancestor) {
		return false, nil
	}

	workdir, err := validateGitWorkdir(workdir)
	if err != nil {
		return false, fmt.Errorf("git export: %w", err)
	}

	cmd := gitInWorkdir(ctx, workdir, cli, "merge-base", "--is-ancestor", ancestor, "HEAD")
	if cmd.Run() == nil {
		return true, nil
	}

	return false, nil
}

func isCommitHash(hash string) bool {
	if len(hash) != 40 {
		return false
	}

	for _, c := range hash {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}

	return true
}

func runGitOutput(cmd *exec.Cmd, label string, cli *cliEnv) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", label, cli.redact(strings.TrimSpace(string(out))), err)
	}

	return nil
}
