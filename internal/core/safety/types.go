// Package safety is the safety layer (spec §7): every destructive or risky
// operation is judged, confirmed, backed up and recorded here before it is
// allowed to run.
//
// A caller describes what it wants to do as an Operation and hands it to a
// Guard. Guard.Assess says what would happen (the dry run); Guard.Do asks for
// the confirmation the risk calls for, takes the backups, runs the operation
// through the git package and writes it to the history, from which Guard.Undo
// can take it back.
package safety

import (
	"path"
	"slices"
	"time"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// Risk is how much an operation can cost the user (spec §7.1).
type Risk int

// Risk classes.
const (
	// RiskSafe runs directly.
	RiskSafe Risk = iota
	// RiskCaution is summarized and confirmed with a single yes.
	RiskCaution
	// RiskDangerous can destroy work: it is backed up first and needs an
	// explicit yes that `--yes` cannot give.
	RiskDangerous
	// RiskCritical rewrites what others may already have: the user confirms
	// by typing the branch name.
	RiskCritical
)

func (r Risk) String() string {
	switch r {
	case RiskSafe:
		return "safe"
	case RiskCaution:
		return "caution"
	case RiskDangerous:
		return "dangerous"
	case RiskCritical:
		return "critical"
	}
	return "unknown"
}

// OpKind names an operation. It is also what the history records.
type OpKind string

// Operations the safety layer knows.
const (
	OpCommit       OpKind = "commit"
	OpAmend        OpKind = "amend"
	OpDiscard      OpKind = "discard"
	OpReset        OpKind = "reset"
	OpDeleteBranch OpKind = "delete-branch"
	OpPush         OpKind = "push"
	OpForcePush    OpKind = "force-push"
	OpPull         OpKind = "pull"
	OpSwitch       OpKind = "switch"
)

// Operation is something the caller wants to do. Build one with the
// constructor for its kind.
type Operation struct {
	Kind OpKind
	// CommitOptions is used by OpCommit and OpAmend.
	CommitOptions git.CommitOptions
	// Paths are the files OpDiscard throws the changes of away.
	Paths []string
	// Rev and ResetMode are used by OpReset.
	Rev       string
	ResetMode git.ResetMode
	// Branch is the branch OpDeleteBranch deletes or OpSwitch switches to.
	Branch string
	// Force lets OpDeleteBranch delete a branch with unmerged commits.
	Force bool
	// PushOptions is used by OpPush and OpForcePush.
	PushOptions git.PushOptions
	// PullOptions is used by OpPull.
	PullOptions git.PullOptions
}

// Commit records the staged changes, or amends the last commit when
// opts.Amend is set.
func Commit(opts git.CommitOptions) Operation {
	kind := OpCommit
	if opts.Amend {
		kind = OpAmend
	}
	return Operation{Kind: kind, CommitOptions: opts}
}

// Discard throws away the uncommitted changes to the given files.
func Discard(paths ...string) Operation { return Operation{Kind: OpDiscard, Paths: paths} }

// Reset moves the current branch to rev.
func Reset(mode git.ResetMode, rev string) Operation {
	return Operation{Kind: OpReset, ResetMode: mode, Rev: rev}
}

// DeleteBranch deletes a local branch; force allows losing unmerged commits.
func DeleteBranch(name string, force bool) Operation {
	return Operation{Kind: OpDeleteBranch, Branch: name, Force: force}
}

// Push sends a branch to its remote. With opts.ForceWithLease it is a forced
// push, the most dangerous operation there is.
func Push(opts git.PushOptions) Operation {
	kind := OpPush
	if opts.ForceWithLease {
		kind = OpForcePush
	}
	return Operation{Kind: kind, PushOptions: opts}
}

// Pull fetches and integrates the remote branch.
func Pull(opts git.PullOptions) Operation { return Operation{Kind: OpPull, PullOptions: opts} }

// Switch checks out another branch.
func Switch(branch string) Operation { return Operation{Kind: OpSwitch, Branch: branch} }

// Policy is the part of gitpal's configuration the safety layer follows.
type Policy struct {
	// ProtectedBranches can never be force-pushed, and committing to them
	// directly draws a warning. Entries may use "*" as in "release/*".
	ProtectedBranches []string
	// LargeFileBytes is the size from which a staged file draws a warning;
	// zero turns the check off.
	LargeFileBytes int64
	// WarnOnSecrets turns the scan of staged changes for credentials on.
	WarnOnSecrets bool
	// BackupRetention is how long backups are kept before PruneBackups
	// removes them.
	BackupRetention time.Duration
}

// DefaultPolicy returns the built-in policy (spec §10).
func DefaultPolicy() Policy {
	return Policy{
		ProtectedBranches: []string{"main", "master", "develop"},
		LargeFileBytes:    5 << 20,
		WarnOnSecrets:     true,
		BackupRetention:   30 * 24 * time.Hour,
	}
}

// Protected reports whether a branch is protected.
func (p Policy) Protected(branch string) bool {
	return slices.ContainsFunc(p.ProtectedBranches, func(pattern string) bool {
		if pattern == branch {
			return true
		}
		// A pattern path.Match cannot read is simply a name that did not match.
		matched, err := path.Match(pattern, branch)
		return err == nil && matched
	})
}

// Severity says what a finding does to the operation.
type Severity int

// Severities.
const (
	// SeverityInfo tells the user something that needs no decision.
	SeverityInfo Severity = iota
	// SeverityWarn asks the user to look before going on.
	SeverityWarn
	// SeverityBlock stops the operation; no confirmation overrides it.
	SeverityBlock
)

// Code identifies a finding.
type Code string

// Findings.
const (
	// CodeOperationInProgress: a merge or rebase is under way (rule 9).
	CodeOperationInProgress Code = "operation_in_progress"
	// CodeProtectedForcePush: a forced push to a protected branch (rule 1).
	CodeProtectedForcePush Code = "protected_force_push"
	// CodeBehindRemote: the remote has commits the branch lacks (rule 5).
	CodeBehindRemote Code = "behind_remote"
	// CodeRewritesPushed: commits that are already on a remote would be
	// rewritten or dropped (rule 3).
	CodeRewritesPushed Code = "rewrites_pushed"
	// CodeProtectedBranchCommit: a commit directly on a protected branch
	// (rule 6).
	CodeProtectedBranchCommit Code = "protected_branch_commit"
	// CodeDetachedHeadCommit: a commit on no branch (rule 8).
	CodeDetachedHeadCommit Code = "detached_head_commit"
	// CodeSecret: a staged line looks like a credential (rule 7).
	CodeSecret Code = "secret"
	// CodeSecretFile: a staged file is of a kind that holds credentials.
	CodeSecretFile Code = "secret_file"
	// CodeLargeFile: a staged file is larger than the configured limit.
	CodeLargeFile Code = "large_file"
	// CodeJunkFile: staged dependencies or build output.
	CodeJunkFile Code = "junk_file"
	// CodeUncommittedChanges: uncommitted changes are backed up first (rule 4).
	CodeUncommittedChanges Code = "uncommitted_changes"
	// CodeUnmergedBranch: the branch has commits that exist nowhere else.
	CodeUnmergedBranch Code = "unmerged_branch"
	// CodeProtectedBranchDelete: a protected branch is about to be deleted.
	CodeProtectedBranchDelete Code = "protected_branch_delete"
	// CodeForcePush: commits on the remote will be replaced.
	CodeForcePush Code = "force_push"
)

// Finding is one thing the safety layer noticed about an operation.
type Finding struct {
	Code     Code
	Severity Severity
	// Path and Line locate a finding in a file; Line is zero when unknown.
	Path string
	Line int
	// Detail names what was found: the kind of secret, a branch, a size.
	// It is never the secret itself.
	Detail string
	// Count is how many items the finding stands for.
	Count int
}

// Localize implements i18n.Localizable.
func (f Finding) Localize(lang i18n.Lang) string {
	key, ok := findingMessages[f.Code]
	if !ok {
		return string(f.Code)
	}
	switch f.Code {
	case CodeSecret:
		return i18n.T(lang, key, f.Path, f.Line, f.Detail)
	case CodeSecretFile:
		return i18n.T(lang, key, f.Path)
	case CodeLargeFile:
		return i18n.T(lang, key, f.Path, f.Detail)
	case CodeJunkFile:
		return i18n.T(lang, key, f.Path, f.Count)
	case CodeOperationInProgress, CodeProtectedForcePush, CodeProtectedBranchCommit, CodeProtectedBranchDelete:
		return i18n.T(lang, key, f.Detail)
	case CodeBehindRemote, CodeRewritesPushed, CodeUnmergedBranch, CodeForcePush, CodeUncommittedChanges:
		return i18n.T(lang, key, f.Count)
	case CodeDetachedHeadCommit:
	}
	return i18n.T(lang, key)
}

var findingMessages = map[Code]i18n.Key{
	CodeOperationInProgress:   i18n.MsgSafetyOperationInProgress,
	CodeProtectedForcePush:    i18n.MsgSafetyProtectedForcePush,
	CodeBehindRemote:          i18n.MsgSafetyBehindRemote,
	CodeRewritesPushed:        i18n.MsgSafetyRewritesPushed,
	CodeProtectedBranchCommit: i18n.MsgSafetyProtectedBranchCommit,
	CodeDetachedHeadCommit:    i18n.MsgSafetyDetachedHeadCommit,
	CodeSecret:                i18n.MsgSafetySecret,
	CodeSecretFile:            i18n.MsgSafetySecretFile,
	CodeLargeFile:             i18n.MsgSafetyLargeFile,
	CodeJunkFile:              i18n.MsgSafetyJunkFile,
	CodeUncommittedChanges:    i18n.MsgSafetyUncommittedChanges,
	CodeUnmergedBranch:        i18n.MsgSafetyUnmergedBranch,
	CodeProtectedBranchDelete: i18n.MsgSafetyProtectedBranchDelete,
	CodeForcePush:             i18n.MsgSafetyForcePush,
}

// allCodes lists every finding, so a test can check each explains itself.
var allCodes = []Code{
	CodeOperationInProgress, CodeProtectedForcePush, CodeBehindRemote, CodeRewritesPushed,
	CodeProtectedBranchCommit, CodeDetachedHeadCommit, CodeSecret, CodeSecretFile, CodeLargeFile,
	CodeJunkFile, CodeUncommittedChanges, CodeUnmergedBranch, CodeProtectedBranchDelete, CodeForcePush,
}

// EffectKind names something an operation will do (the dry-run summary,
// rule 10).
type EffectKind string

// Effects.
const (
	// EffectFilesDiscarded: uncommitted changes to these files are thrown away.
	EffectFilesDiscarded EffectKind = "files_discarded"
	// EffectCommitsDropped: these commits leave the branch.
	EffectCommitsDropped EffectKind = "commits_dropped"
	// EffectBranchDeleted: the branch is deleted.
	EffectBranchDeleted EffectKind = "branch_deleted"
	// EffectRemoteCommitsReplaced: commits on the remote branch are replaced.
	EffectRemoteCommitsReplaced EffectKind = "remote_commits_replaced"
	// EffectCommitsPushed: these commits are sent to the remote.
	EffectCommitsPushed EffectKind = "commits_pushed"
	// EffectCommitRewritten: the last commit is replaced by a new one.
	EffectCommitRewritten EffectKind = "commit_rewritten"
)

// Effect is one consequence of an operation.
type Effect struct {
	Kind  EffectKind
	Count int
	// Items name what is affected: file paths, a branch, short commit hashes.
	Items []string
}

// Confirmation is how the user must agree to an operation.
type Confirmation int

// Confirmations.
const (
	// ConfirmNone: nothing to confirm.
	ConfirmNone Confirmation = iota
	// ConfirmSimple: one yes; `--yes` may give it.
	ConfirmSimple
	// ConfirmExplicit: a yes from the user in person.
	ConfirmExplicit
	// ConfirmTypeBranch: the user types the branch name.
	ConfirmTypeBranch
)

// Assessment is the safety layer's judgment of an operation: the dry run.
type Assessment struct {
	Op       Operation
	Risk     Risk
	Findings []Finding
	Effects  []Effect
	Confirm  Confirmation
	// ConfirmText is what the user must type for ConfirmTypeBranch.
	ConfirmText string
	// BackupCommit is the commit a backup ref will keep; empty for none.
	BackupCommit string
	// BackupWorktree is set when uncommitted changes will be backed up.
	BackupWorktree bool

	// branch and head are the state the judgment was made in.
	branch, head string
	// unmerged is how many commits deleting a branch would lose.
	unmerged int
}

// Blocked reports whether a finding stops the operation.
func (a Assessment) Blocked() bool {
	return slices.ContainsFunc(a.Findings, func(f Finding) bool { return f.Severity == SeverityBlock })
}
