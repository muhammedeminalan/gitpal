package git

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorKind classifies a failed git command so callers can react to the cause
// instead of matching on text.
type ErrorKind string

// Error kinds.
const (
	// KindUnknown is a failure gitpal has no specific explanation for.
	KindUnknown ErrorKind = "unknown"
	// KindGitNotFound means the git executable could not be started.
	KindGitNotFound ErrorKind = "git_not_found"
	// KindDirNotFound means the directory to run in does not exist.
	KindDirNotFound ErrorKind = "dir_not_found"
	// KindTimeout means the command ran longer than allowed and was killed.
	KindTimeout ErrorKind = "timeout"
	// KindCanceled means the caller canceled the command.
	KindCanceled ErrorKind = "canceled"
	// KindNotARepo means the directory is not inside a git repository.
	KindNotARepo ErrorKind = "not_a_repo"

	// KindNothingToCommit: there are no staged changes to commit.
	KindNothingToCommit ErrorKind = "nothing_to_commit"
	// KindIdentityMissing: user.name or user.email is not configured.
	KindIdentityMissing ErrorKind = "identity_missing"
	// KindHookFailed: a local hook (pre-commit, commit-msg, …) rejected the
	// operation. git prints nothing of its own here, so callers that know a
	// hook ran set this kind themselves; the hook's output is in Stderr.
	KindHookFailed ErrorKind = "hook_failed"
	// KindAuthFailed: the remote refused the credentials, or none were available.
	KindAuthFailed ErrorKind = "auth_failed"
	// KindRemoteUnreachable: the remote could not be reached or does not exist.
	KindRemoteUnreachable ErrorKind = "remote_unreachable"
	// KindNoRemote: no remote is configured, or the named one does not exist.
	KindNoRemote ErrorKind = "no_remote"
	// KindRejected: a push was refused because the remote has commits the
	// local branch lacks (non-fast-forward).
	KindRejected ErrorKind = "push_rejected"
	// KindStaleLease: --force-with-lease refused because the remote moved.
	KindStaleLease ErrorKind = "stale_lease"
	// KindRemoteDeclined: a server-side rule or hook (e.g. branch protection)
	// refused the push.
	KindRemoteDeclined ErrorKind = "remote_declined"
	// KindNoUpstream: the branch has no upstream to push to or pull from.
	KindNoUpstream ErrorKind = "no_upstream"
	// KindDiverged: local and remote both have new commits and the chosen
	// strategy (ff-only) cannot reconcile them.
	KindDiverged ErrorKind = "diverged"
	// KindDirtyWorktree: uncommitted changes would be overwritten.
	KindDirtyWorktree ErrorKind = "dirty_worktree"
	// KindConflict: the operation stopped on conflicts, or unresolved
	// conflicts block it.
	KindConflict ErrorKind = "conflict"
	// KindOperationInProgress: a merge, rebase, cherry-pick or revert is
	// already under way.
	KindOperationInProgress ErrorKind = "operation_in_progress"
	// KindDetachedHead: the operation needs a branch but HEAD is detached.
	KindDetachedHead ErrorKind = "detached_head"
	// KindUnbornBranch: the current branch has no commits yet.
	KindUnbornBranch ErrorKind = "unborn_branch"
	// KindBranchExists: a branch with that name already exists.
	KindBranchExists ErrorKind = "branch_exists"
	// KindBranchNotMerged: deleting the branch would lose commits.
	KindBranchNotMerged ErrorKind = "branch_not_merged"
	// KindInvalidName: git rejected a branch or ref name.
	KindInvalidName ErrorKind = "invalid_name"
	// KindPathspecNoMatch: a given path matches no file git knows.
	KindPathspecNoMatch ErrorKind = "pathspec_no_match"
	// KindRefNotFound: a branch, tag, commit or remote ref does not exist.
	KindRefNotFound ErrorKind = "ref_not_found"
	// KindIndexLocked: another git process holds the index lock.
	KindIndexLocked ErrorKind = "index_locked"
)

// allKinds lists every kind gitpal can report, so tests can check that each
// one explains itself to the user.
var allKinds = []ErrorKind{
	KindUnknown, KindGitNotFound, KindDirNotFound, KindTimeout, KindCanceled, KindNotARepo,
	KindNothingToCommit, KindIdentityMissing, KindHookFailed, KindAuthFailed, KindRemoteUnreachable,
	KindNoRemote, KindRejected, KindStaleLease, KindRemoteDeclined, KindNoUpstream, KindDiverged,
	KindDirtyWorktree, KindConflict, KindOperationInProgress, KindDetachedHead, KindUnbornBranch,
	KindBranchExists, KindBranchNotMerged, KindInvalidName, KindPathspecNoMatch, KindRefNotFound,
	KindIndexLocked,
}

// maxErrorText bounds how much of git's output Error.Error prints. The full
// text stays in Error.Stderr.
const maxErrorText = 2000

// Error is a failed git command.
type Error struct {
	Kind ErrorKind
	// Args are the git arguments, with credentials redacted.
	Args []string
	// ExitCode is git's exit status, or -1 if it never exited normally.
	ExitCode int
	// Stderr is what git printed to standard error, trimmed, with
	// credentials redacted.
	Stderr string
	// Err is the underlying error, if any (context or process error).
	Err error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(commandLine(e.Args))
	if e.ExitCode >= 0 {
		fmt.Fprintf(&b, ": exit status %d", e.ExitCode)
	} else if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	if text := truncate(e.Stderr, maxErrorText); text != "" {
		b.WriteString(": ")
		b.WriteString(text)
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Err }

// KindOf returns the kind of a git failure: "" for nil, and KindUnknown for an
// error that did not come from running git.
func KindOf(err error) ErrorKind {
	if err == nil {
		return ""
	}
	var gerr *Error
	if errors.As(err, &gerr) {
		return gerr.Kind
	}
	return KindUnknown
}

// ExitCode returns git's exit status if err is a git command that ran and
// exited non-zero. Some commands use the status as an answer rather than a
// failure (`diff --quiet`, `config --get`).
func ExitCode(err error) (int, bool) {
	var gerr *Error
	if errors.As(err, &gerr) && gerr.ExitCode >= 0 {
		return gerr.ExitCode, true
	}
	return 0, false
}

// stderrKinds maps fragments of git's (English) error output to a kind. The
// first entry with a matching fragment wins, so more specific entries come
// first. Fragments are lower case.
//
// The kind only chooses the explanation shown to the user. Safety decisions
// never depend on it (DECISIONS A-008): callers that must know whether, say, a
// merge stopped on conflicts check the repository state instead.
var stderrKinds = []struct {
	kind      ErrorKind
	fragments []string
}{
	{KindNotARepo, []string{"not a git repository"}},
	{KindOperationInProgress, []string{
		"you have not concluded your merge", "merge_head exists",
		"there is already a rebase-merge directory", "there is already a rebase-apply directory",
		"in the middle of another rebase",
		"cherry-pick is already in progress", "revert is already in progress",
	}},
	{KindIndexLocked, []string{".lock': file exists", "another git process seems to be running"}},
	{KindIdentityMissing, []string{
		"please tell me who you are", "unable to auto-detect email address", "empty ident name",
		"no name was given and auto-detection is disabled", "no email was given and auto-detection is disabled",
	}},
	{KindNothingToCommit, []string{"nothing to commit", "no changes added to commit", "nothing added to commit"}},
	{KindStaleLease, []string{"(stale info)"}},
	{KindRemoteDeclined, []string{"hook declined", "[remote rejected]", "protected branch"}},
	{KindRejected, []string{
		"(non-fast-forward)", "(fetch first)", "updates were rejected",
		"tip of your current branch is behind", "[rejected]",
	}},
	{KindAuthFailed, []string{
		"authentication failed", "could not read username", "could not read password",
		"terminal prompts disabled", "permission denied (publickey", "invalid username or password",
		"returned error: 401", "returned error: 403",
	}},
	{KindNoRemote, []string{"no configured push destination", "no such remote", "no remote repository specified"}},
	{KindRemoteUnreachable, []string{
		"could not resolve host", "connection refused", "connection timed out", "operation timed out",
		"network is unreachable", "failed to connect", "does not appear to be a git repository",
		"repository not found", "could not read from remote repository", "unable to access",
	}},
	{KindNoUpstream, []string{"has no upstream branch", "no tracking information", "no upstream configured"}},
	{KindDiverged, []string{
		"not possible to fast-forward", "diverging branches can't be fast-forwarded",
		"reconcile divergent branches",
	}},
	{KindDirtyWorktree, []string{
		"would be overwritten by", "please commit your changes or stash them",
		"you have unstaged changes", "your index contains uncommitted changes",
	}},
	{KindConflict, []string{
		"you have unmerged files", "you need to resolve your current index first",
		"fix conflicts and then commit", "unmerged paths",
	}},
	{KindDetachedHead, []string{"you are not currently on a branch"}},
	{KindUnbornBranch, []string{"does not have any commits yet"}},
	{KindBranchExists, []string{"a branch named"}},
	{KindBranchNotMerged, []string{"is not fully merged"}},
	{KindInvalidName, []string{"is not a valid branch name", "is not a valid ref name"}},
	{KindPathspecNoMatch, []string{"did not match any file"}},
	{KindRefNotFound, []string{
		"invalid reference", "not a valid object name", "unknown revision", "bad revision",
		"couldn't find remote ref", "' not found",
	}},
}

func classify(stderr string) ErrorKind {
	text := strings.ToLower(stderr)
	for _, entry := range stderrKinds {
		for _, fragment := range entry.fragments {
			if strings.Contains(text, fragment) {
				return entry.kind
			}
		}
	}
	return KindUnknown
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !isRuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + " …"
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
