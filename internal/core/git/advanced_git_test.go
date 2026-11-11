package git_test

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
)

func subjects(repo *gittest.Repo, rev string) []string {
	return strings.Split(repo.Git("log", "--format=%s", rev), "\n")
}

// linearRepo returns main with base, then a branch "feat" with commits one,
// two, three on top.
func linearRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.NewRepo(t)
	repo.WriteFile("base.txt", "base\n")
	repo.Commit("base")
	repo.Git("switch", "--quiet", "-c", "feat")
	for _, n := range []string{"one", "two", "three"} {
		repo.WriteFile(n+".txt", n+"\n")
		repo.Commit(n)
	}
	return repo
}

func TestMerge(t *testing.T) {
	ctx := context.Background()

	t.Run("fast-forward", func(t *testing.T) {
		repo := linearRepo(t)
		repo.Git("switch", "--quiet", "main")
		res, err := repo.Runner.Merge(ctx, repo.Dir, git.MergeOptions{Rev: "feat"})
		require.NoError(t, err)
		assert.True(t, res.FastForward)
		assert.Equal(t, repo.Git("rev-parse", "feat"), res.After)
	})

	t.Run("no-ff despite merge.ff=only", func(t *testing.T) {
		repo := linearRepo(t)
		repo.Git("switch", "--quiet", "main")
		repo.Git("config", "merge.ff", "only")
		res, err := repo.Runner.Merge(ctx, repo.Dir, git.MergeOptions{Rev: "feat", Mode: git.MergeNoFF, Message: "merge feat in"})
		require.NoError(t, err)
		assert.False(t, res.FastForward)
		assert.Equal(t, "merge feat in", repo.Git("log", "-1", "--format=%s"))
	})

	t.Run("squash stages without committing", func(t *testing.T) {
		repo := linearRepo(t)
		repo.Git("switch", "--quiet", "main")
		before := repo.Git("rev-parse", "HEAD")
		res, err := repo.Runner.Merge(ctx, repo.Dir, git.MergeOptions{Rev: "feat", Mode: git.MergeSquash})
		require.NoError(t, err)
		assert.True(t, res.Squashed)
		assert.Equal(t, before, repo.Git("rev-parse", "HEAD"))
		assert.Equal(t, "A  one.txt\nA  three.txt\nA  two.txt", repo.Git("status", "--porcelain=v1"))
	})

	t.Run("already up to date", func(t *testing.T) {
		repo := linearRepo(t)
		res, err := repo.Runner.Merge(ctx, repo.Dir, git.MergeOptions{Rev: "main"})
		require.NoError(t, err)
		assert.True(t, res.UpToDate)
	})

	t.Run("conflict, then abort", func(t *testing.T) {
		repo := statusConflictRepo(t)
		before := repo.Git("rev-parse", "HEAD")
		_, err := repo.Runner.Merge(ctx, repo.Dir, git.MergeOptions{Rev: "other"})
		assert.Equal(t, git.KindConflict, git.KindOf(err))

		op, err := repo.Runner.AbortOperation(ctx, repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, git.OpMerge, op)
		assert.Equal(t, before, repo.Git("rev-parse", "HEAD"))
		assert.Equal(t, "ours\n", repo.ReadFile("f.txt"))
	})

	t.Run("conflict resolved with theirs, then continue", func(t *testing.T) {
		repo := statusConflictRepo(t)
		_, err := repo.Runner.Merge(ctx, repo.Dir, git.MergeOptions{Rev: "other"})
		require.Error(t, err)

		v, err := repo.Runner.Conflict(ctx, repo.Dir, "f.txt")
		require.NoError(t, err)
		assert.Equal(t, "base\n", string(v.Base))
		assert.Equal(t, "ours\n", string(v.Ours))
		assert.Equal(t, "theirs\n", string(v.Theirs))

		_, err = repo.Runner.ContinueOperation(ctx, repo.Dir)
		assert.Equal(t, git.KindConflict, git.KindOf(err), "conflicts first")

		require.NoError(t, repo.Runner.ResolveConflict(ctx, repo.Dir, []string{"f.txt"}, git.SideTheirs))
		op, err := repo.Runner.ContinueOperation(ctx, repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, git.OpMerge, op)
		assert.Equal(t, "theirs\n", repo.ReadFile("f.txt"))
		assert.Len(t, strings.Fields(repo.Git("log", "-1", "--format=%P")), 2)
	})

	t.Run("nothing to abort", func(t *testing.T) {
		repo := linearRepo(t)
		_, err := repo.Runner.AbortOperation(ctx, repo.Dir)
		assert.True(t, git.ErrNoOperation(err))
	})

	t.Run("hostile revision", func(t *testing.T) {
		repo := linearRepo(t)
		_, err := repo.Runner.Merge(ctx, repo.Dir, git.MergeOptions{Rev: "--abort"})
		assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err))
		_, err = repo.Runner.Merge(ctx, repo.Dir, git.MergeOptions{Rev: "main", Mode: "octopus"})
		assert.Error(t, err)
	})
}

func TestResolveConflictWithADeletion(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	repo.WriteFile("f.txt", "base\n")
	repo.Commit("base")
	repo.Git("switch", "--quiet", "-c", "other")
	repo.Git("rm", "--quiet", "f.txt")
	repo.Commit("delete")
	repo.Git("switch", "--quiet", "main")
	repo.WriteFile("f.txt", "changed\n")
	repo.Commit("change")
	_, err := repo.Runner.Merge(ctx, repo.Dir, git.MergeOptions{Rev: "other"})
	require.Equal(t, git.KindConflict, git.KindOf(err))

	require.NoError(t, repo.Runner.ResolveConflict(ctx, repo.Dir, []string{"f.txt"}, git.SideTheirs))
	_, err = os.Stat(repo.Path("f.txt"))
	assert.True(t, os.IsNotExist(err), "their side deleted it")
	_, err = repo.Runner.ContinueOperation(ctx, repo.Dir)
	require.NoError(t, err)
}

func TestCherryPickAndRevert(t *testing.T) {
	ctx := context.Background()
	repo := linearRepo(t)
	repo.Git("switch", "--quiet", "main")

	require.NoError(t, repo.Runner.CherryPick(ctx, repo.Dir, []string{"feat~1"}))
	assert.Equal(t, "two", repo.Git("log", "-1", "--format=%s"))

	// Several revisions make one walk, as in git: these are "one", then "three".
	require.NoError(t, repo.Runner.CherryPick(ctx, repo.Dir, []string{"feat~3..feat~2"}))
	require.NoError(t, repo.Runner.CherryPick(ctx, repo.Dir, []string{"feat"}))
	assert.Equal(t, []string{"three", "one", "two", "base"}, subjects(repo, "HEAD"))

	require.NoError(t, repo.Runner.Revert(ctx, repo.Dir, []string{"HEAD"}, 0))
	assert.Equal(t, `Revert "three"`, repo.Git("log", "-1", "--format=%s"))
	_, err := os.Stat(repo.Path("three.txt"))
	assert.True(t, os.IsNotExist(err))

	t.Run("a conflict stops it", func(t *testing.T) {
		repo := statusConflictRepo(t)
		err := repo.Runner.CherryPick(ctx, repo.Dir, []string{"other"})
		assert.Equal(t, git.KindConflict, git.KindOf(err))
		op, err := repo.Runner.SkipOperation(ctx, repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, git.OpCherryPick, op)
	})

	t.Run("bad arguments", func(t *testing.T) {
		assert.Error(t, repo.Runner.CherryPick(ctx, repo.Dir, nil))
		assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, repo.Runner.CherryPick(ctx, repo.Dir, []string{"--continue"})))
	})
}

func TestRebase(t *testing.T) {
	ctx := context.Background()

	t.Run("onto an updated base", func(t *testing.T) {
		repo := linearRepo(t)
		repo.Git("switch", "--quiet", "main")
		repo.WriteFile("main.txt", "main\n")
		repo.Commit("main moved")
		repo.Git("switch", "--quiet", "feat")

		res, err := repo.Runner.Rebase(ctx, repo.Dir, git.RebaseOptions{Upstream: "main"})
		require.NoError(t, err)
		assert.NotEqual(t, res.Before, res.After)
		assert.Equal(t, []string{"three", "two", "one", "main moved", "base"}, subjects(repo, "HEAD"))
	})

	t.Run("interactive: reorder, reword, squash, fixup, drop", func(t *testing.T) {
		repo := linearRepo(t)
		repo.WriteFile("four.txt", "4\n")
		repo.Commit("four")
		repo.WriteFile("five.txt", "5\n")
		repo.Commit("five")
		// Hostile settings must not change the plan.
		repo.Git("config", "rebase.abbreviateCommands", "true")
		repo.Git("config", "rebase.autoSquash", "true")
		repo.Git("config", "sequence.editor", "false")
		repo.Git("config", "core.editor", "false")

		plan, err := repo.Runner.RebasePlan(ctx, repo.Dir, "main")
		require.NoError(t, err)
		require.Equal(t, []string{"one", "two", "three", "four", "five"}, func() []string {
			var s []string
			for _, p := range plan {
				s = append(s, p.Subject)
			}
			return s
		}())

		steps := []git.RebaseStep{
			{Action: git.RebasePick, Hash: plan[2].Hash},                                     // three
			{Action: git.RebaseReword, Hash: plan[0].Hash, Message: "one, reworded\n\nbody"}, // one
			{Action: git.RebaseSquash, Hash: plan[1].Hash, Message: "one and two together"},  // two
			{Action: git.RebasePick, Hash: plan[3].Hash},                                     // four
			{Action: git.RebaseFixup, Hash: plan[4].Hash},                                    // five
		}
		_, err = repo.Runner.Rebase(ctx, repo.Dir, git.RebaseOptions{Upstream: "main", Steps: steps})
		require.NoError(t, err)

		assert.Equal(t, []string{"four", "one and two together", "three", "base"}, subjects(repo, "HEAD"))
		assert.ElementsMatch(t, []string{"four.txt", "five.txt"},
			strings.Fields(repo.Git("show", "--format=", "--name-only", "HEAD")), "five folded into four")
	})

	t.Run("interactive: reword alone, drop", func(t *testing.T) {
		repo := linearRepo(t)
		plan, err := repo.Runner.RebasePlan(ctx, repo.Dir, "main")
		require.NoError(t, err)
		plan[1].Action, plan[1].Message = git.RebaseReword, "TWO"
		plan[2].Action = git.RebaseDrop
		_, err = repo.Runner.Rebase(ctx, repo.Dir, git.RebaseOptions{Upstream: "main", Steps: plan})
		require.NoError(t, err)
		assert.Equal(t, []string{"TWO", "one", "base"}, subjects(repo, "HEAD"))
	})

	t.Run("a conflict stops it, abort goes back", func(t *testing.T) {
		repo := statusConflictRepo(t)
		before := repo.Git("rev-parse", "HEAD")
		_, err := repo.Runner.Rebase(ctx, repo.Dir, git.RebaseOptions{Upstream: "other"})
		assert.Equal(t, git.KindConflict, git.KindOf(err))
		op, err := repo.Runner.AbortOperation(ctx, repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, git.OpRebase, op)
		assert.Equal(t, before, repo.Git("rev-parse", "HEAD"))
	})

	t.Run("bad plans", func(t *testing.T) {
		repo := linearRepo(t)
		plan, err := repo.Runner.RebasePlan(ctx, repo.Dir, "main")
		require.NoError(t, err)
		for _, steps := range [][]git.RebaseStep{
			{{Action: git.RebaseSquash, Hash: plan[0].Hash}},
			{{Action: "exec", Hash: "rm -rf /"}},
			{{Action: git.RebasePick, Hash: "--root"}},
		} {
			_, err := repo.Runner.Rebase(ctx, repo.Dir, git.RebaseOptions{Upstream: "main", Steps: steps})
			var argErr *git.ArgError
			assert.ErrorAs(t, err, &argErr)
		}
		_, err = repo.Runner.RebasePlan(ctx, repo.Dir, "--root")
		assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err))
	})
}

func TestStash(t *testing.T) {
	ctx := context.Background()
	repo := stageRepo(t, "a.txt")

	none, err := repo.Runner.Stashes(ctx, repo.Dir)
	require.NoError(t, err)
	assert.Empty(t, none)
	created, err := repo.Runner.StashPush(ctx, repo.Dir, git.StashOptions{Message: "nothing"})
	require.NoError(t, err)
	assert.False(t, created)

	repo.WriteFile("a.txt", "work in progress\n")
	repo.WriteFile("new.txt", "untracked\n")
	created, err = repo.Runner.StashPush(ctx, repo.Dir, git.StashOptions{Message: "wip: login", IncludeUntracked: true})
	require.NoError(t, err)
	assert.True(t, created)
	assert.Empty(t, stageShort(repo))

	stashes, err := repo.Runner.Stashes(ctx, repo.Dir)
	require.NoError(t, err)
	require.Len(t, stashes, 1)
	assert.Equal(t, "stash@{0}", stashes[0].Ref)
	assert.Contains(t, stashes[0].Message, "wip: login")
	assert.False(t, stashes[0].Time.IsZero())

	diff, err := repo.Runner.Diff(ctx, repo.Dir, git.DiffOptions{Commit: stashes[0].Ref})
	require.NoError(t, err)
	assert.Equal(t, "a.txt", diff[0].Path)

	require.NoError(t, repo.Runner.StashApply(ctx, repo.Dir, 0, false))
	assert.Equal(t, "work in progress\n", repo.ReadFile("a.txt"))
	assert.Equal(t, "untracked\n", repo.ReadFile("new.txt"))
	repo.Git("reset", "--quiet", "--hard")
	repo.Remove("new.txt")

	hash := stashes[0].Hash
	require.NoError(t, repo.Runner.StashDrop(ctx, repo.Dir, 0))
	stashes, err = repo.Runner.Stashes(ctx, repo.Dir)
	require.NoError(t, err)
	assert.Empty(t, stashes)

	require.NoError(t, repo.Runner.StashStore(ctx, repo.Dir, hash, "restored"))
	require.NoError(t, repo.Runner.StashApply(ctx, repo.Dir, 0, true))
	assert.Equal(t, "work in progress\n", repo.ReadFile("a.txt"))
	stashes, err = repo.Runner.Stashes(ctx, repo.Dir)
	require.NoError(t, err)
	assert.Empty(t, stashes, "pop dropped it")

	t.Run("only some paths", func(t *testing.T) {
		repo := stageRepo(t, "a.txt", "b.txt")
		repo.WriteFile("a.txt", "x\n")
		repo.WriteFile("b.txt", "y\n")
		created, err := repo.Runner.StashPush(ctx, repo.Dir, git.StashOptions{Paths: []string{"a.txt"}})
		require.NoError(t, err)
		assert.True(t, created)
		assert.Equal(t, " M b.txt", stageShort(repo))
	})

	t.Run("bad index", func(t *testing.T) {
		assert.Error(t, repo.Runner.StashDrop(ctx, repo.Dir, -1))
		assert.Error(t, repo.Runner.StashApply(ctx, repo.Dir, 5, false))
		assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, repo.Runner.StashStore(ctx, repo.Dir, "--x", "")))
	})
}

func TestTags(t *testing.T) {
	ctx := context.Background()
	origin := gittest.NewBareRepo(t)
	repo := gittest.Clone(t, origin)
	repo.WriteFile("a.txt", "a\n")
	first := repo.Commit("first")
	repo.Git("push", "--quiet", "origin", "main")
	repo.Git("config", "tag.gpgSign", "true")
	repo.Git("config", "gpg.program", "false")

	// tag.gpgSign cannot turn a lightweight tag into a signed one…
	require.NoError(t, repo.Runner.CreateTag(ctx, repo.Dir, "v1.0", "", ""))
	// …but an annotated tag follows the user's signing setting, like a commit.
	assert.Error(t, repo.Runner.CreateTag(ctx, repo.Dir, "v1.1", first, "signed"))
	repo.Git("config", "tag.gpgSign", "false")
	require.NoError(t, repo.Runner.CreateTag(ctx, repo.Dir, "v1.1", first, "Sürüm 1.1\n\nnotlar"))

	tags, err := repo.Runner.Tags(ctx, repo.Dir)
	require.NoError(t, err)
	require.Len(t, tags, 2)
	byName := map[string]git.Tag{tags[0].Name: tags[0], tags[1].Name: tags[1]}
	assert.False(t, byName["v1.0"].Annotated)
	assert.True(t, byName["v1.1"].Annotated)
	assert.Equal(t, first, byName["v1.1"].Commit)
	assert.Equal(t, "Sürüm 1.1", byName["v1.1"].Subject)

	assert.Equal(t, git.KindBranchExists, git.KindOf(repo.Runner.CreateTag(ctx, repo.Dir, "v1.0", "", "")))
	for _, bad := range []string{"--force", "a..b", "x.lock", ""} {
		var argErr *git.ArgError
		assert.ErrorAs(t, repo.Runner.CreateTag(ctx, repo.Dir, bad, "", ""), &argErr, bad)
	}

	require.NoError(t, repo.Runner.PushTag(ctx, repo.Dir, "origin", "v1.1"))
	assert.Equal(t, first, origin.Git("rev-parse", "v1.1^{commit}"))
	assert.Equal(t, git.KindNoRemote, git.KindOf(repo.Runner.PushTag(ctx, repo.Dir, "nope", "v1.1")))

	require.NoError(t, repo.Runner.DeleteTag(ctx, repo.Dir, "v1.0"))
	assert.Equal(t, git.KindRefNotFound, git.KindOf(repo.Runner.DeleteTag(ctx, repo.Dir, "v1.0")))
}

func TestReflogRecoversALostCommit(t *testing.T) {
	ctx := context.Background()
	repo := linearRepo(t)
	lost := repo.Git("rev-parse", "HEAD")
	repo.Git("reset", "--quiet", "--hard", "HEAD~2")

	entries, err := repo.Runner.Reflog(ctx, repo.Dir, "", 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(entries), 2)
	assert.Equal(t, "HEAD@{0}", entries[0].Selector)
	assert.Contains(t, entries[0].Action, "reset")
	assert.Equal(t, lost, entries[1].Hash)
	assert.False(t, entries[1].Time.IsZero())

	require.NoError(t, repo.Runner.CreateBranch(ctx, repo.Dir, "recovered", entries[1].Hash))
	assert.Equal(t, lost, repo.Git("rev-parse", "recovered"))

	_, err = repo.Runner.Reflog(ctx, repo.Dir, "--all", 1)
	assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err))
}

func TestBlame(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	repo.WriteFile("f.txt", "one\ntwo\n")
	first := repo.Commit("first")
	repo.WriteFile("f.txt", "one\nTWO\nthree\n")
	second := repo.Commit("second")

	lines, err := repo.Runner.Blame(ctx, repo.Dir, "f.txt", "")
	require.NoError(t, err)
	require.Len(t, lines, 3)
	assert.Equal(t, git.BlameLine{Hash: first, Author: gittest.UserName, Time: lines[0].Time, Line: 1, Summary: "first", Text: "one"}, lines[0])
	assert.Equal(t, second, lines[1].Hash)
	assert.Equal(t, "TWO", lines[1].Text)
	assert.Equal(t, 3, lines[2].Line)

	old, err := repo.Runner.Blame(ctx, repo.Dir, "f.txt", first)
	require.NoError(t, err)
	assert.Len(t, old, 2)

	_, err = repo.Runner.Blame(ctx, repo.Dir, "../x", "")
	assert.Equal(t, git.ArgInvalidPath, stageArgCode(t, err))
}

func TestHunkStaging(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	var before strings.Builder
	for i := 1; i <= 30; i++ {
		before.WriteString("line " + string(rune('A'+i%26)) + "\n")
	}
	repo.WriteFile("f.txt", before.String())
	repo.Commit("first")
	lines := strings.Split(strings.TrimSuffix(before.String(), "\n"), "\n")
	lines[1] = "CHANGED top"
	lines = append(lines[:15], append([]string{"inserted middle 1", "inserted middle 2"}, lines[15:]...)...)
	lines[len(lines)-2] = "CHANGED bottom"
	repo.WriteFile("f.txt", strings.Join(lines, "\n")+"\n")

	files, err := repo.Runner.Diff(ctx, repo.Dir, git.DiffOptions{})
	require.NoError(t, err)
	require.Len(t, files[0].Hunks, 3)

	// Stage only the last hunk: the middle insertion it skips shifts it.
	require.NoError(t, repo.Runner.StageHunks(ctx, repo.Dir, files[0], []int{2}))
	staged := repo.Git("diff", "--cached")
	assert.Contains(t, staged, "+CHANGED bottom")
	assert.NotContains(t, staged, "CHANGED top")
	assert.NotContains(t, staged, "inserted middle")

	require.NoError(t, repo.Runner.StageHunks(ctx, repo.Dir, mustUnstagedDiff(t, repo), []int{0}))
	assert.Contains(t, repo.Git("diff", "--cached"), "+CHANGED top")

	// Unstage the bottom one again.
	stagedFiles, err := repo.Runner.Diff(ctx, repo.Dir, git.DiffOptions{Staged: true})
	require.NoError(t, err)
	require.Len(t, stagedFiles[0].Hunks, 2)
	require.NoError(t, repo.Runner.UnstageHunks(ctx, repo.Dir, stagedFiles[0], []int{1}))
	staged = repo.Git("diff", "--cached")
	assert.Contains(t, staged, "+CHANGED top")
	assert.NotContains(t, staged, "CHANGED bottom")
	assert.Equal(t, strings.Join(lines, "\n")+"\n", repo.ReadFile("f.txt"), "the working tree never changes")

	t.Run("what cannot be done by hunk", func(t *testing.T) {
		assert.Equal(t, git.ArgUnsupportedFile, stageArgCode(t, repo.Runner.StageHunks(ctx, repo.Dir, git.FileDiff{Path: "x", Change: git.ChangeAdded}, []int{0})))
		assert.Equal(t, git.ArgUnsupportedFile, stageArgCode(t, repo.Runner.StageHunks(ctx, repo.Dir, files[0], []int{9})))
		assert.Equal(t, git.ArgNoPaths, stageArgCode(t, repo.Runner.StageHunks(ctx, repo.Dir, files[0], nil)))
	})

	t.Run("awkward names", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows cannot create these names")
		}
		repo := gittest.NewRepo(t)
		name := "tab\there \"q\".txt"
		repo.WriteFile(name, "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n")
		repo.Commit("first")
		repo.WriteFile(name, "A\nb\nc\nd\ne\nf\ng\nh\ni\nJ\n")
		require.NoError(t, repo.Runner.StageHunks(ctx, repo.Dir, mustUnstagedDiff(t, repo), []int{1}))
		assert.Contains(t, repo.Git("diff", "--cached"), "+J")
	})
}

func mustUnstagedDiff(t *testing.T, repo *gittest.Repo) git.FileDiff {
	t.Helper()
	files, err := repo.Runner.Diff(context.Background(), repo.Dir, git.DiffOptions{})
	require.NoError(t, err)
	require.Len(t, files, 1)
	return files[0]
}

func TestRemoteAdministration(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)

	require.NoError(t, repo.Runner.AddRemote(ctx, repo.Dir, "origin", "https://example.com/a.git"))
	require.NoError(t, repo.Runner.SetRemoteURL(ctx, repo.Dir, "origin", "https://example.com/push.git", true))
	require.NoError(t, repo.Runner.RenameRemote(ctx, repo.Dir, "origin", "upstream"))
	remotes, err := repo.Runner.Remotes(ctx, repo.Dir)
	require.NoError(t, err)
	assert.Equal(t, []git.Remote{{Name: "upstream", FetchURL: "https://example.com/a.git", PushURL: "https://example.com/push.git"}}, remotes)

	require.NoError(t, repo.Runner.RemoveRemote(ctx, repo.Dir, "upstream"))
	assert.Equal(t, git.KindNoRemote, git.KindOf(repo.Runner.RemoveRemote(ctx, repo.Dir, "upstream")))
	for _, url := range []string{"--upload-pack=x", "", "a\nb"} {
		var argErr *git.ArgError
		assert.ErrorAs(t, repo.Runner.AddRemote(ctx, repo.Dir, "x", url), &argErr)
	}
	assert.Equal(t, git.ArgInvalidRemote, stageArgCode(t, repo.Runner.AddRemote(ctx, repo.Dir, "-x", "https://e.com/x")))
}

func TestSubmodules(t *testing.T) {
	ctx := context.Background()
	lib := gittest.NewRepo(t)
	lib.WriteFile("lib.txt", "lib\n")
	lib.Commit("lib")
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("first")
	repo.Git("-c", "protocol.file.allow=always", "submodule", "add", "--quiet", lib.Dir, "vendor/lib")
	repo.Commit("add submodule")

	subs, err := repo.Runner.Submodules(ctx, repo.Dir)
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, "vendor/lib", subs[0].Path)
	assert.True(t, subs[0].Initialized)

	clone := gittest.Clone(t, repo)
	subs, err = clone.Runner.Submodules(ctx, clone.Dir)
	require.NoError(t, err)
	assert.False(t, subs[0].Initialized)
	require.NoError(t, clone.Runner.UpdateSubmodules(ctx, clone.Dir))
	assert.Equal(t, "lib\n", clone.ReadFile("vendor/lib/lib.txt"))
}

func TestSetIdentity(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	require.NoError(t, repo.Runner.SetIdentity(ctx, repo.Dir, false, git.Identity{Name: "Repo Name"}))
	id, err := repo.Runner.Identity(ctx, repo.Dir)
	require.NoError(t, err)
	assert.Equal(t, git.Identity{Name: "Repo Name", Email: gittest.UserEmail}, id)

	require.NoError(t, repo.Runner.SetIdentity(ctx, repo.Dir, true, git.Identity{Email: "global@example.com"}))
	id, err = repo.Runner.Identity(ctx, gittest.TempDir(t))
	require.NoError(t, err)
	assert.Equal(t, "global@example.com", id.Email)
}
