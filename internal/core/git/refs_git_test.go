package git_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
)

func TestGitPath(t *testing.T) {
	ctx := context.Background()
	repo := branchRepo(t)

	path, err := repo.Runner.GitPath(ctx, repo.Dir, "gitpal/history.jsonl")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(repo.Dir, ".git", "gitpal", "history.jsonl"), path)

	t.Run("a linked worktree has its own", func(t *testing.T) {
		linked := filepath.Join(gittest.TempDir(t), "linked")
		repo.Git("worktree", "add", "--quiet", "-b", "linked", linked)

		path, err := repo.Runner.GitPath(ctx, linked, "gitpal/history.jsonl")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(repo.Dir, ".git", "worktrees", "linked", "gitpal", "history.jsonl"), path)
	})

	t.Run("refuses names that leave the git directory", func(t *testing.T) {
		for _, bad := range []string{"", "../outside", "/abs", "--git-dir"} {
			_, err := repo.Runner.GitPath(ctx, repo.Dir, bad)
			assert.Error(t, err, bad)
		}
	})

	t.Run("outside a repository", func(t *testing.T) {
		_, err := repo.Runner.GitPath(ctx, gittest.TempDir(t), "x")
		assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	})
}

func TestResolveCommit(t *testing.T) {
	ctx := context.Background()
	repo := branchRepo(t)
	head := repo.Git("rev-parse", "HEAD")

	got, err := repo.Runner.ResolveCommit(ctx, repo.Dir, "main")
	require.NoError(t, err)
	assert.Equal(t, head, got)

	_, err = repo.Runner.ResolveCommit(ctx, repo.Dir, "nope")
	assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	_, err = repo.Runner.ResolveCommit(ctx, repo.Dir, "--all")
	assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err))
}

// gitpal keeps its own refs under refs/gitpal/; the functions that write refs
// directly can touch nothing else.
func TestOwnRefs(t *testing.T) {
	ctx := context.Background()
	repo := branchRepo(t)
	first := repo.Git("rev-parse", "HEAD")
	repo.WriteFile("a.txt", "b\n")
	second := repo.Commit("second")

	require.NoError(t, repo.Runner.CreateRef(ctx, repo.Dir, "refs/gitpal/backup/one", first))
	require.NoError(t, repo.Runner.CreateRef(ctx, repo.Dir, "refs/gitpal/backup/two", second))

	t.Run("listed by prefix, sorted", func(t *testing.T) {
		refs, err := repo.Runner.Refs(ctx, repo.Dir, "refs/gitpal/backup/")
		require.NoError(t, err)
		assert.Equal(t, []git.Ref{
			{Name: "refs/gitpal/backup/one", Hash: first},
			{Name: "refs/gitpal/backup/two", Hash: second},
		}, refs)

		refs, err = repo.Runner.Refs(ctx, repo.Dir, "refs/gitpal/nothing/")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})

	t.Run("a ref keeps its commit alive and out of the branch list", func(t *testing.T) {
		assert.Equal(t, []string{"main"}, branchNames(branchesOf(t, repo, git.BranchListOptions{Remotes: true})))
	})

	t.Run("creating never overwrites", func(t *testing.T) {
		err := repo.Runner.CreateRef(ctx, repo.Dir, "refs/gitpal/backup/one", second)
		require.Error(t, err)
		assert.Equal(t, first, repo.Git("rev-parse", "refs/gitpal/backup/one"))
	})

	t.Run("delete", func(t *testing.T) {
		require.NoError(t, repo.Runner.DeleteRef(ctx, repo.Dir, "refs/gitpal/backup/two"))
		refs, err := repo.Runner.Refs(ctx, repo.Dir, "refs/gitpal/")
		require.NoError(t, err)
		assert.Len(t, refs, 1)
	})

	t.Run("nothing outside refs/gitpal/ can be written or deleted", func(t *testing.T) {
		for _, bad := range []string{
			"refs/heads/main", "refs/tags/v1", "HEAD", "refs/gitpal/../heads/main", "refs/gitpal/", "--stdin", "refs/gitpal/a b", "",
		} {
			assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, repo.Runner.CreateRef(ctx, repo.Dir, bad, first)), bad)
			assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, repo.Runner.DeleteRef(ctx, repo.Dir, bad)), bad)
		}
		assert.Equal(t, second, repo.Git("rev-parse", "refs/heads/main"))
	})

	t.Run("a target that is not a commit", func(t *testing.T) {
		err := repo.Runner.CreateRef(ctx, repo.Dir, "refs/gitpal/backup/bad", "nope")
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	})
}

func TestWorktreeSnapshot(t *testing.T) {
	ctx := context.Background()

	t.Run("captures staged and unstaged changes without touching anything", func(t *testing.T) {
		repo := stageRepo(t, "a.txt", "b.txt")
		repo.WriteFile("a.txt", "staged\n")
		repo.Git("add", "a.txt")
		repo.WriteFile("b.txt", "unstaged\n")
		before := stageShort(repo)

		hash, ok, err := repo.Runner.WorktreeSnapshot(ctx, repo.Dir)
		require.NoError(t, err)
		require.True(t, ok)

		assert.Equal(t, before, stageShort(repo), "index and working tree are as they were")
		assert.Empty(t, repo.Git("stash", "list"), "nothing shows up in the user's stash")
		assert.Equal(t, "unstaged", repo.Git("show", hash+":b.txt"))
		assert.Equal(t, "staged", repo.Git("show", hash+":a.txt"))

		t.Run("and can be put back after the changes are gone", func(t *testing.T) {
			repo.Git("reset", "--quiet", "--hard")
			require.Empty(t, stageShort(repo))

			require.NoError(t, repo.Runner.ApplySnapshot(ctx, repo.Dir, hash))

			assert.Equal(t, before, stageShort(repo))
			assert.Equal(t, "unstaged\n", repo.ReadFile("b.txt"))
		})
	})

	t.Run("nothing to capture", func(t *testing.T) {
		repo := stageRepo(t, "a.txt")
		repo.WriteFile("untracked.txt", "x\n")

		_, ok, err := repo.Runner.WorktreeSnapshot(ctx, repo.Dir)
		require.NoError(t, err)
		assert.False(t, ok, "untracked files are not at risk and are not captured")
	})

	t.Run("before the first commit", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("a.txt", "x\n")
		repo.Git("add", "a.txt")

		_, ok, err := repo.Runner.WorktreeSnapshot(ctx, repo.Dir)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("a hostile snapshot name", func(t *testing.T) {
		repo := stageRepo(t, "a.txt")
		assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, repo.Runner.ApplySnapshot(ctx, repo.Dir, "--index")))
	})
}

func TestReset(t *testing.T) {
	ctx := context.Background()
	setup := func(t *testing.T) (*gittest.Repo, string) {
		repo := stageRepo(t, "a.txt")
		first := repo.Git("rev-parse", "HEAD")
		repo.WriteFile("a.txt", "second\n")
		repo.Commit("second")
		repo.WriteFile("a.txt", "uncommitted\n")
		repo.WriteFile("untracked.txt", "mine\n")
		return repo, first
	}

	t.Run("soft keeps index and working tree", func(t *testing.T) {
		repo, first := setup(t)
		repo.Git("add", "a.txt")
		require.NoError(t, repo.Runner.Reset(ctx, repo.Dir, git.ResetSoft, first))
		assert.Equal(t, first, repo.Git("rev-parse", "HEAD"))
		assert.Equal(t, "M  a.txt\n?? untracked.txt", stageShort(repo))
		assert.Equal(t, "uncommitted\n", repo.ReadFile("a.txt"))
	})

	t.Run("mixed keeps the working tree", func(t *testing.T) {
		repo, first := setup(t)
		repo.Git("add", "a.txt")
		require.NoError(t, repo.Runner.Reset(ctx, repo.Dir, git.ResetMixed, first))
		assert.Equal(t, " M a.txt\n?? untracked.txt", stageShort(repo))
		assert.Equal(t, "uncommitted\n", repo.ReadFile("a.txt"))
	})

	t.Run("hard discards tracked changes, keeps untracked files", func(t *testing.T) {
		repo, first := setup(t)
		require.NoError(t, repo.Runner.Reset(ctx, repo.Dir, git.ResetHard, first))
		assert.Equal(t, first, repo.Git("rev-parse", "HEAD"))
		assert.Equal(t, "original\n", repo.ReadFile("a.txt"))
		assert.Equal(t, "?? untracked.txt", stageShort(repo))
	})

	t.Run("bad arguments", func(t *testing.T) {
		repo, first := setup(t)
		assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, repo.Runner.Reset(ctx, repo.Dir, git.ResetHard, "--soft")))
		assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, repo.Runner.Reset(ctx, repo.Dir, "--merge", first)))
		assert.Equal(t, git.KindRefNotFound, git.KindOf(repo.Runner.Reset(ctx, repo.Dir, git.ResetHard, "nope")))
		assert.Equal(t, "uncommitted\n", repo.ReadFile("a.txt"))
	})
}
