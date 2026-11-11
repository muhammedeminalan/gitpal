package cli_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/cli"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
	"github.com/muhammedeminalan/gitpal/internal/tui"
)

// inRepo returns a harness working in a fresh repository with one commit on
// the branch "work", speaking English.
func inRepo(t *testing.T) (*harness, *gittest.Repo) {
	t.Helper()
	h := newHarness(t)
	h.writeConfig("language: en\n")
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "one\n")
	repo.Commit("first")
	repo.Git("switch", "--quiet", "-c", "work")
	h.cwd = repo.Dir
	return h, repo
}

// withRemote gives the harness's repository a bare origin with "work" pushed.
func withRemote(t *testing.T, repo *gittest.Repo) *gittest.Repo {
	t.Helper()
	origin := gittest.NewBareRepo(t)
	repo.Git("remote", "add", "origin", origin.Dir)
	repo.Git("push", "--quiet", "--set-upstream", "origin", "work")
	return origin
}

func (h *harness) answer(lines ...string) {
	h.interactive = true
	h.env.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
}

func (h *harness) pickWith(fn func(items []tui.Item) []string) {
	h.env.Pick = func(_ context.Context, opts tui.PickOptions) ([]string, error) { return fn(opts.Items), nil }
}

func TestStatus(t *testing.T) {
	h, repo := inRepo(t)
	repo.WriteFile("a.txt", "two\n")
	repo.WriteFile("new.txt", "x\n")
	repo.WriteFile("staged.txt", "x\n")
	repo.Git("add", "staged.txt")

	require.Equal(t, cli.ExitOK, h.run("status"))
	out := h.stdout.String()
	assert.Contains(t, out, "Branch: work")
	assert.Contains(t, out, "No upstream")
	assert.Contains(t, out, "Staged for commit:\n  A  staged.txt")
	assert.Contains(t, out, "Changed, not staged:\n  M  a.txt")
	assert.Contains(t, out, "New files (untracked):\n  ?  new.txt")

	require.Equal(t, cli.ExitOK, h.run("status", "--json"))
	var st struct {
		Branch string `json:"branch"`
		Clean  bool   `json:"clean"`
		Files  []struct {
			Path   string `json:"path"`
			Staged bool   `json:"staged"`
		} `json:"files"`
	}
	require.NoError(t, json.Unmarshal(h.stdout.Bytes(), &st))
	assert.Equal(t, "work", st.Branch)
	assert.False(t, st.Clean)
	assert.Len(t, st.Files, 3)

	t.Run("outside a repository", func(t *testing.T) {
		h := newHarness(t)
		assert.Equal(t, cli.ExitError, h.run("status"))
		assert.Contains(t, h.stderr.String(), "not a git repository")
	})

	t.Run("--repo points elsewhere", func(t *testing.T) {
		h2 := newHarness(t)
		h2.writeConfig("language: en\n")
		require.Equal(t, cli.ExitOK, h2.run("status", "--repo", repo.Dir))
		assert.Contains(t, h2.stdout.String(), "Branch: work")
	})
}

func TestAddUnstageDiscard(t *testing.T) {
	h, repo := inRepo(t)
	repo.WriteFile("a.txt", "precious\n")
	repo.WriteFile("b.txt", "new\n")

	require.Equal(t, cli.ExitOK, h.run("add", "a.txt"))
	assert.Equal(t, "M  a.txt\n?? b.txt", repo.Git("status", "--porcelain=v1"))

	require.Equal(t, cli.ExitOK, h.run("unstage"))
	assert.Equal(t, " M a.txt\n?? b.txt", repo.Git("status", "--porcelain=v1"))

	t.Run("add without paths needs a person or --all", func(t *testing.T) {
		assert.Equal(t, cli.ExitError, h.run("add"))
		require.Equal(t, cli.ExitOK, h.run("add", "--all"))
		require.Equal(t, cli.ExitOK, h.run("unstage"))
	})

	t.Run("add with the picker", func(t *testing.T) {
		h.interactive = true
		h.pickWith(func(items []tui.Item) []string {
			require.Len(t, items, 2)
			return []string{"b.txt"}
		})
		require.Equal(t, cli.ExitOK, h.run("add"))
		assert.Equal(t, " M a.txt\nA  b.txt", repo.Git("status", "--porcelain=v1"))
		h.interactive = false
	})

	t.Run("discard without a person is refused", func(t *testing.T) {
		code := h.run("discard", "a.txt")
		assert.Equal(t, cli.ExitBlocked, code)
		assert.Equal(t, "precious\n", repo.ReadFile("a.txt"))
		assert.Contains(t, h.stderr.String(), "explicit confirmation")
	})

	t.Run("--yes is not enough", func(t *testing.T) {
		assert.Equal(t, cli.ExitBlocked, h.run("discard", "a.txt", "--yes"))
		assert.Equal(t, "precious\n", repo.ReadFile("a.txt"))
	})

	t.Run("a dry run says what would happen", func(t *testing.T) {
		require.Equal(t, cli.ExitOK, h.run("discard", "a.txt", "--dry-run"))
		assert.Contains(t, h.stdout.String(), "Risk: dangerous")
		assert.Contains(t, h.stdout.String(), "uncommitted changes in 1 file(s) are thrown away: a.txt")
		assert.Contains(t, h.stdout.String(), "Dry run")
		assert.Equal(t, "precious\n", repo.ReadFile("a.txt"))
	})

	t.Run("a short yes is not enough for a dangerous operation", func(t *testing.T) {
		h.answer("y")
		assert.Equal(t, cli.ExitCanceled, h.run("discard", "a.txt"))
		assert.Equal(t, "precious\n", repo.ReadFile("a.txt"))
	})

	t.Run("confirmed, and undone", func(t *testing.T) {
		h.answer("yes")
		require.Equal(t, cli.ExitOK, h.run("discard", "a.txt"))
		assert.Equal(t, "one\n", repo.ReadFile("a.txt"))
		assert.Contains(t, h.stderr.String(), "Backup taken: refs/gitpal/backup-worktree/")

		h.answer("y")
		require.Equal(t, cli.ExitOK, h.run("undo"))
		assert.Equal(t, "precious\n", repo.ReadFile("a.txt"))
		assert.Contains(t, h.stdout.String(), "Undone: discard")
	})

	t.Run("ignore", func(t *testing.T) {
		require.Equal(t, cli.ExitOK, h.run("ignore", "build/"))
		assert.Equal(t, "/build/\n", repo.ReadFile(".gitignore"))
	})
}

func TestCommit(t *testing.T) {
	h, repo := inRepo(t)
	repo.WriteFile("a.txt", "two\n")
	repo.Git("add", "a.txt")

	require.Equal(t, cli.ExitOK, h.run("commit", "-m", "feat: subject", "-m", "body paragraph"))
	assert.Equal(t, "feat: subject\n\nbody paragraph", repo.Git("log", "-1", "--format=%B"))
	assert.Contains(t, h.stdout.String(), "Committed:")

	t.Run("nothing staged", func(t *testing.T) {
		assert.Equal(t, cli.ExitError, h.run("commit", "-m", "x"))
		assert.Contains(t, h.stderr.String(), "nothing to commit")
	})

	t.Run("no message and nobody to ask", func(t *testing.T) {
		repo.WriteFile("a.txt", "three\n")
		repo.Git("add", "a.txt")
		assert.Equal(t, cli.ExitError, h.run("commit"))
		assert.Contains(t, h.stderr.String(), "-m")
	})

	t.Run("the message typed in", func(t *testing.T) {
		h.answer("", "typed message")
		require.Equal(t, cli.ExitOK, h.run("commit"))
		assert.Equal(t, "typed message", repo.Git("log", "-1", "--format=%s"))
	})

	t.Run("on a protected branch: shown, and --yes covers it", func(t *testing.T) {
		h.interactive = false
		repo.Git("switch", "--quiet", "main")
		repo.WriteFile("a.txt", "on main\n")
		repo.Git("add", "a.txt")

		assert.Equal(t, cli.ExitBlocked, h.run("commit", "-m", "x"))
		require.Equal(t, cli.ExitOK, h.run("commit", "-m", "x", "--yes"))
		assert.Contains(t, h.stderr.String(), "protected branch `main`")
	})

	t.Run("a secret is warned about without repeating it", func(t *testing.T) {
		token := "ghp" + "_" + strings.Repeat("a1B2", 9)
		repo.Git("switch", "--quiet", "work")
		repo.WriteFile("conf.go", "const t = \""+token+"\"\n")
		repo.Git("add", "conf.go")
		h.answer("n")

		assert.Equal(t, cli.ExitCanceled, h.run("commit", "-m", "leak"))
		assert.Contains(t, h.stderr.String(), "conf.go, line 1: looks like a secret (GitHub token)")
		assert.NotContains(t, h.stderr.String()+h.stdout.String(), token)
	})

	t.Run("amend keeps the message", func(t *testing.T) {
		repo.Git("reset", "--quiet")
		repo.Git("checkout", "--quiet", "--", ".")
		h.answer("y")
		require.Equal(t, cli.ExitOK, h.run("commit", "--amend"))
		assert.Contains(t, h.stdout.String(), "typed message")
	})
}

func TestSaveCommitsAndPushes(t *testing.T) {
	h, repo := inRepo(t)
	origin := withRemote(t, repo)
	repo.WriteFile("a.txt", "two\n")
	repo.WriteFile("new.txt", "new\n")
	repo.WriteFile("other.txt", "not this one\n")

	h.interactive = true
	h.pickWith(func(items []tui.Item) []string {
		for _, item := range items {
			assert.True(t, item.Selected, "everything is chosen by default")
		}
		return []string{"a.txt", "new.txt"}
	})
	h.env.Stdin = strings.NewReader("feat: saved\n")

	require.Equal(t, cli.ExitOK, h.run("save"), h.stderr.String())

	assert.Equal(t, "feat: saved", repo.Git("log", "-1", "--format=%s"))
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), origin.Git("rev-parse", "refs/heads/work"))
	assert.Equal(t, "?? other.txt", repo.Git("status", "--porcelain=v1"))
	assert.Contains(t, h.stdout.String(), "Saved: 2 file(s)")

	t.Run("nothing to save", func(t *testing.T) {
		repo.Git("add", "other.txt")
		repo.Git("commit", "--quiet", "-m", "other")
		h.interactive = false
		require.Equal(t, cli.ExitOK, h.run("save", "-m", "x"))
		assert.Contains(t, h.stdout.String(), "nothing to save")
	})
}

func TestSaveSetsTheUpstreamOfANewBranch(t *testing.T) {
	h, repo := inRepo(t)
	origin := withRemote(t, repo)
	repo.Git("switch", "--quiet", "-c", "feat/new")
	repo.WriteFile("f.txt", "f\n")

	require.Equal(t, cli.ExitOK, h.run("save", "--all", "-m", "feat: new"), h.stderr.String())

	assert.Equal(t, repo.Git("rev-parse", "HEAD"), origin.Git("rev-parse", "refs/heads/feat/new"))
	assert.Equal(t, "origin/feat/new", repo.Git("rev-parse", "--abbrev-ref", "@{upstream}"))
}

func TestPushPullFetch(t *testing.T) {
	h, repo := inRepo(t)
	origin := withRemote(t, repo)
	other := gittest.Clone(t, origin)
	other.Git("switch", "--quiet", "work")

	repo.WriteFile("a.txt", "two\n")
	repo.Commit("second")
	require.Equal(t, cli.ExitOK, h.run("push"))
	assert.Contains(t, h.stdout.String(), "Pushed: work → origin/work")

	require.Equal(t, cli.ExitOK, h.run("push"))
	assert.Contains(t, h.stdout.String(), "already up to date")

	other.Git("pull", "--quiet")
	other.WriteFile("b.txt", "theirs\n")
	other.Commit("theirs")
	other.Git("push", "--quiet")

	require.Equal(t, cli.ExitOK, h.run("fetch"))
	repo.WriteFile("c.txt", "mine\n")
	repo.Commit("mine")
	assert.Equal(t, cli.ExitBlocked, h.run("push"), "behind the remote")
	assert.Contains(t, h.stderr.String(), "Pull first")

	assert.Equal(t, cli.ExitError, h.run("pull"), "ff-only by default, diverged")
	require.Equal(t, cli.ExitOK, h.run("pull", "--rebase", "--yes"), h.stderr.String())
	require.Equal(t, cli.ExitOK, h.run("push"))

	t.Run("force push to a protected branch is blocked", func(t *testing.T) {
		repo.Git("switch", "--quiet", "main")
		repo.Git("push", "--quiet", "--set-upstream", "origin", "main")
		repo.Git("commit", "--quiet", "--amend", "-m", "rewritten")
		h.answer("main")
		assert.Equal(t, cli.ExitBlocked, h.run("push", "--force"))
		assert.Contains(t, h.stderr.String(), "protected branch")
	})
}

func TestBranchCommands(t *testing.T) {
	h, repo := inRepo(t)

	require.Equal(t, cli.ExitOK, h.run("branch", "new", "feat/x"))
	assert.Equal(t, "feat/x", repo.Git("branch", "--show-current"))

	assert.Equal(t, cli.ExitError, h.run("branch", "new", "Yeni Özellik"))
	assert.Contains(t, h.stderr.String(), "Suggestion: yeni-ozellik")

	require.Equal(t, cli.ExitOK, h.run("branch", "switch", "work"))
	require.Equal(t, cli.ExitOK, h.run("branch"))
	assert.Contains(t, h.stdout.String(), "* work")
	assert.Contains(t, h.stdout.String(), "feat/x")

	require.Equal(t, cli.ExitOK, h.run("branch", "rename", "feat/x", "feat/y"))
	require.Equal(t, cli.ExitOK, h.run("branch", "delete", "feat/y", "--yes"))
	assert.NotContains(t, repo.Git("branch"), "feat/y")

	t.Run("clean deletes merged branches but never protected ones", func(t *testing.T) {
		repo.Git("branch", "done-1")
		repo.Git("branch", "done-2")
		h.interactive = false
		require.Equal(t, cli.ExitOK, h.run("branch", "clean", "--yes"))
		assert.Equal(t, "main\nwork", repo.Git("for-each-ref", "--format=%(refname:short)", "refs/heads/"))
	})

	t.Run("json", func(t *testing.T) {
		require.Equal(t, cli.ExitOK, h.run("branch", "--json"))
		var branches []struct{ Name string }
		require.NoError(t, json.Unmarshal(h.stdout.Bytes(), &branches))
		assert.Len(t, branches, 2)
	})
}

func TestLog(t *testing.T) {
	h, repo := inRepo(t)
	repo.WriteFile("a.txt", "two\n")
	repo.Commit("feat: second")

	require.Equal(t, cli.ExitOK, h.run("log", "--graph"))
	lines := strings.Split(strings.TrimSpace(h.stdout.String()), "\n")
	require.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[0], "* "))
	assert.Contains(t, lines[0], "feat: second")
	assert.Contains(t, lines[0], "HEAD -> work")

	require.Equal(t, cli.ExitOK, h.run("log", "--grep", "SECOND", "--json"))
	var commits []struct{ Subject string }
	require.NoError(t, json.Unmarshal(h.stdout.Bytes(), &commits))
	require.Len(t, commits, 1)

	require.Equal(t, cli.ExitOK, h.run("log", "--", "nope.txt"))
	assert.Contains(t, h.stdout.String(), "No commits")
}

func TestUndoBackupsHistory(t *testing.T) {
	h, repo := inRepo(t)
	first := repo.Git("rev-parse", "HEAD")
	repo.WriteFile("a.txt", "two\n")
	repo.Git("add", "a.txt")
	require.Equal(t, cli.ExitOK, h.run("commit", "-m", "second"))

	assert.Equal(t, cli.ExitBlocked, h.run("undo"), "a yes is needed")
	require.Equal(t, cli.ExitOK, h.run("undo", "--dry-run"))
	assert.Contains(t, h.stderr.String(), "Will undo: commit")

	require.Equal(t, cli.ExitOK, h.run("undo", "--yes"))
	assert.Equal(t, first, repo.Git("rev-parse", "HEAD"))
	assert.Equal(t, "M  a.txt", repo.Git("status", "--porcelain=v1"))

	assert.Equal(t, cli.ExitError, h.run("undo", "--yes"))
	assert.Contains(t, h.stderr.String(), "nothing to undo")

	require.Equal(t, cli.ExitOK, h.run("history"))
	assert.Contains(t, h.stdout.String(), "undone")

	require.Equal(t, cli.ExitOK, h.run("backups"))
	assert.Contains(t, h.stdout.String(), "No backups")
	require.Equal(t, cli.ExitOK, h.run("backups", "prune"))
}

func TestConfigCommands(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitOK, h.run("config", "set", "language", "en"))
	require.Equal(t, cli.ExitOK, h.run("config", "get", "language"))
	assert.Equal(t, "en\n", h.stdout.String())

	assert.Equal(t, cli.ExitError, h.run("config", "set", "language", "de"))
	assert.Equal(t, cli.ExitError, h.run("config", "get", "nope"))
	assert.Contains(t, h.stderr.String(), "No such setting: nope")

	require.Equal(t, cli.ExitOK, h.run("config"))
	assert.Contains(t, h.stdout.String(), "safety.protected_branches = main,master,develop")

	require.Equal(t, cli.ExitOK, h.run("config", "path"))
	assert.Equal(t, h.vars["GITPAL_CONFIG"]+"\n", h.stdout.String())

	var edited string
	h.env.Editor = func(_ context.Context, path string) error { edited = path; return nil }
	require.Equal(t, cli.ExitOK, h.run("config", "edit"))
	assert.Equal(t, h.vars["GITPAL_CONFIG"], edited)
}

func TestInitAndClone(t *testing.T) {
	h := newHarness(t)
	h.writeConfig("language: en\n")

	require.Equal(t, cli.ExitOK, h.run("init", "proj"))
	dir := filepath.Join(h.cwd, "proj")
	assert.Contains(t, h.stdout.String(), dir)

	origin := gittest.NewBareRepo(t)
	seed := gittest.Clone(t, origin)
	seed.WriteFile("a.txt", "a\n")
	seed.Commit("first")
	seed.Git("push", "--quiet", "origin", "main")

	require.Equal(t, cli.ExitOK, h.run("clone", origin.Dir, "copy"), h.stderr.String())
	assert.Contains(t, h.stdout.String(), filepath.Join(h.cwd, "copy"))
}

func TestDashboardOpensForAPerson(t *testing.T) {
	h, _ := inRepo(t)
	opened := false
	h.env.Dashboard = func(_ context.Context, opts tui.DashboardOptions) error {
		opened = true
		assert.NotNil(t, opts.Guard)
		return nil
	}
	h.interactive = true

	require.Equal(t, cli.ExitOK, h.run())
	assert.True(t, opened)
}
