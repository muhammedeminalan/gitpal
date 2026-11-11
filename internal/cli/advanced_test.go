package cli_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/cli"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
	"github.com/muhammedeminalan/gitpal/internal/tui"
)

// conflicted returns a harness whose repository has branches "work" and
// "other" that changed a.txt differently.
func conflicted(t *testing.T) (*harness, *gittest.Repo) {
	t.Helper()
	h, repo := inRepo(t)
	repo.Git("switch", "--quiet", "-c", "other")
	repo.WriteFile("a.txt", "theirs\n")
	repo.Commit("theirs")
	repo.Git("switch", "--quiet", "work")
	repo.WriteFile("a.txt", "ours\n")
	repo.Commit("ours")
	return h, repo
}

func TestMergeConflictResolveContinue(t *testing.T) {
	h, repo := conflicted(t)

	assert.Equal(t, cli.ExitError, h.run("merge", "other", "--yes"))
	assert.Contains(t, h.stderr.String(), "gitpal resolve")

	require.Equal(t, cli.ExitOK, h.run("conflicts"))
	assert.Contains(t, h.stdout.String(), "a.txt")
	assert.Contains(t, h.stdout.String(), "merge")

	require.Equal(t, cli.ExitOK, h.run("resolve", "a.txt", "--theirs"))
	require.Equal(t, cli.ExitOK, h.run("continue"))
	assert.Equal(t, "theirs\n", repo.ReadFile("a.txt"))
	assert.Len(t, strings.Fields(repo.Git("log", "-1", "--format=%P")), 2)

	require.Equal(t, cli.ExitOK, h.run("undo", "--yes"))
	assert.Equal(t, "ours\n", repo.ReadFile("a.txt"), "the merge's undo brings back the state before it")
}

func TestMergeAbort(t *testing.T) {
	h, repo := conflicted(t)
	before := repo.Git("rev-parse", "HEAD")
	require.Equal(t, cli.ExitError, h.run("merge", "other", "--yes"))

	assert.Equal(t, cli.ExitBlocked, h.run("abort"), "a yes is needed")
	require.Equal(t, cli.ExitOK, h.run("abort", "--yes"))
	assert.Equal(t, before, repo.Git("rev-parse", "HEAD"))

	assert.Equal(t, cli.ExitError, h.run("continue"))
	assert.Contains(t, h.stderr.String(), "no merge, rebase")
}

func TestInteractiveRebase(t *testing.T) {
	h, repo := inRepo(t)
	for _, n := range []string{"one", "two", "three"} {
		repo.WriteFile(n+".txt", n+"\n")
		repo.Commit(n)
	}
	h.interactive = true
	var shown []git.RebaseStep
	h.env.RebaseEditor = func(_ context.Context, opts tui.RebaseOptions) ([]git.RebaseStep, error) {
		shown = opts.Steps
		steps := append([]git.RebaseStep(nil), opts.Steps...)
		steps[1].Action = git.RebaseFixup
		steps[2].Action, steps[2].Message = git.RebaseReword, "THREE"
		return steps, nil
	}
	h.env.Stdin = strings.NewReader("y\n")

	require.Equal(t, cli.ExitOK, h.run("rebase", "-i", "main"), h.stderr.String())

	require.Len(t, shown, 3)
	assert.Equal(t, "one", shown[0].Subject)
	assert.Equal(t, []string{"THREE", "one", "first"}, strings.Split(repo.Git("log", "--format=%s"), "\n"))

	t.Run("nothing to rebase", func(t *testing.T) {
		require.Equal(t, cli.ExitOK, h.run("rebase", "-i", "HEAD"))
		assert.Contains(t, h.stdout.String(), "no commits to replay")
	})

	t.Run("not without a terminal", func(t *testing.T) {
		h.interactive = false
		assert.Equal(t, cli.ExitError, h.run("rebase", "-i", "main"))
	})
}

func TestStashCommands(t *testing.T) {
	h, repo := inRepo(t)
	repo.WriteFile("a.txt", "work\n")

	require.Equal(t, cli.ExitOK, h.run("stash", "save", "-m", "wip"))
	assert.Equal(t, "one\n", repo.ReadFile("a.txt"))
	require.Equal(t, cli.ExitOK, h.run("stash"))
	assert.Contains(t, h.stdout.String(), "stash@{0}")
	assert.Contains(t, h.stdout.String(), "wip")

	require.Equal(t, cli.ExitOK, h.run("stash", "show"))
	assert.Contains(t, h.stdout.String(), "+work")

	require.Equal(t, cli.ExitOK, h.run("stash", "apply"))
	assert.Equal(t, "work\n", repo.ReadFile("a.txt"))

	assert.Equal(t, cli.ExitBlocked, h.run("stash", "drop", "--yes"), "dropping is dangerous")
	h.answer("yes")
	require.Equal(t, cli.ExitOK, h.run("stash", "drop"))
	h.interactive = false
	require.Equal(t, cli.ExitOK, h.run("stash", "list", "--json"))
	assert.Equal(t, "[]\n", h.stdout.String())

	assert.Equal(t, cli.ExitError, h.run("stash", "apply", "x"))
}

func TestTagCommands(t *testing.T) {
	h, repo := inRepo(t)
	origin := withRemote(t, repo)

	require.Equal(t, cli.ExitOK, h.run("tag", "new", "v1.0", "-m", "first release"))
	require.Equal(t, cli.ExitOK, h.run("tag"))
	assert.Contains(t, h.stdout.String(), "v1.0")
	assert.Contains(t, h.stdout.String(), "first release")

	require.Equal(t, cli.ExitOK, h.run("tag", "push", "v1.0"))
	assert.NotEmpty(t, origin.Git("rev-parse", "v1.0"))

	require.Equal(t, cli.ExitOK, h.run("tag", "delete", "v1.0"))
	assert.Equal(t, cli.ExitError, h.run("tag", "delete", "v1.0"))
}

func TestCherryPickRevertReflogRecoverBlame(t *testing.T) {
	h, repo := inRepo(t)
	repo.Git("switch", "--quiet", "-c", "side")
	repo.WriteFile("s.txt", "side\n")
	repo.Commit("side commit")
	repo.Git("switch", "--quiet", "work")

	require.Equal(t, cli.ExitOK, h.run("cherry-pick", "side", "--yes"))
	assert.Equal(t, "side commit", repo.Git("log", "-1", "--format=%s"))
	require.Equal(t, cli.ExitOK, h.run("revert", "HEAD"))
	assert.Equal(t, `Revert "side commit"`, repo.Git("log", "-1", "--format=%s"))

	lost := repo.Git("rev-parse", "HEAD")
	repo.Git("reset", "--quiet", "--hard", "HEAD~2")
	require.Equal(t, cli.ExitOK, h.run("reflog", "-n", "5"))
	assert.Contains(t, h.stdout.String(), lost[:7])
	require.Equal(t, cli.ExitOK, h.run("recover", lost, "saved"))
	assert.Equal(t, lost, repo.Git("rev-parse", "saved"))

	require.Equal(t, cli.ExitOK, h.run("blame", "a.txt"))
	assert.Contains(t, h.stdout.String(), "one")
}

func TestHunkStagingFromTheCommandLine(t *testing.T) {
	h, repo := inRepo(t)
	var b strings.Builder
	for i := range 20 {
		b.WriteString("line\n")
		_ = i
	}
	repo.WriteFile("f.txt", b.String())
	repo.Commit("f")
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	lines[0], lines[19] = "TOP", "BOTTOM"
	repo.WriteFile("f.txt", strings.Join(lines, "\n")+"\n")

	h.interactive = true
	h.pickWith(func(items []tui.Item) []string {
		require.Len(t, items, 2)
		assert.Contains(t, items[1].Label, "+BOTTOM")
		return []string{items[1].Value}
	})
	require.Equal(t, cli.ExitOK, h.run("add", "-p"))
	staged := repo.Git("diff", "--cached")
	assert.Contains(t, staged, "+BOTTOM")
	assert.NotContains(t, staged, "+TOP")

	h.pickWith(func(items []tui.Item) []string { return []string{items[0].Value} })
	require.Equal(t, cli.ExitOK, h.run("unstage", "-p"))
	assert.Empty(t, repo.Git("diff", "--cached"))
}

func TestRemoteSubmoduleIdentity(t *testing.T) {
	h, repo := inRepo(t)
	require.Equal(t, cli.ExitOK, h.run("remote", "add", "origin", "https://user:tok3n@example.com/x.git"))
	require.Equal(t, cli.ExitOK, h.run("remote"))
	assert.Contains(t, h.stdout.String(), "example.com/x.git")
	assert.NotContains(t, h.stdout.String(), "tok3n")
	require.Equal(t, cli.ExitOK, h.run("remote", "rename", "origin", "up"))
	require.Equal(t, cli.ExitOK, h.run("remote", "set-url", "up", "https://example.com/y.git", "--push"))
	require.Equal(t, cli.ExitOK, h.run("remote", "remove", "up"))

	require.Equal(t, cli.ExitOK, h.run("submodule"))
	assert.Contains(t, h.stdout.String(), "No submodules")

	require.Equal(t, cli.ExitOK, h.run("identity", "--name", "Ayşe", "--json"))
	var id git.Identity
	require.NoError(t, json.Unmarshal(h.stdout.Bytes(), &id))
	assert.Equal(t, "Ayşe", id.Name)
	assert.Equal(t, "Ayşe", repo.Git("config", "user.name"))
}

func TestIgnoreTemplates(t *testing.T) {
	h, repo := inRepo(t)
	require.Equal(t, cli.ExitOK, h.run("ignore", "--templates"))
	assert.Contains(t, h.stdout.String(), "flutter")
	require.Equal(t, cli.ExitOK, h.run("ignore", "-t", "go,macos", "secret.txt"))
	content := repo.ReadFile(".gitignore")
	assert.Contains(t, content, "*.exe")
	assert.Contains(t, content, ".DS_Store")
	assert.Contains(t, content, "/secret.txt")
	assert.Equal(t, cli.ExitError, h.run("ignore", "-t", "cobol"))
}

func TestWorkspace(t *testing.T) {
	h, repo := inRepo(t)
	other := gittest.NewRepo(t)
	other.WriteFile("x.txt", "x\n")
	other.Commit("x")
	other.WriteFile("dirty.txt", "d\n")

	require.Equal(t, cli.ExitOK, h.run("workspace"))
	assert.Contains(t, h.stdout.String(), "empty")

	require.Equal(t, cli.ExitOK, h.run("workspace", "add"))
	require.Equal(t, cli.ExitOK, h.run("workspace", "add", other.Dir))
	require.Equal(t, cli.ExitOK, h.run("workspace", "add", other.Dir), "adding twice is harmless")
	require.Equal(t, cli.ExitOK, h.run("workspace", "list"))
	assert.Equal(t, repo.Dir+"\n"+other.Dir+"\n", h.stdout.String())

	require.Equal(t, cli.ExitOK, h.run("workspace", "status"))
	assert.Contains(t, h.stdout.String(), "work · clean")
	assert.Contains(t, h.stdout.String(), "1 change(s)")

	require.Equal(t, cli.ExitOK, h.run("workspace", "fetch"))
	assert.Equal(t, cli.ExitError, h.run("workspace", "pull"), "a repository without an upstream fails, the rest go on")
	assert.Contains(t, h.stdout.String(), "upstream")

	require.Equal(t, cli.ExitOK, h.run("workspace", "remove", other.Dir))
	require.Equal(t, cli.ExitOK, h.run("workspace", "list"))
	assert.Equal(t, repo.Dir+"\n", h.stdout.String())
}
