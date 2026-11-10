package gittest_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eminalan/gitpal/internal/core/git"
	"github.com/eminalan/gitpal/internal/core/git/gittest"
)

// Spec §17: tests must never read or touch the developer's real git
// configuration. This simulates a developer machine and checks that nothing
// from it reaches git.
func TestRunnerIgnoresTheDevelopersRealConfiguration(t *testing.T) {
	home := t.TempDir()
	realConfig := filepath.Join(home, ".gitconfig")
	require.NoError(t, os.WriteFile(realConfig, []byte(
		"[user]\n\tname = Real Developer\n\temail = real@example.com\n[commit]\n\tgpgsign = true\n[gitpal]\n\tleak = yes\n",
	), 0o600))
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", realConfig)
	t.Setenv("GIT_AUTHOR_NAME", "Env Author")
	t.Setenv("GIT_COMMITTER_EMAIL", "env@example.com")

	ctx := context.Background()
	repo := gittest.NewRepo(t)

	id, err := repo.Runner.Identity(ctx, repo.Dir)
	require.NoError(t, err)
	assert.Equal(t, git.Identity{Name: "Test User", Email: "test@example.com"}, id)

	_, leaked, err := repo.Runner.ConfigGet(ctx, repo.Dir, "gitpal.leak")
	require.NoError(t, err)
	assert.False(t, leaked, "a key from the developer's ~/.gitconfig reached the test")

	repo.WriteFile("a.txt", "a\n")
	repo.Commit("first")
	assert.Equal(t, "Test User <test@example.com>", repo.Git("log", "-1", "--format=%an <%ae>"))
	assert.Equal(t, "Test User <test@example.com>", repo.Git("log", "-1", "--format=%cn <%ce>"))

	after, err := os.ReadFile(realConfig)
	require.NoError(t, err)
	assert.Contains(t, string(after), "Real Developer", "the developer's config file must be left untouched")
}

func TestNewRepoStartsOnMainAndCommits(t *testing.T) {
	repo := gittest.NewRepo(t)
	repo.WriteFile("dir/a.txt", "hello\n")

	hash := repo.Commit("first commit")

	assert.Len(t, hash, 40)
	assert.Equal(t, hash, repo.Git("rev-parse", "HEAD"))
	assert.Equal(t, "main", repo.Git("branch", "--show-current"))
	assert.Equal(t, "first commit", repo.Git("log", "-1", "--format=%s"))
	assert.FileExists(t, filepath.Join(repo.Dir, "dir", "a.txt"))
}

func TestCloneSharesHistoryWithItsRemote(t *testing.T) {
	origin := gittest.NewBareRepo(t)
	work := gittest.Clone(t, origin)
	work.WriteFile("a.txt", "a\n")
	hash := work.Commit("first")
	work.Git("push", "origin", "main")

	other := gittest.Clone(t, origin)

	assert.Equal(t, hash, other.Git("rev-parse", "HEAD"))
	assert.Equal(t, "origin/main", other.Git("rev-parse", "--abbrev-ref", "main@{upstream}"))
}
