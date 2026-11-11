package safety

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
)

// blockedDuringOperation are the operations that must wait while a merge or
// rebase is under way (rule 9). Committing concludes a merge and discarding a
// file is part of resolving one, so those stay allowed.
var blockedDuringOperation = map[OpKind]bool{
	OpAmend: true, OpReset: true, OpPull: true, OpSwitch: true, OpForcePush: true,
}

// Assess judges an operation without running it: its risk, what it will do,
// what stands in its way and how it must be confirmed. This is the dry run.
func (g *Guard) Assess(ctx context.Context, op Operation) (Assessment, error) {
	st, err := g.git.Status(ctx, g.dir, git.StatusOptions{Untracked: git.UntrackedNone})
	if err != nil {
		return Assessment{}, err
	}
	a := Assessment{Op: op, Risk: RiskSafe, branch: st.Branch.Head, head: st.Branch.OID}

	if st.Operation != git.OpNone && blockedDuringOperation[op.Kind] {
		a.add(Finding{Code: CodeOperationInProgress, Severity: SeverityBlock, Detail: string(st.Operation)})
	}

	switch op.Kind {
	case OpCommit, OpAmend:
		err = g.assessCommit(ctx, &a, st)
	case OpDiscard:
		g.assessDiscard(&a, st)
	case OpReset:
		err = g.assessReset(ctx, &a, st)
	case OpDeleteBranch:
		err = g.assessDeleteBranch(ctx, &a)
	case OpPush, OpForcePush:
		err = g.assessPush(ctx, &a)
	case OpPull:
		g.assessPull(&a, st)
	case OpSwitch:
		g.backupDirty(&a, st)
	default:
		err = fmt.Errorf("safety: unknown operation %q", op.Kind)
	}
	if err != nil {
		return Assessment{}, err
	}

	switch a.Risk {
	case RiskSafe:
		a.Confirm = ConfirmNone
	case RiskCaution:
		a.Confirm = ConfirmSimple
	case RiskDangerous:
		a.Confirm = ConfirmExplicit
	case RiskCritical:
		a.Confirm = ConfirmTypeBranch
	}
	return a, nil
}

func (a *Assessment) add(f Finding) { a.Findings = append(a.Findings, f) }

// raise lifts the risk to at least r.
func (a *Assessment) raise(r Risk) {
	if r > a.Risk {
		a.Risk = r
	}
}

// changedPaths returns the tracked files with uncommitted changes.
func changedPaths(st git.Status) []string {
	var paths []string
	for _, e := range st.Entries {
		if e.Kind != git.EntryUntracked && e.Kind != git.EntryIgnored {
			paths = append(paths, e.Path)
		}
	}
	return paths
}

// backupDirty arranges for uncommitted changes to be backed up before an
// operation that moves the working tree to other content (rule 4).
func (g *Guard) backupDirty(a *Assessment, st git.Status) {
	if changed := changedPaths(st); len(changed) > 0 && !st.Branch.Unborn {
		a.BackupWorktree = true
		a.add(Finding{Code: CodeUncommittedChanges, Severity: SeverityInfo, Count: len(changed)})
	}
}

func (g *Guard) assessCommit(ctx context.Context, a *Assessment, st git.Status) error {
	if st.Branch.Detached {
		a.add(Finding{Code: CodeDetachedHeadCommit, Severity: SeverityWarn})
	}
	if st.Branch.Head != "" && g.policy.Protected(st.Branch.Head) {
		a.add(Finding{Code: CodeProtectedBranchCommit, Severity: SeverityWarn, Detail: st.Branch.Head})
	}
	if err := g.scanStaged(ctx, a, st); err != nil {
		return err
	}
	if len(a.Findings) > 0 {
		a.raise(RiskCaution)
	}

	if a.Op.Kind == OpAmend && a.head != "" {
		a.raise(RiskCaution)
		a.BackupCommit = a.head
		a.Effects = append(a.Effects, Effect{Kind: EffectCommitRewritten, Count: 1, Items: []string{short(a.head)}})
		pushed, err := g.git.RemoteContains(ctx, g.dir, a.head)
		if err != nil {
			return err
		}
		if pushed {
			a.add(Finding{Code: CodeRewritesPushed, Severity: SeverityWarn, Count: 1})
			a.raise(RiskDangerous)
		}
	}
	return nil
}

// scanStaged looks at what is about to be committed (rule 7): credentials in
// the added lines, files that usually hold credentials, large files, and
// dependencies or build output.
func (g *Guard) scanStaged(ctx context.Context, a *Assessment, st git.Status) error {
	junk := map[string]int{}
	for _, e := range st.Staged() {
		if e.Index == git.StateDeleted {
			continue
		}
		verdict := scanPath(e.Path)
		switch {
		case verdict.code == CodeJunkFile:
			junk[verdict.group]++
		case verdict.code == CodeSecretFile && g.policy.WarnOnSecrets:
			a.add(Finding{Code: CodeSecretFile, Severity: SeverityWarn, Path: e.Path})
		}
		// The working tree copy stands in for the staged one: reading the
		// exact staged size would cost a git call per file.
		if info, err := os.Lstat(filepath.Join(g.dir, filepath.FromSlash(e.Path))); err == nil &&
			g.policy.LargeFileBytes > 0 && info.Mode().IsRegular() && info.Size() > g.policy.LargeFileBytes {
			a.add(Finding{Code: CodeLargeFile, Severity: SeverityWarn, Path: e.Path, Detail: humanSize(info.Size())})
		}
	}
	groups := make([]string, 0, len(junk))
	for group := range junk {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	for _, group := range groups {
		a.add(Finding{Code: CodeJunkFile, Severity: SeverityWarn, Path: group, Count: junk[group]})
	}

	if !g.policy.WarnOnSecrets || len(st.Staged()) == 0 {
		return nil
	}
	files, err := g.git.Diff(ctx, g.dir, git.DiffOptions{Staged: true})
	if err != nil {
		return err
	}
	for _, file := range files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind != git.LineAdded {
					continue
				}
				if label := scanLine(line.Text); label != "" {
					a.add(Finding{Code: CodeSecret, Severity: SeverityWarn, Path: file.Path, Line: line.NewLine, Detail: label})
				}
			}
		}
	}
	return nil
}

func (g *Guard) assessDiscard(a *Assessment, st git.Status) {
	var hit []string
	for _, e := range st.Unstaged() {
		if slices.ContainsFunc(a.Op.Paths, func(p string) bool { return covers(p, e.Path) }) {
			hit = append(hit, e.Path)
		}
	}
	if len(hit) == 0 {
		return
	}
	a.raise(RiskDangerous)
	a.BackupWorktree = true
	a.Effects = append(a.Effects, Effect{Kind: EffectFilesDiscarded, Count: len(hit), Items: hit})
}

// covers reports whether a path given by the user (a file or a directory)
// includes file.
func covers(given, file string) bool {
	given = strings.TrimSuffix(given, "/")
	return given == file || given == "." || strings.HasPrefix(file, given+"/")
}

func (g *Guard) assessReset(ctx context.Context, a *Assessment, st git.Status) error {
	target, err := g.git.ResolveCommit(ctx, g.dir, a.Op.Rev)
	if err != nil {
		return err
	}
	dropped := 0
	if a.head != "" && a.head != target {
		if dropped, _, err = g.git.AheadBehind(ctx, g.dir, a.head, target); err != nil {
			return err
		}
	}
	if dropped > 0 {
		a.raise(RiskCaution)
		a.BackupCommit = a.head
		a.Effects = append(a.Effects, Effect{Kind: EffectCommitsDropped, Count: dropped})
		pushed, err := g.git.RemoteContains(ctx, g.dir, a.head)
		if err != nil {
			return err
		}
		if pushed {
			a.add(Finding{Code: CodeRewritesPushed, Severity: SeverityWarn, Count: dropped})
		}
	}
	if a.Op.ResetMode != git.ResetHard {
		return nil
	}
	changed := changedPaths(st)
	if len(changed) > 0 {
		a.Effects = append(a.Effects, Effect{Kind: EffectFilesDiscarded, Count: len(changed), Items: changed})
		g.backupDirty(a, st)
	}
	if dropped > 0 || len(changed) > 0 {
		a.raise(RiskDangerous)
	}
	return nil
}

func (g *Guard) assessDeleteBranch(ctx context.Context, a *Assessment) error {
	name := a.Op.Branch
	tip, err := g.git.ResolveCommit(ctx, g.dir, "refs/heads/"+name)
	if err != nil {
		return err
	}
	a.raise(RiskCaution)
	a.BackupCommit = tip
	a.Effects = append(a.Effects, Effect{Kind: EffectBranchDeleted, Count: 1, Items: []string{name}})

	// Commits are lost when they are on no other branch here and on no
	// remote. Without a commit to compare with, assume the worst.
	lost := 1
	if a.head != "" {
		if lost, _, err = g.git.AheadBehind(ctx, g.dir, tip, a.head); err != nil {
			return err
		}
	}
	if lost > 0 {
		pushed, err := g.git.RemoteContains(ctx, g.dir, tip)
		if err != nil {
			return err
		}
		if pushed {
			lost = 0
		}
	}
	a.unmerged = lost
	if lost > 0 {
		severity := SeverityWarn
		if !a.Op.Force {
			severity = SeverityBlock
		}
		a.add(Finding{Code: CodeUnmergedBranch, Severity: severity, Count: lost})
		a.Effects = append(a.Effects, Effect{Kind: EffectCommitsDropped, Count: lost})
		a.raise(RiskDangerous)
	}
	if g.policy.Protected(name) {
		a.add(Finding{Code: CodeProtectedBranchDelete, Severity: SeverityWarn, Detail: name})
		a.raise(RiskDangerous)
	}
	return nil
}

func (g *Guard) assessPush(ctx context.Context, a *Assessment) error {
	opts := a.Op.PushOptions
	branch := opts.Branch
	if branch == "" {
		branch = a.branch
	}
	if branch == "" {
		// A detached HEAD: git.Push explains that itself.
		return nil
	}
	force := a.Op.Kind == OpForcePush

	// Where the branch will land decides whether it is protected: a local
	// branch of any name can be set to push to main.
	remoteBranch := branch
	merge, _, err := g.git.ConfigGet(ctx, g.dir, "branch."+branch+".merge")
	if err != nil {
		return err
	}
	if name, ok := strings.CutPrefix(merge, "refs/heads/"); ok && name != "" {
		remoteBranch = name
	}
	if force {
		a.raise(RiskCritical)
		a.ConfirmText = branch
		for _, name := range []string{branch, remoteBranch} {
			if g.policy.Protected(name) {
				a.add(Finding{Code: CodeProtectedForcePush, Severity: SeverityBlock, Detail: name})
				break
			}
		}
	}

	// Ahead and behind are as of the last fetch, and only meaningful for a
	// push to the branch's own upstream.
	upstream, ok, err := g.git.Upstream(ctx, g.dir, branch)
	if err != nil || !ok || opts.Remote != "" {
		return err
	}
	ahead, behind, err := g.git.AheadBehind(ctx, g.dir, "refs/heads/"+branch, upstream)
	if err != nil {
		if git.KindOf(err) == git.KindRefNotFound {
			return nil // the upstream is configured but not fetched yet
		}
		return err
	}
	if ahead > 0 {
		a.Effects = append(a.Effects, Effect{Kind: EffectCommitsPushed, Count: ahead})
	}
	if force {
		if tip, err := g.git.ResolveCommit(ctx, g.dir, upstream); err == nil {
			a.BackupCommit = tip
		}
		if behind > 0 {
			a.Effects = append(a.Effects, Effect{Kind: EffectRemoteCommitsReplaced, Count: behind})
			a.add(Finding{Code: CodeForcePush, Severity: SeverityWarn, Count: behind})
		}
		return nil
	}
	if behind > 0 {
		a.add(Finding{Code: CodeBehindRemote, Severity: SeverityBlock, Count: behind})
	}
	return nil
}

func (g *Guard) assessPull(a *Assessment, st git.Status) {
	if s := a.Op.PullOptions.Strategy; s == git.PullMerge || s == git.PullRebase {
		a.raise(RiskCaution)
		a.BackupCommit = a.head
	}
	g.backupDirty(a, st)
}

func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

func humanSize(n int64) string {
	const kb, mb = 1 << 10, 1 << 20
	switch {
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	}
	return fmt.Sprintf("%d B", n)
}
