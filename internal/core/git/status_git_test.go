package git_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
)

// statusOf reads the status with default options and fails the test on error.
func statusOf(t *testing.T, repo *gittest.Repo) git.Status {
	t.Helper()
	st, err := repo.Runner.Status(context.Background(), repo.Dir, git.StatusOptions{})
	require.NoError(t, err)
	return st
}

// statusTryGit runs a git command that is allowed to fail (a merge that stops
// on a conflict, for instance).
func statusTryGit(repo *gittest.Repo, args ...string) error {
	_, err := repo.Runner.Run(context.Background(), git.Cmd{Dir: repo.Dir, Args: args})
	return err
}

// statusEntry finds the entry for a path.
func statusEntry(t *testing.T, st git.Status, path string) git.StatusEntry {
	t.Helper()
	for _, e := range st.Entries {
		if e.Path == path {
			return e
		}
	}
	require.Failf(t, "no status entry", "path %q not in %+v", path, st.Entries)
	return git.StatusEntry{}
}

// statusConflictRepo returns a repository with branches "main" and "other"
// that both changed f.txt, checked out on main.
func statusConflictRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.NewRepo(t)
	repo.WriteFile("f.txt", "base\n")
	repo.Commit("base")
	repo.Git("switch", "--quiet", "-c", "other")
	repo.WriteFile("f.txt", "theirs\n")
	repo.Commit("theirs")
	repo.Git("switch", "--quiet", "main")
	repo.WriteFile("f.txt", "ours\n")
	repo.Commit("ours")
	return repo
}

func TestStatusUnbornRepository(t *testing.T) {
	repo := gittest.NewRepo(t)

	st := statusOf(t, repo)

	assert.Equal(t, git.BranchStatus{Head: "main", Unborn: true}, st.Branch)
	assert.True(t, st.Clean())
	assert.Equal(t, git.OpNone, st.Operation)
}

func TestStatusCleanRepository(t *testing.T) {
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "a\n")
	hash := repo.Commit("first")

	st := statusOf(t, repo)

	assert.Equal(t, git.BranchStatus{Head: "main", OID: hash}, st.Branch)
	assert.True(t, st.Clean())
	assert.Empty(t, st.Entries)
}

func TestStatusChanges(t *testing.T) {
	repo := gittest.NewRepo(t)
	for _, name := range []string{"modified.txt", "staged.txt", "both.txt", "deleted.txt", "removed.txt", "old name.txt"} {
		repo.WriteFile(name, "one\ntwo\nthree\nfour\n")
	}
	repo.Commit("first")

	repo.WriteFile("modified.txt", "changed\n")
	repo.WriteFile("staged.txt", "changed\n")
	repo.Git("add", "staged.txt")
	repo.WriteFile("both.txt", "staged\n")
	repo.Git("add", "both.txt")
	repo.WriteFile("both.txt", "and again\n")
	repo.Remove("deleted.txt")
	repo.Git("rm", "--quiet", "removed.txt")
	repo.Git("mv", "old name.txt", "new name.txt")
	repo.WriteFile("added.txt", "new\n")
	repo.Git("add", "added.txt")
	repo.WriteFile("deep/er/untracked.txt", "new\n")
	repo.WriteFile("deep/other.txt", "new\n")

	st := statusOf(t, repo)

	assert.False(t, st.Clean())
	assert.Equal(t, git.StatusEntry{
		Kind: git.EntryChanged, Path: "modified.txt", Index: git.StateUnmodified, Worktree: git.StateModified,
	}, statusEntry(t, st, "modified.txt"))
	assert.Equal(t, git.StatusEntry{
		Kind: git.EntryChanged, Path: "staged.txt", Index: git.StateModified, Worktree: git.StateUnmodified,
	}, statusEntry(t, st, "staged.txt"))
	assert.Equal(t, git.StatusEntry{
		Kind: git.EntryChanged, Path: "both.txt", Index: git.StateModified, Worktree: git.StateModified,
	}, statusEntry(t, st, "both.txt"))
	assert.Equal(t, git.StatusEntry{
		Kind: git.EntryChanged, Path: "deleted.txt", Index: git.StateUnmodified, Worktree: git.StateDeleted,
	}, statusEntry(t, st, "deleted.txt"))
	assert.Equal(t, git.StatusEntry{
		Kind: git.EntryChanged, Path: "removed.txt", Index: git.StateDeleted, Worktree: git.StateUnmodified,
	}, statusEntry(t, st, "removed.txt"))
	assert.Equal(t, git.StatusEntry{
		Kind: git.EntryChanged, Path: "added.txt", Index: git.StateAdded, Worktree: git.StateUnmodified,
	}, statusEntry(t, st, "added.txt"))
	assert.Equal(t, git.StatusEntry{
		Kind: git.EntryRenamed, Path: "new name.txt", OrigPath: "old name.txt",
		Index: git.StateRenamed, Worktree: git.StateUnmodified, Score: 100,
	}, statusEntry(t, st, "new name.txt"))

	assert.ElementsMatch(t, []string{"deep/er/untracked.txt", "deep/other.txt"}, entryPaths(st.Untracked()),
		"every untracked file is listed, not just its directory")
	assert.ElementsMatch(t,
		[]string{"staged.txt", "both.txt", "removed.txt", "added.txt", "new name.txt"}, entryPaths(st.Staged()))
	assert.ElementsMatch(t, []string{"modified.txt", "both.txt", "deleted.txt"}, entryPaths(st.Unstaged()))
	assert.Empty(t, st.Conflicted())
}

func TestStatusUntrackedModes(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("first")
	repo.WriteFile("dir/one.txt", "1\n")
	repo.WriteFile("dir/two.txt", "2\n")

	t.Run("normal collapses a directory", func(t *testing.T) {
		st, err := repo.Runner.Status(ctx, repo.Dir, git.StatusOptions{Untracked: git.UntrackedNormal})
		require.NoError(t, err)
		assert.Equal(t, []string{"dir/"}, entryPaths(st.Untracked()))
	})

	t.Run("none hides them", func(t *testing.T) {
		st, err := repo.Runner.Status(ctx, repo.Dir, git.StatusOptions{Untracked: git.UntrackedNone})
		require.NoError(t, err)
		assert.Empty(t, st.Untracked())
		assert.True(t, st.Clean())
	})
}

func TestStatusIgnoredFilesOnlyWhenAsked(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	repo.WriteFile(".gitignore", "*.log\n")
	repo.Commit("first")
	repo.WriteFile("debug.log", "noise\n")

	st := statusOf(t, repo)
	assert.Empty(t, st.Entries)

	st, err := repo.Runner.Status(ctx, repo.Dir, git.StatusOptions{Ignored: true})
	require.NoError(t, err)
	assert.Equal(t, []git.StatusEntry{{Kind: git.EntryIgnored, Path: "debug.log"}}, st.Entries)
	assert.True(t, st.Clean(), "ignored files do not make the repository dirty")
}

func TestStatusAwkwardFileNames(t *testing.T) {
	names := []string{"çalışma dosyası.txt", "-dash.txt", "*.txt", "a b/c d.txt", "quote'single.txt"}
	if runtime.GOOS != "windows" {
		names = append(names, "tab\there.txt", "new\nline.txt", `double"quote.txt`, `back\slash.txt`)
	}
	repo := gittest.NewRepo(t)
	for _, name := range names {
		repo.WriteFile(name, "x\n")
	}

	st := statusOf(t, repo)

	assert.ElementsMatch(t, names, entryPaths(st.Untracked()))
}

func TestStatusDetachedHead(t *testing.T) {
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "a\n")
	hash := repo.Commit("first")
	repo.Git("switch", "--quiet", "--detach")

	st := statusOf(t, repo)

	assert.Equal(t, git.BranchStatus{Detached: true, OID: hash}, st.Branch)
}

func TestStatusAheadBehindAndGoneUpstream(t *testing.T) {
	origin := gittest.NewBareRepo(t)
	first := gittest.Clone(t, origin)
	first.WriteFile("a.txt", "a\n")
	first.Commit("first")
	first.Git("push", "--quiet", "--set-upstream", "origin", "main")
	second := gittest.Clone(t, origin)

	t.Run("in sync", func(t *testing.T) {
		st := statusOf(t, second)
		assert.Equal(t, "origin/main", st.Branch.Upstream)
		assert.False(t, st.Branch.UpstreamGone)
		assert.Zero(t, st.Branch.Ahead)
		assert.Zero(t, st.Branch.Behind)
	})

	t.Run("ahead and behind", func(t *testing.T) {
		first.WriteFile("b.txt", "b\n")
		first.Commit("second")
		first.Git("push", "--quiet")
		second.WriteFile("c.txt", "c\n")
		second.Commit("local one")
		second.WriteFile("d.txt", "d\n")
		second.Commit("local two")
		second.Git("fetch", "--quiet")

		st := statusOf(t, second)
		assert.Equal(t, 2, st.Branch.Ahead)
		assert.Equal(t, 1, st.Branch.Behind)
	})

	t.Run("upstream deleted on the remote", func(t *testing.T) {
		first.Git("switch", "--quiet", "-c", "feat")
		first.Git("push", "--quiet", "--set-upstream", "origin", "feat")
		second.Git("fetch", "--quiet")
		second.Git("switch", "--quiet", "feat")
		first.Git("push", "--quiet", "origin", "--delete", "feat")
		second.Git("fetch", "--quiet", "--prune")

		st := statusOf(t, second)
		assert.Equal(t, "feat", st.Branch.Head)
		assert.Equal(t, "origin/feat", st.Branch.Upstream)
		assert.True(t, st.Branch.UpstreamGone)
	})
}

func TestStatusOperationsInProgress(t *testing.T) {
	t.Run("merge stopped on a conflict", func(t *testing.T) {
		repo := statusConflictRepo(t)
		require.Error(t, statusTryGit(repo, "merge", "other"))

		st := statusOf(t, repo)

		assert.Equal(t, git.OpMerge, st.Operation)
		assert.Equal(t, []git.StatusEntry{{
			Kind: git.EntryUnmerged, Path: "f.txt", Index: git.StateUnmerged, Worktree: git.StateUnmerged,
		}}, st.Conflicted())
		assert.True(t, st.Entries[0].Conflicted())
		assert.False(t, st.Clean())
	})

	t.Run("rebase stopped on a conflict", func(t *testing.T) {
		repo := statusConflictRepo(t)
		require.Error(t, statusTryGit(repo, "rebase", "other"))

		st := statusOf(t, repo)

		assert.Equal(t, git.OpRebase, st.Operation)
		assert.Len(t, st.Conflicted(), 1)
		assert.True(t, st.Branch.Detached, "a rebase runs on a detached HEAD")
	})

	t.Run("rebase with the apply backend", func(t *testing.T) {
		repo := statusConflictRepo(t)
		require.Error(t, statusTryGit(repo, "rebase", "--apply", "other"))

		op, err := repo.Runner.InProgress(context.Background(), repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, git.OpRebase, op)
	})

	t.Run("am stopped on a patch that does not apply", func(t *testing.T) {
		repo := statusConflictRepo(t)
		patch := repo.Git("format-patch", "--stdout", "-1", "other")
		_, err := repo.Runner.Run(context.Background(), git.Cmd{Dir: repo.Dir, Args: []string{"am"}, Stdin: []byte(patch + "\n")})
		require.Error(t, err)

		op, err := repo.Runner.InProgress(context.Background(), repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, git.OpAm, op)
	})

	t.Run("cherry-pick stopped on a conflict", func(t *testing.T) {
		repo := statusConflictRepo(t)
		require.Error(t, statusTryGit(repo, "cherry-pick", "other"))

		st := statusOf(t, repo)

		assert.Equal(t, git.OpCherryPick, st.Operation)
		assert.Len(t, st.Conflicted(), 1)
	})

	t.Run("revert stopped on a conflict", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("f.txt", "one\n")
		repo.Commit("one")
		repo.WriteFile("f.txt", "two\n")
		two := repo.Commit("two")
		repo.WriteFile("f.txt", "three\n")
		repo.Commit("three")
		require.Error(t, statusTryGit(repo, "revert", "--no-edit", two))

		st := statusOf(t, repo)

		assert.Equal(t, git.OpRevert, st.Operation)
		assert.Len(t, st.Conflicted(), 1)
	})

	t.Run("bisect", func(t *testing.T) {
		repo := statusConflictRepo(t)
		repo.Git("bisect", "start")

		op, err := repo.Runner.InProgress(context.Background(), repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, git.OpBisect, op)
	})

	t.Run("nothing", func(t *testing.T) {
		repo := statusConflictRepo(t)

		op, err := repo.Runner.InProgress(context.Background(), repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, git.OpNone, op)
	})
}

// A linked worktree keeps its own MERGE_HEAD and friends under
// .git/worktrees/<name>; each worktree must report its own state.
func TestStatusLinkedWorktreeHasItsOwnOperation(t *testing.T) {
	repo := statusConflictRepo(t)
	linkedDir := filepath.Join(gittest.TempDir(t), "linked")
	repo.Git("worktree", "add", "--quiet", "-b", "linked", linkedDir, "main")
	linked := &gittest.Repo{Dir: linkedDir, Runner: repo.Runner}
	require.Error(t, statusTryGit(linked, "merge", "other"))

	inLinked, err := repo.Runner.Status(context.Background(), linkedDir, git.StatusOptions{})
	require.NoError(t, err)
	assert.Equal(t, git.OpMerge, inLinked.Operation)
	assert.Equal(t, "linked", inLinked.Branch.Head)
	assert.Len(t, inLinked.Conflicted(), 1)

	inMain := statusOf(t, repo)
	assert.Equal(t, git.OpNone, inMain.Operation)
	assert.True(t, inMain.Clean())
}

// Settings in the user's git configuration that change what `git status`
// prints must not change what gitpal reads.
func TestStatusIgnoresHostileConfiguration(t *testing.T) {
	origin := gittest.NewBareRepo(t)
	repo := gittest.Clone(t, origin)
	repo.WriteFile("old name.txt", "one\ntwo\nthree\nfour\n")
	repo.WriteFile("tracked.txt", "x\n")
	repo.Commit("first")
	repo.Git("push", "--quiet", "--set-upstream", "origin", "main")
	repo.WriteFile("tracked.txt", "y\n")
	repo.Commit("second")
	repo.Git("mv", "old name.txt", "new name.txt")
	repo.WriteFile("sub/untracked.txt", "new\n")
	repo.Git("stash", "push", "--quiet", "--include-untracked")
	repo.Git("stash", "apply", "--quiet", "--index")

	want := statusOf(t, repo)
	require.Equal(t, git.EntryRenamed, statusEntry(t, want, "new name.txt").Kind)
	require.Equal(t, 1, want.Branch.Ahead)

	for key, value := range map[string]string{
		"status.renames":              "false",
		"diff.renames":                "false",
		"status.showUntrackedFiles":   "no",
		"status.relativePaths":        "true",
		"status.branch":               "false",
		"status.short":                "true",
		"status.aheadBehind":          "false",
		"status.showStash":            "true",
		"status.submoduleSummary":     "true",
		"status.displayCommentPrefix": "true",
		"column.ui":                   "always",
		"color.status":                "always",
		"advice.statusHints":          "true",
		"core.quotepath":              "true",
	} {
		repo.Git("config", key, value)
	}

	assert.Equal(t, want, statusOf(t, repo))
}

func TestStatusOutsideARepository(t *testing.T) {
	runner := gittest.NewRunner(t)
	dir := gittest.TempDir(t)

	_, err := runner.Status(context.Background(), dir, git.StatusOptions{})
	assert.Equal(t, git.KindNotARepo, git.KindOf(err))

	_, err = runner.InProgress(context.Background(), dir)
	assert.Equal(t, git.KindNotARepo, git.KindOf(err))
}

func entryPaths(entries []git.StatusEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	return paths
}
