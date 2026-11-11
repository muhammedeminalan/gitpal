package safety

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/history"
)

// Guard is the gate in front of one repository's risky operations.
type Guard struct {
	git     *git.Runner
	dir     string
	policy  Policy
	history *history.Store
	now     func() time.Time
}

// GuardOption adjusts a Guard.
type GuardOption func(*Guard)

// WithClock replaces the clock backups and history entries are stamped with.
func WithClock(now func() time.Time) GuardOption { return func(g *Guard) { g.now = now } }

// NewGuard returns the guard for the working tree rooted at dir.
func NewGuard(ctx context.Context, runner *git.Runner, dir string, policy Policy, opts ...GuardOption) (*Guard, error) {
	path, err := runner.GitPath(ctx, dir, history.FileName)
	if err != nil {
		return nil, err
	}
	g := &Guard{git: runner, dir: dir, policy: policy, history: history.Open(path), now: time.Now}
	for _, opt := range opts {
		opt(g)
	}
	return g, nil
}

// History returns the record of what was done in this repository.
func (g *Guard) History() *history.Store { return g.history }

// Answer is the user's reply to a request for confirmation.
type Answer struct {
	Approved bool
	// Typed is what the user typed when asked to type the branch name.
	Typed string
}

// Approver shows an assessment to the user and returns their answer.
type Approver func(Assessment) (Answer, error)

// DoOptions adjusts Do.
type DoOptions struct {
	// AssumeYes answers yes to a simple confirmation (the `--yes` flag). It
	// never covers a dangerous or critical operation.
	AssumeYes bool
}

// Result is what an operation did.
type Result struct {
	Assessment Assessment
	// Entry is the operation as written to the history.
	Entry history.Entry
	// Commit, Push and Pull are set by the operations of that kind.
	Commit git.CommitResult
	Push   git.PushResult
	Pull   git.PullResult
}

// Do runs an operation through the safety layer: it is assessed, refused if
// a rule blocks it (*BlockedError), confirmed in the way its risk calls for
// (ErrDeclined, ErrConfirmationRequired), backed up, run and recorded.
//
// approve may be nil when nobody can be asked; only operations that need no
// confirmation, or a simple one with AssumeYes, run then.
func (g *Guard) Do(ctx context.Context, op Operation, approve Approver, opts DoOptions) (Result, error) {
	a, err := g.Assess(ctx, op)
	if err != nil {
		return Result{}, err
	}
	res := Result{Assessment: a}
	if a.Blocked() {
		return res, &BlockedError{Assessment: a}
	}
	if err := g.confirm(ctx, a, approve, opts); err != nil {
		return res, err
	}

	stamp := g.now()
	entry := history.Entry{
		Time: stamp, Op: string(op.Kind), Branch: a.branch, Before: a.head,
		Paths: op.Paths, Target: target(op), Detail: detail(op),
	}
	if a.BackupCommit != "" {
		if entry.Backup, err = g.backup(ctx, backupPrefix, stamp, string(op.Kind), a.BackupCommit); err != nil {
			return res, err
		}
	}
	if a.BackupWorktree {
		if entry.WorktreeBackup, err = g.backupWorktree(ctx, stamp, string(op.Kind)); err != nil {
			return res, err
		}
	}

	runErr := g.run(ctx, a, &res)

	entry.After = entry.Before
	if head, born, err := g.git.Head(ctx, g.dir); err == nil && born {
		entry.After = head
	}
	if runErr != nil {
		entry.Failed = failureKind(runErr)
	}
	written, histErr := g.history.Append(entry)
	res.Entry = written
	if runErr != nil {
		return res, runErr
	}
	return res, histErr
}

// confirm obtains the confirmation an assessment calls for.
func (g *Guard) confirm(ctx context.Context, a Assessment, approve Approver, opts DoOptions) error {
	if a.Confirm == ConfirmNone || (a.Confirm == ConfirmSimple && opts.AssumeYes) {
		return nil
	}
	if approve == nil {
		return ErrConfirmationRequired
	}
	answer, err := approve(a)
	if err != nil {
		return err
	}
	if !answer.Approved || (a.Confirm == ConfirmTypeBranch && answer.Typed != a.ConfirmText) {
		return ErrDeclined
	}

	// The user agreed to what they were shown. If the repository is somewhere
	// else by now, that agreement is about another state.
	st, err := g.git.Status(ctx, g.dir, git.StatusOptions{Untracked: git.UntrackedNone})
	if err != nil {
		return err
	}
	if st.Branch.Head != a.branch || st.Branch.OID != a.head {
		return ErrRepositoryChanged
	}
	return nil
}

// run executes the operation with the git package.
func (g *Guard) run(ctx context.Context, a Assessment, res *Result) (err error) {
	op := a.Op
	switch op.Kind {
	case OpCommit, OpAmend:
		res.Commit, err = g.git.Commit(ctx, g.dir, op.CommitOptions)
	case OpDiscard:
		err = g.git.Restore(ctx, g.dir, git.RestoreOptions{Paths: op.Paths, Worktree: true})
	case OpReset:
		err = g.git.Reset(ctx, g.dir, op.ResetMode, op.Rev)
	case OpDeleteBranch:
		// With nothing to lose (and a backup ref in place) the delete is
		// forced: git's own idea of "merged" looks only at the upstream and
		// HEAD and would refuse some branches that are safely on a remote.
		err = g.git.DeleteBranch(ctx, g.dir, op.Branch, op.Force || a.unmerged == 0)
	case OpPush, OpForcePush:
		res.Push, err = g.git.Push(ctx, g.dir, op.PushOptions)
	case OpPull:
		res.Pull, err = g.git.Pull(ctx, g.dir, op.PullOptions)
	case OpSwitch:
		err = g.git.SwitchBranch(ctx, g.dir, op.Branch)
	default:
		err = fmt.Errorf("safety: unknown operation %q", op.Kind)
	}
	return err
}

// target is the other branch or revision an operation involves.
func target(op Operation) string {
	switch op.Kind {
	case OpDeleteBranch, OpSwitch:
		return op.Branch
	case OpReset:
		return op.Rev
	case OpPush, OpForcePush:
		return op.PushOptions.Remote
	case OpPull:
		return op.PullOptions.Remote
	case OpCommit, OpAmend, OpDiscard:
	}
	return ""
}

// detail is the variant of an operation that an undo needs to know.
func detail(op Operation) string {
	switch op.Kind {
	case OpReset:
		return string(op.ResetMode)
	case OpPull:
		return string(op.PullOptions.Strategy)
	case OpCommit, OpAmend, OpDiscard, OpDeleteBranch, OpPush, OpForcePush, OpSwitch:
	}
	return ""
}

// failureKind names a failure for the history: a kind, never the error text,
// which can quote paths and output.
func failureKind(err error) string {
	var argErr *git.ArgError
	if errors.As(err, &argErr) {
		return "arg_" + string(argErr.Code)
	}
	return string(git.KindOf(err))
}
