package safety

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/history"
)

// Backups are refs in gitpal's own namespace (spec §7.3). A ref keeps its
// commit from being garbage collected and shows up in no branch or tag list.
//
//	refs/gitpal/backup/<time>-<operation>           a commit at risk
//	refs/gitpal/backup-worktree/<time>-<operation>  uncommitted changes at risk
const (
	backupPrefix         = git.OwnRefPrefix + "backup/"
	backupWorktreePrefix = git.OwnRefPrefix + "backup-worktree/"

	// backupTimeLayout sorts as text and is valid in a ref name.
	backupTimeLayout = "20060102T150405.000Z"

	// OpRestoreBackup is the operation recorded when a backup of
	// uncommitted changes or of a deleted branch is put back.
	OpRestoreBackup = "restore-backup"
)

// Backup is one thing gitpal kept before a risky operation.
type Backup struct {
	// Ref is the backup's name; Hash the commit it holds.
	Ref  string
	Hash string
	// Time is when it was taken; Op the operation it was taken for.
	Time time.Time
	Op   string
	// Worktree is set when it holds uncommitted changes rather than a commit
	// of the history.
	Worktree bool
}

// backup creates a backup ref for rev and returns its name.
func (g *Guard) backup(ctx context.Context, prefix string, stamp time.Time, op, rev string) (string, error) {
	// Two backups in the same millisecond get different names.
	for attempt := range 100 {
		name := prefix + backupName(stamp.Add(time.Duration(attempt)*time.Millisecond), op)
		existing, err := g.git.Refs(ctx, g.dir, name)
		if err != nil {
			return "", err
		}
		if len(existing) > 0 {
			continue
		}
		return name, g.git.CreateRef(ctx, g.dir, name, rev)
	}
	return "", fmt.Errorf("safety: could not find a free backup name")
}

func backupName(stamp time.Time, op string) string {
	// The layout's "." is fine in a ref name but reads badly; drop it.
	return strings.Replace(stamp.UTC().Format(backupTimeLayout), ".", "", 1) + "-" + op
}

// backupWorktree backs up the uncommitted changes, if there are any, and
// returns the backup's name.
func (g *Guard) backupWorktree(ctx context.Context, stamp time.Time, op string) (string, error) {
	snapshot, ok, err := g.git.WorktreeSnapshot(ctx, g.dir)
	if err != nil || !ok {
		return "", err
	}
	return g.backup(ctx, backupWorktreePrefix, stamp, op, snapshot)
}

// Backups lists the backups, newest first.
func (g *Guard) Backups(ctx context.Context) ([]Backup, error) {
	var backups []Backup
	for _, prefix := range []string{backupPrefix, backupWorktreePrefix} {
		refs, err := g.git.Refs(ctx, g.dir, prefix)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			stamp, op, ok := strings.Cut(strings.TrimPrefix(ref.Name, prefix), "-")
			if !ok {
				continue
			}
			when, err := time.Parse(strings.Replace(backupTimeLayout, ".", "", 1), stamp)
			if err != nil {
				continue // not a name gitpal wrote
			}
			backups = append(backups, Backup{
				Ref: ref.Name, Hash: ref.Hash, Time: when.UTC(), Op: op, Worktree: prefix == backupWorktreePrefix,
			})
		}
	}
	sort.SliceStable(backups, func(i, j int) bool { return backups[i].Time.After(backups[j].Time) })
	return backups, nil
}

// PruneBackups deletes the backups older than the policy's retention and
// returns them.
func (g *Guard) PruneBackups(ctx context.Context) ([]Backup, error) {
	backups, err := g.Backups(ctx)
	if err != nil {
		return nil, err
	}
	now := g.now()
	var removed []Backup
	for _, b := range backups {
		if now.Sub(b.Time) <= g.policy.BackupRetention {
			continue
		}
		if err := g.git.DeleteRef(ctx, g.dir, b.Ref); err != nil {
			return removed, err
		}
		removed = append(removed, b)
	}
	return removed, nil
}

// RestoreBackup puts a backup back.
//
// Uncommitted changes are applied on top of the working tree; that fails,
// changing nothing, when they no longer fit. The backup of a deleted branch
// recreates the branch. Any other backup of a commit moves the current
// branch back to it, which is a hard reset and goes through Do like one.
func (g *Guard) RestoreBackup(ctx context.Context, ref string, approve Approver, opts DoOptions) (Result, error) {
	backups, err := g.Backups(ctx)
	if err != nil {
		return Result{}, err
	}
	var found *Backup
	for i := range backups {
		if backups[i].Ref == ref {
			found = &backups[i]
		}
	}
	if found == nil {
		return Result{}, ErrBackupNotFound.with(ref)
	}

	switch {
	case found.Worktree:
		return g.record(ctx, OpRestoreBackup, ref, g.git.ApplySnapshot(ctx, g.dir, ref))
	case found.Op == string(OpDeleteBranch):
		name, err := g.deletedBranchName(ref)
		if err != nil {
			return Result{}, err
		}
		if name == "" {
			return Result{}, ErrBackupUnknownName.with(found.Hash)
		}
		return g.record(ctx, OpRestoreBackup, ref, g.git.CreateBranch(ctx, g.dir, name, ref))
	}
	return g.Do(ctx, Reset(git.ResetHard, ref), approve, opts)
}

// deletedBranchName finds the name of the branch a backup was taken of.
func (g *Guard) deletedBranchName(ref string) (string, error) {
	entries, err := g.history.Entries()
	if err != nil {
		return "", err
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Backup == ref && entries[i].Op == string(OpDeleteBranch) {
			return entries[i].Target, nil
		}
	}
	return "", nil
}

// record writes a step that did not go through Do to the history.
func (g *Guard) record(ctx context.Context, op, target string, runErr error) (Result, error) {
	entry := history.Entry{Time: g.now(), Op: op, Target: target}
	if st, err := g.git.Status(ctx, g.dir, git.StatusOptions{Untracked: git.UntrackedNone}); err == nil {
		entry.Branch, entry.Before, entry.After = st.Branch.Head, st.Branch.OID, st.Branch.OID
	}
	if runErr != nil {
		entry.Failed = failureKind(runErr)
	}
	written, histErr := g.history.Append(entry)
	if runErr != nil {
		return Result{Entry: written}, runErr
	}
	return Result{Entry: written}, histErr
}
