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

// These tests run the real git binary, always against throw-away repositories
// and an isolated configuration (see gittest).

func TestInstalledGitIsSupported(t *testing.T) {
	version, err := gittest.NewRunner(t).Version(context.Background())
	require.NoError(t, err)

	assert.True(t, version.AtLeast(git.MinVersion), "git %s is older than the supported minimum %s", version, git.MinVersion)
}

// The scenario behind environment scrubbing: gitpal started from a hook of
// another repository must still act on the repository it was pointed at.
func TestRunIgnoresRepositoryInheritedFromAHook(t *testing.T) {
	outer := gittest.NewRepo(t)
	inner := gittest.NewRepo(t)
	t.Setenv("GIT_DIR", filepath.Join(outer.Dir, ".git"))
	t.Setenv("GIT_WORK_TREE", outer.Dir)
	// Unlike gittest.NewRunner, this runner inherits the process environment,
	// GIT_DIR included — exactly what gitpal's own runner does in production.
	runner := git.NewRunner(git.WithEnv(gittest.Env(t)...))

	got, err := runner.TopLevel(context.Background(), inner.Dir)
	require.NoError(t, err)

	assert.Equal(t, inner.Dir, got)
}

func TestTopLevel(t *testing.T) {
	repo := gittest.NewRepo(t)
	nested := repo.MkdirAll("deep/down")

	t.Run("from a nested directory", func(t *testing.T) {
		got, err := repo.Runner.TopLevel(context.Background(), nested)
		require.NoError(t, err)
		assert.Equal(t, repo.Dir, got)
	})

	t.Run("outside any repository", func(t *testing.T) {
		_, err := repo.Runner.TopLevel(context.Background(), gittest.TempDir(t))
		assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	})

	t.Run("directory that does not exist", func(t *testing.T) {
		_, err := repo.Runner.TopLevel(context.Background(), filepath.Join(repo.Dir, "nope"))
		require.Error(t, err)
		assert.NotEqual(t, git.KindGitNotFound, git.KindOf(err))
	})
}

func TestIdentity(t *testing.T) {
	ctx := context.Background()

	t.Run("from global configuration", func(t *testing.T) {
		id, err := gittest.NewRunner(t).Identity(ctx, gittest.TempDir(t))
		require.NoError(t, err)
		assert.Equal(t, git.Identity{Name: "Test User", Email: "test@example.com"}, id)
		assert.True(t, id.Complete())
	})

	t.Run("empty when nothing is configured", func(t *testing.T) {
		id, err := gittest.NewRunner(t, gittest.WithoutIdentity()).Identity(ctx, gittest.TempDir(t))
		require.NoError(t, err)
		assert.Equal(t, git.Identity{}, id)
		assert.False(t, id.Complete())
	})

	t.Run("repository setting wins over global", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.Git("config", "user.email", "work@example.com")

		id, err := repo.Runner.Identity(ctx, repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, git.Identity{Name: "Test User", Email: "work@example.com"}, id)
	})

	t.Run("a name alone is not a complete identity", func(t *testing.T) {
		assert.False(t, git.Identity{Name: "Test User"}.Complete())
		assert.False(t, git.Identity{Email: "test@example.com"}.Complete())
	})
}

func TestConfigGet(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	repo.Git("config", "gitpal.sample", "değer with spaces")

	t.Run("existing key", func(t *testing.T) {
		value, found, err := repo.Runner.ConfigGet(ctx, repo.Dir, "gitpal.sample")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "değer with spaces", value)
	})

	t.Run("missing key is not an error", func(t *testing.T) {
		value, found, err := repo.Runner.ConfigGet(ctx, repo.Dir, "gitpal.absent")
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, value)
	})

	t.Run("malformed key is an error", func(t *testing.T) {
		_, _, err := repo.Runner.ConfigGet(ctx, repo.Dir, "not a key")
		assert.Error(t, err)
	})
}

func TestHead(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)

	t.Run("unborn branch has no head commit", func(t *testing.T) {
		hash, ok, err := repo.Runner.Head(ctx, repo.Dir)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Empty(t, hash)
	})

	t.Run("after the first commit", func(t *testing.T) {
		repo.WriteFile("a.txt", "a\n")
		want := repo.Commit("first")

		hash, ok, err := repo.Runner.Head(ctx, repo.Dir)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, want, hash)
	})

	t.Run("outside a repository", func(t *testing.T) {
		_, _, err := repo.Runner.Head(ctx, gittest.TempDir(t))
		assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	})
}
