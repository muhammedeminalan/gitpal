package git_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
)

func TestInit(t *testing.T) {
	ctx := context.Background()
	runner := gittest.NewRunner(t)
	dir := filepath.Join(gittest.TempDir(t), "new project")

	require.NoError(t, runner.Init(ctx, dir, "trunk"))

	root, err := runner.TopLevel(ctx, dir)
	require.NoError(t, err)
	assert.Equal(t, dir, root)
	branch, err := runner.CurrentBranch(ctx, dir)
	require.NoError(t, err)
	assert.Equal(t, "trunk", branch)

	require.NoError(t, runner.Init(ctx, dir, ""), "running it again is harmless")
	assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, runner.Init(ctx, dir, "--bare")))

	t.Run("a directory called like an option", func(t *testing.T) {
		weird := filepath.Join(gittest.TempDir(t), "-x")
		require.NoError(t, runner.Init(ctx, weird, ""))
		_, err := os.Stat(filepath.Join(weird, ".git"))
		require.NoError(t, err)
	})
}

func TestClone(t *testing.T) {
	ctx := context.Background()
	origin, first, _ := remoteSetup(t)
	first.Git("switch", "--quiet", "-c", "feat")
	first.Git("push", "--quiet", "origin", "feat")

	t.Run("full clone with progress", func(t *testing.T) {
		dest := filepath.Join(gittest.TempDir(t), "copy")
		var progress bytes.Buffer

		require.NoError(t, origin.Runner.Clone(ctx, origin.Dir, dest, git.CloneOptions{Progress: &progress}))

		head, ok, err := origin.Runner.Head(ctx, dest)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, origin.Git("rev-parse", "main"), head)
		assert.Contains(t, progress.String(), "Cloning into")
	})

	t.Run("a branch, shallow", func(t *testing.T) {
		dest := filepath.Join(gittest.TempDir(t), "copy")
		require.NoError(t, origin.Runner.Clone(ctx, "file://"+filepath.ToSlash(origin.Dir), dest, git.CloneOptions{Branch: "feat", Depth: 1}))
		branch, err := origin.Runner.CurrentBranch(ctx, dest)
		require.NoError(t, err)
		assert.Equal(t, "feat", branch)
	})

	t.Run("into a directory that is not empty", func(t *testing.T) {
		dest := gittest.TempDir(t)
		require.NoError(t, os.WriteFile(filepath.Join(dest, "x"), []byte("x"), 0o600))
		require.Error(t, origin.Runner.Clone(ctx, origin.Dir, dest, git.CloneOptions{}))
	})

	t.Run("a repository that does not exist", func(t *testing.T) {
		err := origin.Runner.Clone(ctx, filepath.Join(gittest.TempDir(t), "nope"), filepath.Join(gittest.TempDir(t), "c"), git.CloneOptions{})
		assert.Equal(t, git.KindRemoteUnreachable, git.KindOf(err))
	})

	t.Run("credentials never reach the progress output or the error", func(t *testing.T) {
		var progress bytes.Buffer
		err := origin.Runner.Clone(ctx, "http://user:s3cr3t@127.0.0.1:1/x.git", filepath.Join(gittest.TempDir(t), "c"),
			git.CloneOptions{Progress: &progress})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "s3cr3t")
		assert.NotContains(t, progress.String(), "s3cr3t")
	})

	t.Run("hostile arguments", func(t *testing.T) {
		dest := filepath.Join(gittest.TempDir(t), "c")
		for _, bad := range []string{"--upload-pack=touch /tmp/x", "-u", "", "a\nb"} {
			var argErr *git.ArgError
			assert.ErrorAs(t, origin.Runner.Clone(ctx, bad, dest, git.CloneOptions{}), &argErr, bad)
		}
		var argErr *git.ArgError
		assert.ErrorAs(t, origin.Runner.Clone(ctx, origin.Dir, "--bare", git.CloneOptions{}), &argErr)
		assert.ErrorAs(t, origin.Runner.Clone(ctx, origin.Dir, dest, git.CloneOptions{Branch: "--x"}), &argErr)
	})
}

func TestFetchRefAndDiffSince(t *testing.T) {
	ctx := context.Background()
	origin, first, second := remoteSetup(t)
	first.Git("switch", "--quiet", "-c", "feature")
	first.WriteFile("f.txt", "feature\n")
	first.Commit("feature work")
	first.Git("push", "--quiet", "origin", "feature:refs/pull/7/head")
	_ = origin

	require.NoError(t, second.Runner.FetchRef(ctx, second.Dir, "origin", "refs/pull/7/head", "pr-7"))
	assert.Equal(t, first.Git("rev-parse", "HEAD"), second.Git("rev-parse", "pr-7"))
	assert.Equal(t, git.KindNoRemote, git.KindOf(second.Runner.FetchRef(ctx, second.Dir, "nope", "x", "y")))
	assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, second.Runner.FetchRef(ctx, second.Dir, "origin", "x", "--force")))

	files, err := first.Runner.Diff(ctx, first.Dir, git.DiffOptions{Since: "main"})
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "f.txt", files[0].Path)
	_, err = first.Runner.Diff(ctx, first.Dir, git.DiffOptions{Since: "--output=x"})
	assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err))
}
