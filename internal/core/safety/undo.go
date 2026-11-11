package safety

import (
	"context"
	"slices"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/history"
)

// UndoPlan says which operation an undo would take back, and whether it can.
type UndoPlan struct {
	// Entry is the operation; the zero value when there is none.
	Entry history.Entry
	// Err is why it cannot be undone; nil when it can. The reasons are the
	// ErrUndo… values.
	Err error

	// steps performs the undo; safetyBackup is set when it replaces the
	// current HEAD and that commit should be kept first.
	steps        func(context.Context) error
	safetyBackup bool
	// branch and head are the state the plan was made in.
	branch, head string
}

// PlanUndo looks at the last operation gitpal performed here and works out
// how to take it back, without doing it.
//
// An undo only goes ahead when the repository is still where the operation
// left it: if commits were added since, or the branch changed, moving things
// back automatically could destroy that newer work, and the plan says so
// instead.
func (g *Guard) PlanUndo(ctx context.Context) (UndoPlan, error) {
	entry, ok, err := g.history.LastUndoable()
	if err != nil {
		return UndoPlan{}, err
	}
	if !ok {
		return UndoPlan{Err: ErrUndoNothing}, nil
	}
	st, err := g.git.Status(ctx, g.dir, git.StatusOptions{Untracked: git.UntrackedNone})
	if err != nil {
		return UndoPlan{}, err
	}
	plan := UndoPlan{Entry: entry, branch: st.Branch.Head, head: st.Branch.OID}
	plan.Err, err = g.planSteps(ctx, &plan, st)
	return plan, err
}

// planSteps fills in how to undo plan.Entry. The first result is the reason
// it cannot be undone; the second is a failure to find out.
func (g *Guard) planSteps(ctx context.Context, plan *UndoPlan, st git.Status) (refusal, err error) {
	e := plan.Entry
	moved := st.Branch.OID != e.After || st.Branch.Head != e.Branch || st.Operation != git.OpNone
	dirty := changedPaths(st)

	switch OpKind(e.Op) {
	case OpCommit, OpAmend:
		if e.Before == "" {
			return ErrUndoFirstCommit, nil
		}
		if moved {
			return ErrUndoMoved, nil
		}
		pushed, err := g.git.RemoteContains(ctx, g.dir, e.After)
		if err != nil || pushed {
			return ErrUndoPushed, err
		}
		// Soft: the commit's changes stay staged, nothing is lost.
		plan.steps = func(ctx context.Context) error { return g.git.Reset(ctx, g.dir, git.ResetSoft, e.Before) }

	case OpDiscard:
		if !g.refExists(ctx, e.WorktreeBackup) {
			return ErrUndoNoBackup, nil
		}
		// Putting the old content back over a file edited since would
		// destroy the newer edit.
		for _, changed := range dirty {
			if slices.ContainsFunc(e.Paths, func(p string) bool { return covers(p, changed) }) {
				return ErrUndoDirty, nil
			}
		}
		plan.steps = func(ctx context.Context) error {
			return g.git.Restore(ctx, g.dir, git.RestoreOptions{Paths: e.Paths, Source: e.WorktreeBackup, Worktree: true})
		}

	case OpReset:
		if e.Before == "" {
			return ErrUndoNoBackup, nil
		}
		if moved {
			return ErrUndoMoved, nil
		}
		mode := git.ResetMode(e.Detail)
		if mode != git.ResetHard {
			// The working tree was never touched; move the branch back the
			// same gentle way.
			plan.steps = func(ctx context.Context) error { return g.git.Reset(ctx, g.dir, mode, e.Before) }
			return nil, nil
		}
		if len(dirty) > 0 {
			return ErrUndoDirty, nil
		}
		plan.safetyBackup = true
		plan.steps = func(ctx context.Context) error {
			if err := g.git.Reset(ctx, g.dir, git.ResetHard, e.Before); err != nil {
				return err
			}
			if e.WorktreeBackup == "" || !g.refExists(ctx, e.WorktreeBackup) {
				return nil
			}
			return g.git.ApplySnapshot(ctx, g.dir, e.WorktreeBackup)
		}

	case OpDeleteBranch:
		if !g.refExists(ctx, e.Backup) {
			return ErrUndoNoBackup, nil
		}
		if g.refExists(ctx, "refs/heads/"+e.Target) {
			return ErrUndoBranchExists.with(e.Target), nil
		}
		plan.steps = func(ctx context.Context) error { return g.git.CreateBranch(ctx, g.dir, e.Target, e.Backup) }

	case OpPull:
		if e.Before == "" {
			return ErrUndoNotSupported.with(e.Op), nil
		}
		if moved {
			return ErrUndoMoved, nil
		}
		if len(dirty) > 0 {
			return ErrUndoDirty, nil
		}
		plan.safetyBackup = true
		plan.steps = func(ctx context.Context) error { return g.git.Reset(ctx, g.dir, git.ResetHard, e.Before) }

	case OpSwitch:
		if e.Branch == "" {
			return ErrUndoNotSupported.with(e.Op), nil
		}
		plan.steps = func(ctx context.Context) error { return g.git.SwitchBranch(ctx, g.dir, e.Branch) }

	case OpPush, OpForcePush:
		return ErrUndoPushed, nil

	default:
		return ErrUndoNotSupported.with(e.Op), nil
	}
	return nil, nil
}

// refExists reports whether a ref or revision resolves to a commit.
func (g *Guard) refExists(ctx context.Context, ref string) bool {
	if ref == "" {
		return false
	}
	_, err := g.git.ResolveCommit(ctx, g.dir, ref)
	return err == nil
}

// Undo takes back the last operation gitpal performed here, after one yes.
// When that is not possible it changes nothing and returns the reason (see
// PlanUndo). The undo is itself written to the history.
func (g *Guard) Undo(ctx context.Context, approve Approver, opts DoOptions) (history.Entry, error) {
	plan, err := g.PlanUndo(ctx)
	if err != nil {
		return history.Entry{}, err
	}
	if plan.Err != nil {
		return history.Entry{}, plan.Err
	}

	// The confirmation is checked against the state the plan was made in.
	a := Assessment{Risk: RiskCaution, Confirm: ConfirmSimple, branch: plan.branch, head: plan.head}
	if err := g.confirm(ctx, a, approve, opts); err != nil {
		return history.Entry{}, err
	}

	stamp := g.now()
	entry := history.Entry{Time: stamp, Op: history.OpUndo, Undoes: plan.Entry.ID, Branch: a.branch, Before: a.head}
	if plan.safetyBackup && a.head != "" {
		if entry.Backup, err = g.backup(ctx, backupPrefix, stamp, history.OpUndo, a.head); err != nil {
			return history.Entry{}, err
		}
	}

	runErr := plan.steps(ctx)

	entry.After = entry.Before
	if head, born, err := g.git.Head(ctx, g.dir); err == nil && born {
		entry.After = head
	}
	if runErr != nil {
		entry.Failed = failureKind(runErr)
	}
	written, histErr := g.history.Append(entry)
	if runErr != nil {
		return written, runErr
	}
	return written, histErr
}
