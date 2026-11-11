package safety

import (
	"errors"
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// Error is a refusal by the safety layer that is not about a specific
// finding: a confirmation that was not given, an undo that is not possible.
// Compare with errors.Is against the Err… values.
type Error struct {
	key i18n.Key
	// Arg fills the message's placeholder, if it has one.
	Arg string
}

// Refusals.
var (
	// ErrDeclined: the user did not confirm. Nothing changed.
	ErrDeclined = &Error{key: i18n.MsgSafetyDeclined}
	// ErrConfirmationRequired: the operation needs a confirmation nobody was
	// there to give.
	ErrConfirmationRequired = &Error{key: i18n.MsgSafetyConfirmationRequired}
	// ErrRepositoryChanged: the repository is no longer in the state the
	// user confirmed.
	ErrRepositoryChanged = &Error{key: i18n.MsgSafetyRepositoryChanged}

	// ErrUndoNothing: the history holds nothing that can be undone.
	ErrUndoNothing = &Error{key: i18n.MsgSafetyUndoNothing}
	// ErrUndoPushed: the operation reached a remote.
	ErrUndoPushed = &Error{key: i18n.MsgSafetyUndoPushed}
	// ErrUndoMoved: the repository moved on since the operation.
	ErrUndoMoved = &Error{key: i18n.MsgSafetyUndoMoved}
	// ErrUndoDirty: uncommitted changes are in the way.
	ErrUndoDirty = &Error{key: i18n.MsgSafetyUndoDirty}
	// ErrUndoNoBackup: the backup the undo needs is gone.
	ErrUndoNoBackup = &Error{key: i18n.MsgSafetyUndoNoBackup}
	// ErrUndoFirstCommit: the first commit of a repository cannot be undone.
	ErrUndoFirstCommit = &Error{key: i18n.MsgSafetyUndoFirstCommit}
	// ErrUndoBranchExists: a branch with the deleted branch's name exists.
	ErrUndoBranchExists = &Error{key: i18n.MsgSafetyUndoBranchExists}
	// ErrUndoNotSupported: this kind of operation has no automatic undo.
	ErrUndoNotSupported = &Error{key: i18n.MsgSafetyUndoNotSupported}

	// ErrBackupNotFound: no backup with that name.
	ErrBackupNotFound = &Error{key: i18n.MsgSafetyBackupNotFound}
	// ErrBackupUnknownName: the backup is of a deleted branch whose name was
	// not recorded.
	ErrBackupUnknownName = &Error{key: i18n.MsgSafetyBackupUnknownName}
)

// with returns the same refusal with its placeholder filled in.
func (e *Error) with(arg string) *Error { return &Error{key: e.key, Arg: arg} }

func (e *Error) Error() string { return e.Localize(i18n.EN) }

// Localize implements i18n.Localizable.
func (e *Error) Localize(lang i18n.Lang) string {
	if e.Arg == "" {
		return i18n.T(lang, e.key)
	}
	return i18n.T(lang, e.key, e.Arg)
}

// Is makes errors.Is match a refusal whatever its argument.
func (e *Error) Is(target error) bool {
	var other *Error
	return errors.As(target, &other) && other.key == e.key
}

// BlockedError is returned when a finding stops an operation. No
// confirmation overrides it.
type BlockedError struct {
	Assessment Assessment
}

func (e *BlockedError) Error() string { return e.Localize(i18n.EN) }

// Localize implements i18n.Localizable: the refusal, then each reason.
func (e *BlockedError) Localize(lang i18n.Lang) string {
	var b strings.Builder
	b.WriteString(i18n.T(lang, i18n.MsgSafetyBlocked))
	for _, f := range e.Assessment.Findings {
		if f.Severity == SeverityBlock {
			b.WriteString("\n- ")
			b.WriteString(f.Localize(lang))
		}
	}
	return b.String()
}

// AllCodes lists every finding the safety layer can report.
func AllCodes() []Code { return append([]Code(nil), allCodes...) }
