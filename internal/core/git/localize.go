package git

import (
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// kindMessages explains each kind to the user (spec §11: git's errors are
// translated into plain language).
var kindMessages = map[ErrorKind]i18n.Key{
	KindUnknown:             i18n.MsgGitFailed,
	KindGitNotFound:         i18n.MsgGitKindGitNotFound,
	KindDirNotFound:         i18n.MsgGitKindDirNotFound,
	KindTimeout:             i18n.MsgGitKindTimeout,
	KindCanceled:            i18n.MsgGitKindCanceled,
	KindNotARepo:            i18n.MsgGitKindNotARepo,
	KindNothingToCommit:     i18n.MsgGitKindNothingToCommit,
	KindIdentityMissing:     i18n.MsgGitKindIdentityMissing,
	KindHookFailed:          i18n.MsgGitKindHookFailed,
	KindAuthFailed:          i18n.MsgGitKindAuthFailed,
	KindRemoteUnreachable:   i18n.MsgGitKindRemoteUnreachable,
	KindNoRemote:            i18n.MsgGitKindNoRemote,
	KindRejected:            i18n.MsgGitKindRejected,
	KindStaleLease:          i18n.MsgGitKindStaleLease,
	KindRemoteDeclined:      i18n.MsgGitKindRemoteDeclined,
	KindNoUpstream:          i18n.MsgGitKindNoUpstream,
	KindDiverged:            i18n.MsgGitKindDiverged,
	KindDirtyWorktree:       i18n.MsgGitKindDirtyWorktree,
	KindConflict:            i18n.MsgGitKindConflict,
	KindOperationInProgress: i18n.MsgGitKindOperationInProgress,
	KindDetachedHead:        i18n.MsgGitKindDetachedHead,
	KindUnbornBranch:        i18n.MsgGitKindUnbornBranch,
	KindBranchExists:        i18n.MsgGitKindBranchExists,
	KindBranchNotMerged:     i18n.MsgGitKindBranchNotMerged,
	KindBranchCheckedOut:    i18n.MsgGitKindBranchCheckedOut,
	KindAmbiguousRef:        i18n.MsgGitKindAmbiguousRef,
	KindInvalidName:         i18n.MsgGitKindInvalidName,
	KindPathspecNoMatch:     i18n.MsgGitKindPathspecNoMatch,
	KindPathIgnored:         i18n.MsgGitKindPathIgnored,
	KindRefNotFound:         i18n.MsgGitKindRefNotFound,
	KindIndexLocked:         i18n.MsgGitKindIndexLocked,
}

// Localize implements i18n.Localizable: a plain-language explanation of the
// failure, followed by git's own output. The output is kept because it often
// is the actual answer (a hook's lint errors, the list of conflicting files).
func (e *Error) Localize(lang i18n.Lang) string {
	var text string
	if key, ok := kindMessages[e.Kind]; ok && e.Kind != KindUnknown {
		text = i18n.T(lang, key)
	} else {
		text = i18n.T(lang, i18n.MsgGitFailed, commandLine(e.Args), e.ExitCode)
	}
	if out := truncate(e.Stderr, maxErrorText); out != "" {
		text += "\n\n" + i18n.T(lang, i18n.MsgGitOutput) + "\n" + indent(out)
	}
	return text
}

func indent(s string) string {
	return "  " + strings.ReplaceAll(s, "\n", "\n  ")
}

// ArgCode classifies an argument gitpal refused before running git.
type ArgCode string

// Argument codes.
const (
	// ArgNoPaths: an operation on files was given no files.
	ArgNoPaths ArgCode = "no_paths"
	// ArgInvalidPath: a path is empty, absolute, contains a NUL byte or leaves
	// the repository.
	ArgInvalidPath ArgCode = "invalid_path"
	// ArgEmptyMessage: a commit message is empty.
	ArgEmptyMessage ArgCode = "empty_message"
	// ArgInvalidBranch: a branch name is empty, starts with "-" or contains
	// whitespace or control characters.
	ArgInvalidBranch ArgCode = "invalid_branch"
	// ArgInvalidRemote: a remote name is unusable in the same way.
	ArgInvalidRemote ArgCode = "invalid_remote"
	// ArgInvalidRev: a revision (commit, tag, range) is unusable in the same way.
	ArgInvalidRev ArgCode = "invalid_rev"
	// ArgInvalidStrategy: a pull strategy is none of merge, rebase, ff-only.
	ArgInvalidStrategy ArgCode = "invalid_strategy"
	// ArgUnsupportedFile: a file's changes cannot be staged hunk by hunk
	// (binary, new, deleted, renamed), or a hunk does not exist.
	ArgUnsupportedFile ArgCode = "unsupported_file"
)

var allArgCodes = []ArgCode{
	ArgNoPaths, ArgInvalidPath, ArgEmptyMessage, ArgInvalidBranch, ArgInvalidRemote, ArgInvalidRev,
	ArgInvalidStrategy, ArgUnsupportedFile,
}

var argMessages = map[ArgCode]i18n.Key{
	ArgNoPaths:       i18n.MsgGitArgNoPaths,
	ArgInvalidPath:   i18n.MsgGitArgInvalidPath,
	ArgEmptyMessage:  i18n.MsgGitArgEmptyMessage,
	ArgInvalidBranch: i18n.MsgGitArgInvalidBranch,
	ArgInvalidRemote: i18n.MsgGitArgInvalidRemote,
	ArgInvalidRev:    i18n.MsgGitArgInvalidRev,

	ArgInvalidStrategy: i18n.MsgGitArgInvalidStrategy,
	ArgUnsupportedFile: i18n.MsgGitArgUnsupportedFile,
}

// ArgError reports an argument refused before git was started. It is not a
// git failure: KindOf reports it as KindUnknown and git never ran.
type ArgError struct {
	Code ArgCode
	// Value is the refused argument, as given.
	Value string
}

func (e *ArgError) Error() string { return e.Localize(i18n.EN) }

// Localize implements i18n.Localizable.
func (e *ArgError) Localize(lang i18n.Lang) string {
	key, ok := argMessages[e.Code]
	if !ok {
		return string(e.Code)
	}
	if e.Code == ArgNoPaths || e.Code == ArgEmptyMessage {
		return i18n.T(lang, key)
	}
	return i18n.T(lang, key, e.Value)
}
