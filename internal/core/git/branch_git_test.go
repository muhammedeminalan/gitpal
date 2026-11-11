package git_test

import (
	"context"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
)

func branchesOf(t *testing.T, repo *gittest.Repo, opts git.BranchListOptions) []git.Branch {
	t.Helper()
	branches, err := repo.Runner.Branches(context.Background(), repo.Dir, opts)
	require.NoError(t, err)
	return branches
}

func branchNames(branches []git.Branch) []string {
	names := make([]string, 0, len(branches))
	for _, b := range branches {
		names = append(names, b.Name)
	}
	return names
}

func branchNamed(t *testing.T, branches []git.Branch, ref string) git.Branch {
	t.Helper()
	for _, b := range branches {
		if b.Ref == ref {
			return b
		}
	}
	require.Failf(t, "no branch", "ref %q not among %v", ref, branchNames(branches))
	return git.Branch{}
}

func branchCurrent(t *testing.T, repo *gittest.Repo) string {
	t.Helper()
	name, err := repo.Runner.CurrentBranch(context.Background(), repo.Dir)
	require.NoError(t, err)
	return name
}

// branchRepo returns a repository with one commit on main.
func branchRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("first")
	return repo
}

// branchRemoteRepo returns a clone whose origin has main and feat.
func branchRemoteRepo(t *testing.T) (origin, repo *gittest.Repo) {
	t.Helper()
	origin = gittest.NewBareRepo(t)
	seed := gittest.Clone(t, origin)
	seed.WriteFile("a.txt", "a\n")
	seed.Commit("first")
	seed.Git("push", "--quiet", "--set-upstream", "origin", "main")
	seed.Git("switch", "--quiet", "-c", "feat")
	seed.WriteFile("feat.txt", "feat\n")
	seed.Commit("feat work")
	seed.Git("push", "--quiet", "--set-upstream", "origin", "feat")
	return origin, gittest.Clone(t, origin)
}

func TestBranchesUnbornRepository(t *testing.T) {
	repo := gittest.NewRepo(t)

	assert.Empty(t, branchesOf(t, repo, git.BranchListOptions{Remotes: true}))
	assert.Equal(t, "main", branchCurrent(t, repo), "the branch about to be born")
}

func TestBranchesList(t *testing.T) {
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Git("add", "--all")
	_, err := repo.Runner.Run(context.Background(), git.Cmd{
		Dir: repo.Dir, Args: []string{"commit", "--quiet", "-m", "ilk commit\n\ngövde"},
		Env: []string{"GIT_COMMITTER_DATE=2026-03-01T10:00:00+03:00"},
	})
	require.NoError(t, err)
	first := repo.Git("rev-parse", "HEAD")
	repo.Git("branch", "özellik/giriş")
	repo.Git("switch", "--quiet", "-c", "feat/x")
	repo.WriteFile("b.txt", "b\n")
	second := repo.Commit("ikinci")

	branches := branchesOf(t, repo, git.BranchListOptions{})

	assert.Equal(t, []string{"feat/x", "main", "özellik/giriş"}, branchNames(branches))
	feat := branches[0]
	assert.Equal(t, "refs/heads/feat/x", feat.Ref)
	assert.True(t, feat.Current)
	assert.Equal(t, second, feat.Hash)
	assert.Equal(t, "ikinci", feat.Subject)
	assert.False(t, feat.Remote)
	assert.Empty(t, feat.RemoteName)
	assert.Empty(t, feat.Upstream)

	main := branches[1]
	assert.False(t, main.Current)
	assert.Equal(t, first, main.Hash)
	assert.Equal(t, "ilk commit", main.Subject)
	assert.True(t, main.CommitterDate.Equal(time.Date(2026, 3, 1, 7, 0, 0, 0, time.UTC)))

	assert.Equal(t, "feat/x", branchCurrent(t, repo))
}

func TestBranchesDetachedHead(t *testing.T) {
	repo := branchRepo(t)
	repo.Git("switch", "--quiet", "--detach")

	branches := branchesOf(t, repo, git.BranchListOptions{})

	require.Len(t, branches, 1)
	assert.False(t, branches[0].Current)
	assert.Empty(t, branchCurrent(t, repo))
}

func TestBranchesRemoteTracking(t *testing.T) {
	origin, repo := branchRemoteRepo(t)
	// The clone has origin/HEAD, a symbolic ref that is not a branch.
	repo.Git("remote", "set-head", "origin", "main")
	repo.Git("switch", "--quiet", "feat")
	repo.Git("switch", "--quiet", "main")
	repo.WriteFile("local.txt", "local\n")
	repo.Commit("local work")
	// A local branch that looks like a remote-tracking one.
	repo.Git("branch", "origin/main")

	t.Run("local only by default", func(t *testing.T) {
		assert.Equal(t, []string{"feat", "main", "origin/main"}, branchNames(branchesOf(t, repo, git.BranchListOptions{})))
	})

	branches := branchesOf(t, repo, git.BranchListOptions{Remotes: true})

	t.Run("remote-tracking branches, without origin/HEAD", func(t *testing.T) {
		assert.Equal(t, []string{"feat", "main", "origin/main", "origin/feat", "origin/main"}, branchNames(branches))
		remote := branchNamed(t, branches, "refs/remotes/origin/feat")
		assert.True(t, remote.Remote)
		assert.Equal(t, "origin", remote.RemoteName)
		assert.Equal(t, "origin/feat", remote.Name)
		assert.Empty(t, remote.Upstream)
	})

	t.Run("a local branch named like a remote one stays local", func(t *testing.T) {
		local := branchNamed(t, branches, "refs/heads/origin/main")
		assert.False(t, local.Remote)
		assert.Equal(t, "origin/main", local.Name)
		assert.Empty(t, local.RemoteName)
	})

	t.Run("upstream, ahead and behind", func(t *testing.T) {
		seed := gittest.Clone(t, origin)
		seed.WriteFile("remote.txt", "remote\n")
		seed.Commit("remote work one")
		seed.WriteFile("remote.txt", "remote, again\n")
		seed.Commit("remote work two")
		seed.Git("push", "--quiet")
		repo.Git("fetch", "--quiet")

		main := branchNamed(t, branchesOf(t, repo, git.BranchListOptions{}), "refs/heads/main")
		assert.Equal(t, "origin/main", main.Upstream)
		assert.False(t, main.UpstreamGone)
		assert.Equal(t, 1, main.Ahead)
		assert.Equal(t, 2, main.Behind)
	})

	t.Run("upstream deleted on the remote", func(t *testing.T) {
		seed := gittest.Clone(t, origin)
		seed.Git("push", "--quiet", "origin", "--delete", "feat")
		repo.Git("fetch", "--quiet", "--prune")

		feat := branchNamed(t, branchesOf(t, repo, git.BranchListOptions{}), "refs/heads/feat")
		assert.Equal(t, "origin/feat", feat.Upstream)
		assert.True(t, feat.UpstreamGone)
		assert.Zero(t, feat.Ahead)
	})
}

func TestValidateBranchName(t *testing.T) {
	ctx := context.Background()
	repo := branchRepo(t)
	repo.Git("switch", "--quiet", "-c", "other")
	repo.Git("switch", "--quiet", "main")

	for _, good := range []string{"feat/x", "özellik/giriş", "v1.2", "a-b_c", "head", "main"} {
		assert.NoError(t, repo.Runner.ValidateBranchName(ctx, repo.Dir, good), good)
	}

	for _, bad := range []string{
		"", "--force", "-D", "a..b", "@{-1}", "@", "HEAD", "x.lock", "a b", "a\tb", "a\nb",
		"refs/heads/x", "feat/", "/feat", "a//b", ".hidden", "a~1", "a^", "a:b", "a?b", "a*b", "a[b", "a\\b", "a.",
	} {
		err := repo.Runner.ValidateBranchName(ctx, repo.Dir, bad)
		var argErr *git.ArgError
		require.ErrorAs(t, err, &argErr, "%q", bad)
		assert.Equal(t, git.ArgInvalidBranch, argErr.Code)
		assert.Equal(t, bad, argErr.Value)
	}
}

// Whatever SuggestBranchName returns, git accepts as it is.
func TestSuggestBranchNameAlwaysValid(t *testing.T) {
	ctx := context.Background()
	repo := branchRepo(t)
	inputs := []string{
		"feat/Yeni Özellik!", "x.lock", "a..b", "@{-1}", "--force", "HEAD", "refs/heads/x", "a~b^c:d?e*f[g\\h",
		"//a//", ".a.", "a.lock/b.lock", "ÇĞIİÖŞÜ", "🚀", "a/./b", "a/../b", "@", "a@{b", "-", ".",
	}
	// A fixed seed: the same strings on every run.
	rng := rand.New(rand.NewPCG(1, 2))
	alphabet := []rune("ab Z9-_./\\@{}~^:?*[]çİş🚀\t.lock")
	for range 300 {
		n := 1 + rng.IntN(14)
		runes := make([]rune, n)
		for i := range runes {
			runes[i] = alphabet[rng.IntN(len(alphabet))]
		}
		inputs = append(inputs, string(runes))
	}

	for _, input := range inputs {
		name := git.SuggestBranchName(input)
		if name == "" {
			continue
		}
		assert.NoError(t, repo.Runner.ValidateBranchName(ctx, repo.Dir, name), "input %q gave %q", input, name)
	}
}

func TestCreateBranch(t *testing.T) {
	ctx := context.Background()

	t.Run("from HEAD, without switching", func(t *testing.T) {
		repo := branchRepo(t)

		require.NoError(t, repo.Runner.CreateBranch(ctx, repo.Dir, "feat/x", ""))

		assert.Equal(t, repo.Git("rev-parse", "main"), repo.Git("rev-parse", "feat/x"))
		assert.Equal(t, "main", branchCurrent(t, repo))
	})

	t.Run("from a start point", func(t *testing.T) {
		repo := branchRepo(t)
		first := repo.Git("rev-parse", "HEAD")
		repo.WriteFile("a.txt", "b\n")
		repo.Commit("second")

		require.NoError(t, repo.Runner.CreateBranch(ctx, repo.Dir, "old", first))

		assert.Equal(t, first, repo.Git("rev-parse", "old"))
	})

	t.Run("already exists", func(t *testing.T) {
		repo := branchRepo(t)
		assert.Equal(t, git.KindBranchExists, git.KindOf(repo.Runner.CreateBranch(ctx, repo.Dir, "main", "")))
	})

	t.Run("a start point that does not exist", func(t *testing.T) {
		repo := branchRepo(t)
		assert.Equal(t, git.KindRefNotFound, git.KindOf(repo.Runner.CreateBranch(ctx, repo.Dir, "x", "nope")))
	})

	t.Run("before the first commit", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		assert.Equal(t, git.KindUnbornBranch, git.KindOf(repo.Runner.CreateBranch(ctx, repo.Dir, "x", "")))
	})

	t.Run("hostile names are refused and have no effect", func(t *testing.T) {
		repo := branchRepo(t)
		for _, bad := range []string{"--force", "-D", "a..b", "@{-1}", "HEAD", "x.lock", "a b", "", "refs/heads/x"} {
			assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, repo.Runner.CreateBranch(ctx, repo.Dir, bad, "")), bad)
			assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, repo.Runner.CreateAndSwitch(ctx, repo.Dir, bad, "")), bad)
			assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, repo.Runner.RenameBranch(ctx, repo.Dir, "main", bad)), bad)
		}
		for _, bad := range []string{"--force", "-f", "a b"} {
			assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, repo.Runner.CreateBranch(ctx, repo.Dir, "ok", bad)), bad)
			assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, repo.Runner.CreateAndSwitch(ctx, repo.Dir, "ok", bad)), bad)
		}
		assert.Equal(t, []string{"main"}, branchNames(branchesOf(t, repo, git.BranchListOptions{})))
	})

	// Tracking follows one rule, whatever branch.autoSetupMerge says: a
	// branch started from a remote-tracking branch tracks it, any other does
	// not.
	t.Run("tracking does not depend on configuration", func(t *testing.T) {
		for _, setting := range []string{"always", "false", "true"} {
			_, repo := branchRemoteRepo(t)
			repo.Git("config", "branch.autoSetupMerge", setting)
			repo.Git("config", "branch.autoSetupRebase", "always")

			require.NoError(t, repo.Runner.CreateBranch(ctx, repo.Dir, "from-remote", "origin/feat"))
			require.NoError(t, repo.Runner.CreateBranch(ctx, repo.Dir, "from-local", "main"))
			require.NoError(t, repo.Runner.CreateAndSwitch(ctx, repo.Dir, "switched-remote", "origin/feat"))
			require.NoError(t, repo.Runner.CreateAndSwitch(ctx, repo.Dir, "switched-local", "main"))

			branches := branchesOf(t, repo, git.BranchListOptions{})
			assert.Equal(t, "origin/feat", branchNamed(t, branches, "refs/heads/from-remote").Upstream, setting)
			assert.Empty(t, branchNamed(t, branches, "refs/heads/from-local").Upstream, setting)
			assert.Equal(t, "origin/feat", branchNamed(t, branches, "refs/heads/switched-remote").Upstream, setting)
			assert.Empty(t, branchNamed(t, branches, "refs/heads/switched-local").Upstream, setting)
		}
	})
}

func TestSwitchBranch(t *testing.T) {
	ctx := context.Background()

	t.Run("to an existing branch", func(t *testing.T) {
		repo := branchRepo(t)
		repo.Git("branch", "feat")

		require.NoError(t, repo.Runner.SwitchBranch(ctx, repo.Dir, "feat"))

		assert.Equal(t, "feat", branchCurrent(t, repo))
	})

	t.Run("uncommitted changes that do not conflict come along", func(t *testing.T) {
		repo := branchRepo(t)
		repo.Git("branch", "feat")
		repo.WriteFile("a.txt", "edited\n")

		require.NoError(t, repo.Runner.SwitchBranch(ctx, repo.Dir, "feat"))

		assert.Equal(t, "edited\n", repo.ReadFile("a.txt"))
	})

	t.Run("uncommitted changes that would be lost stop it", func(t *testing.T) {
		repo := branchRepo(t)
		repo.Git("switch", "--quiet", "-c", "feat")
		repo.WriteFile("a.txt", "feat version\n")
		repo.Commit("feat")
		repo.Git("switch", "--quiet", "main")
		repo.WriteFile("a.txt", "my uncommitted work\n")

		err := repo.Runner.SwitchBranch(ctx, repo.Dir, "feat")

		assert.Equal(t, git.KindDirtyWorktree, git.KindOf(err))
		assert.Equal(t, "main", branchCurrent(t, repo))
		assert.Equal(t, "my uncommitted work\n", repo.ReadFile("a.txt"))
	})

	t.Run("a branch that does not exist", func(t *testing.T) {
		repo := branchRepo(t)
		assert.Equal(t, git.KindRefNotFound, git.KindOf(repo.Runner.SwitchBranch(ctx, repo.Dir, "nope")))
	})

	t.Run("a name that only exists on the remote creates a tracking branch", func(t *testing.T) {
		_, repo := branchRemoteRepo(t)
		repo.Git("config", "checkout.guess", "false")

		require.NoError(t, repo.Runner.SwitchBranch(ctx, repo.Dir, "feat"))

		assert.Equal(t, "feat", branchCurrent(t, repo))
		feat := branchNamed(t, branchesOf(t, repo, git.BranchListOptions{}), "refs/heads/feat")
		assert.Equal(t, "origin/feat", feat.Upstream)
		assert.Equal(t, repo.Git("rev-parse", "origin/feat"), feat.Hash)
	})

	t.Run("a name on two remotes is ambiguous", func(t *testing.T) {
		origin, repo := branchRemoteRepo(t)
		repo.Git("remote", "add", "second", origin.Dir)
		repo.Git("fetch", "--quiet", "second")
		repo.Git("config", "checkout.defaultRemote", "second")

		err := repo.Runner.SwitchBranch(ctx, repo.Dir, "feat")

		assert.Equal(t, git.KindAmbiguousRef, git.KindOf(err))
		assert.Equal(t, "main", branchCurrent(t, repo))
		assert.Equal(t, []string{"main"}, branchNames(branchesOf(t, repo, git.BranchListOptions{})))
	})

	t.Run("hostile names are refused and have no effect", func(t *testing.T) {
		repo := branchRepo(t)
		repo.Git("branch", "feat")
		marker := filepath.Join(gittest.TempDir(t), "written-by-git")
		for _, bad := range []string{"--detach", "-", "--orphan=x", "-c", "", "a b", "--output=" + marker} {
			assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, repo.Runner.SwitchBranch(ctx, repo.Dir, bad)), bad)
		}
		assert.Equal(t, "main", branchCurrent(t, repo))
		_, err := os.Stat(marker)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestCreateAndSwitch(t *testing.T) {
	ctx := context.Background()

	t.Run("from HEAD", func(t *testing.T) {
		repo := branchRepo(t)

		require.NoError(t, repo.Runner.CreateAndSwitch(ctx, repo.Dir, "feat/yeni", ""))

		assert.Equal(t, "feat/yeni", branchCurrent(t, repo))
		assert.Equal(t, repo.Git("rev-parse", "main"), repo.Git("rev-parse", "HEAD"))
	})

	t.Run("from a start point", func(t *testing.T) {
		repo := branchRepo(t)
		first := repo.Git("rev-parse", "HEAD")
		repo.WriteFile("a.txt", "b\n")
		repo.Commit("second")

		require.NoError(t, repo.Runner.CreateAndSwitch(ctx, repo.Dir, "old", first))

		assert.Equal(t, first, repo.Git("rev-parse", "HEAD"))
	})

	t.Run("already exists", func(t *testing.T) {
		repo := branchRepo(t)
		repo.Git("branch", "feat")
		assert.Equal(t, git.KindBranchExists, git.KindOf(repo.Runner.CreateAndSwitch(ctx, repo.Dir, "feat", "")))
		assert.Equal(t, "main", branchCurrent(t, repo))
	})

	t.Run("before the first commit it renames the branch to be born", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		require.NoError(t, repo.Runner.CreateAndSwitch(ctx, repo.Dir, "trunk", ""))
		assert.Equal(t, "trunk", branchCurrent(t, repo))
	})
}

func TestRenameBranch(t *testing.T) {
	ctx := context.Background()

	t.Run("another branch", func(t *testing.T) {
		repo := branchRepo(t)
		repo.Git("branch", "old")

		require.NoError(t, repo.Runner.RenameBranch(ctx, repo.Dir, "old", "new"))

		assert.Equal(t, []string{"main", "new"}, branchNames(branchesOf(t, repo, git.BranchListOptions{})))
	})

	t.Run("the current branch", func(t *testing.T) {
		repo := branchRepo(t)

		require.NoError(t, repo.Runner.RenameBranch(ctx, repo.Dir, "main", "trunk"))

		assert.Equal(t, "trunk", branchCurrent(t, repo))
	})

	t.Run("never overwrites an existing branch", func(t *testing.T) {
		repo := branchRepo(t)
		repo.Git("switch", "--quiet", "-c", "feat")
		repo.WriteFile("b.txt", "b\n")
		feat := repo.Commit("feat")

		err := repo.Runner.RenameBranch(ctx, repo.Dir, "main", "feat")

		assert.Equal(t, git.KindBranchExists, git.KindOf(err))
		assert.Equal(t, feat, repo.Git("rev-parse", "feat"))
	})

	t.Run("a branch that does not exist", func(t *testing.T) {
		repo := branchRepo(t)
		assert.Equal(t, git.KindRefNotFound, git.KindOf(repo.Runner.RenameBranch(ctx, repo.Dir, "nope", "new")))
	})

	t.Run("a hostile old name", func(t *testing.T) {
		repo := branchRepo(t)
		assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, repo.Runner.RenameBranch(ctx, repo.Dir, "--force", "new")))
	})
}

func TestDeleteBranch(t *testing.T) {
	ctx := context.Background()

	// main, merged (same commit as main) and unmerged (one commit ahead).
	setup := func(t *testing.T) *gittest.Repo {
		repo := branchRepo(t)
		repo.Git("branch", "merged")
		repo.Git("switch", "--quiet", "-c", "unmerged")
		repo.WriteFile("b.txt", "b\n")
		repo.Commit("work that exists nowhere else")
		repo.Git("switch", "--quiet", "main")
		return repo
	}

	t.Run("a merged branch", func(t *testing.T) {
		repo := setup(t)
		require.NoError(t, repo.Runner.DeleteBranch(ctx, repo.Dir, "merged", false))
		assert.Equal(t, []string{"main", "unmerged"}, branchNames(branchesOf(t, repo, git.BranchListOptions{})))
	})

	t.Run("an unmerged branch is kept", func(t *testing.T) {
		repo := setup(t)
		err := repo.Runner.DeleteBranch(ctx, repo.Dir, "unmerged", false)
		assert.Equal(t, git.KindBranchNotMerged, git.KindOf(err))
		assert.Contains(t, branchNames(branchesOf(t, repo, git.BranchListOptions{})), "unmerged")
	})

	t.Run("an unmerged branch, forced", func(t *testing.T) {
		repo := setup(t)
		require.NoError(t, repo.Runner.DeleteBranch(ctx, repo.Dir, "unmerged", true))
		assert.NotContains(t, branchNames(branchesOf(t, repo, git.BranchListOptions{})), "unmerged")
	})

	t.Run("the branch that is checked out", func(t *testing.T) {
		repo := setup(t)
		for _, force := range []bool{false, true} {
			err := repo.Runner.DeleteBranch(ctx, repo.Dir, "main", force)
			assert.Equal(t, git.KindBranchCheckedOut, git.KindOf(err))
		}
		assert.Equal(t, "main", branchCurrent(t, repo))
	})

	t.Run("a branch that does not exist", func(t *testing.T) {
		repo := setup(t)
		assert.Equal(t, git.KindRefNotFound, git.KindOf(repo.Runner.DeleteBranch(ctx, repo.Dir, "nope", false)))
	})

	t.Run("hostile names are refused and have no effect", func(t *testing.T) {
		repo := setup(t)
		for _, bad := range []string{"--all", "-r", "--force", "", "a b"} {
			assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, repo.Runner.DeleteBranch(ctx, repo.Dir, bad, true)), bad)
		}
		assert.Len(t, branchesOf(t, repo, git.BranchListOptions{}), 3)
	})
}

func TestMergedBranches(t *testing.T) {
	ctx := context.Background()
	repo := branchRepo(t)
	repo.Git("branch", "done-one")
	repo.Git("branch", "done-two")
	repo.Git("switch", "--quiet", "-c", "in-progress")
	repo.WriteFile("b.txt", "b\n")
	repo.Commit("work")
	repo.Git("switch", "--quiet", "main")

	t.Run("into HEAD", func(t *testing.T) {
		merged, err := repo.Runner.MergedBranches(ctx, repo.Dir, "")
		require.NoError(t, err)
		assert.Equal(t, []string{"done-one", "done-two"}, merged, "without main itself and the unmerged branch")
	})

	t.Run("into another branch", func(t *testing.T) {
		merged, err := repo.Runner.MergedBranches(ctx, repo.Dir, "in-progress")
		require.NoError(t, err)
		assert.Equal(t, []string{"done-one", "done-two"}, merged, "without the target and the current branch")
	})

	t.Run("while on one of them", func(t *testing.T) {
		repo.Git("switch", "--quiet", "done-one")
		defer repo.Git("switch", "--quiet", "main")

		merged, err := repo.Runner.MergedBranches(ctx, repo.Dir, "main")
		require.NoError(t, err)
		assert.Equal(t, []string{"done-two"}, merged)
	})

	t.Run("a target that does not exist", func(t *testing.T) {
		_, err := repo.Runner.MergedBranches(ctx, repo.Dir, "nope")
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	})

	t.Run("a hostile target", func(t *testing.T) {
		_, err := repo.Runner.MergedBranches(ctx, repo.Dir, "--no-merged")
		assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err))
	})

	t.Run("before the first commit", func(t *testing.T) {
		empty := gittest.NewRepo(t)
		merged, err := empty.Runner.MergedBranches(ctx, empty.Dir, "")
		require.NoError(t, err)
		assert.Empty(t, merged)
	})
}

func TestSetAndUnsetUpstream(t *testing.T) {
	ctx := context.Background()
	_, repo := branchRemoteRepo(t)
	repo.Git("branch", "--no-track", "work", "origin/feat")
	upstream := func() string {
		return branchNamed(t, branchesOf(t, repo, git.BranchListOptions{}), "refs/heads/work").Upstream
	}
	require.Empty(t, upstream())

	require.NoError(t, repo.Runner.SetUpstream(ctx, repo.Dir, "work", "origin/feat"))
	assert.Equal(t, "origin/feat", upstream())

	require.NoError(t, repo.Runner.UnsetUpstream(ctx, repo.Dir, "work"))
	assert.Empty(t, upstream())

	t.Run("unsetting what is not set", func(t *testing.T) {
		assert.Equal(t, git.KindNoUpstream, git.KindOf(repo.Runner.UnsetUpstream(ctx, repo.Dir, "work")))
	})

	t.Run("an upstream that does not exist", func(t *testing.T) {
		assert.Equal(t, git.KindRefNotFound, git.KindOf(repo.Runner.SetUpstream(ctx, repo.Dir, "work", "origin/nope")))
	})

	t.Run("a branch that does not exist", func(t *testing.T) {
		assert.Equal(t, git.KindRefNotFound, git.KindOf(repo.Runner.SetUpstream(ctx, repo.Dir, "nope", "origin/feat")))
		assert.Equal(t, git.KindRefNotFound, git.KindOf(repo.Runner.UnsetUpstream(ctx, repo.Dir, "nope")))
	})

	t.Run("hostile names", func(t *testing.T) {
		assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, repo.Runner.SetUpstream(ctx, repo.Dir, "--force", "origin/feat")))
		assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, repo.Runner.SetUpstream(ctx, repo.Dir, "work", "--force")))
		assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, repo.Runner.UnsetUpstream(ctx, repo.Dir, "-d")))
	})
}

// Settings in the user's git configuration that change what git prints about
// branches must not change what gitpal reads.
func TestBranchesIgnoreHostileConfiguration(t *testing.T) {
	_, repo := branchRemoteRepo(t)
	repo.Git("branch", "zzz")
	repo.Git("branch", "aaa")
	want := branchesOf(t, repo, git.BranchListOptions{Remotes: true})

	for key, value := range map[string]string{
		"branch.sort":            "-committerdate",
		"column.ui":              "always",
		"column.branch":          "always",
		"color.branch":           "always",
		"core.warnAmbiguousRefs": "false",
		"log.date":               "relative",
		"status.aheadBehind":     "false",
	} {
		repo.Git("config", key, value)
	}

	assert.Equal(t, want, branchesOf(t, repo, git.BranchListOptions{Remotes: true}))
}

func TestBranchesOutsideARepository(t *testing.T) {
	ctx := context.Background()
	runner := gittest.NewRunner(t)
	dir := gittest.TempDir(t)

	_, err := runner.Branches(ctx, dir, git.BranchListOptions{})
	assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	_, err = runner.CurrentBranch(ctx, dir)
	assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	assert.Equal(t, git.KindNotARepo, git.KindOf(runner.SwitchBranch(ctx, dir, "main")))
	assert.Equal(t, git.KindNotARepo, git.KindOf(runner.CreateBranch(ctx, dir, "x", "")))
}
