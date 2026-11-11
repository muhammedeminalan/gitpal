package git_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
)

// stageRepo returns a repository with one commit holding the given files.
func stageRepo(t *testing.T, files ...string) *gittest.Repo {
	t.Helper()
	repo := gittest.NewRepo(t)
	for _, name := range files {
		repo.WriteFile(name, "original\n")
	}
	repo.WriteFile("keep.txt", "keep\n")
	repo.Commit("first")
	return repo
}

// stageShort is `git status --short` with every untracked file listed: a
// compact picture of index and working tree for assertions.
func stageShort(repo *gittest.Repo) string {
	return repo.Git("status", "--porcelain=v1", "--untracked-files=all", "--no-renames")
}

func stageArgCode(t *testing.T, err error) git.ArgCode {
	t.Helper()
	var argErr *git.ArgError
	require.ErrorAs(t, err, &argErr)
	return argErr.Code
}

func TestStage(t *testing.T) {
	ctx := context.Background()

	t.Run("only the named file", func(t *testing.T) {
		repo := stageRepo(t, "a.txt", "b.txt")
		repo.WriteFile("a.txt", "changed\n")
		repo.WriteFile("b.txt", "changed\n")

		require.NoError(t, repo.Runner.Stage(ctx, repo.Dir, []string{"a.txt"}))

		assert.Equal(t, "M  a.txt\n M b.txt", stageShort(repo))
	})

	t.Run("a new file and a deletion", func(t *testing.T) {
		repo := stageRepo(t, "gone.txt")
		repo.Remove("gone.txt")
		repo.WriteFile("new.txt", "new\n")

		require.NoError(t, repo.Runner.Stage(ctx, repo.Dir, []string{"gone.txt", "new.txt"}))

		assert.Equal(t, "D  gone.txt\nA  new.txt", stageShort(repo))
	})

	t.Run("a directory stages everything under it", func(t *testing.T) {
		repo := stageRepo(t, "src/a.go", "src/deep/b.go", "src/gone.go", "other.txt")
		repo.WriteFile("src/a.go", "changed\n")
		repo.WriteFile("src/deep/b.go", "changed\n")
		repo.WriteFile("src/deep/new.go", "new\n")
		repo.Remove("src/gone.go")
		repo.WriteFile("other.txt", "changed\n")

		require.NoError(t, repo.Runner.Stage(ctx, repo.Dir, []string{"src"}))

		assert.Equal(t, " M other.txt\nM  src/a.go\nM  src/deep/b.go\nA  src/deep/new.go\nD  src/gone.go", stageShort(repo))
	})

	t.Run("a glob-looking name is a file name, not a pattern", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows cannot create files named with * or :")
		}
		repo := stageRepo(t)
		repo.WriteFile("*.txt", "star\n")
		repo.WriteFile("a.txt", "a\n")
		repo.WriteFile(":(exclude)x", "magic\n")

		require.NoError(t, repo.Runner.Stage(ctx, repo.Dir, []string{"*.txt", ":(exclude)x"}))

		assert.Equal(t, "A  *.txt\nA  :(exclude)x\n?? a.txt", stageShort(repo))
	})

	t.Run("awkward names", func(t *testing.T) {
		names := []string{"çalışma dosyası.txt", "-dash.txt", "--force", "quote'single.txt"}
		if runtime.GOOS != "windows" {
			names = append(names, "tab\there.txt", "new\nline.txt", `double"quote.txt`)
		}
		repo := stageRepo(t)
		for _, name := range names {
			repo.WriteFile(name, "x\n")
		}

		require.NoError(t, repo.Runner.Stage(ctx, repo.Dir, names))

		st := statusOf(t, repo)
		assert.ElementsMatch(t, names, entryPaths(st.Staged()))
		assert.Empty(t, st.Untracked())
	})

	// The list goes to git on stdin, so it is not bound by the length limit
	// of a command line (32 KiB on Windows).
	t.Run("thousands of paths", func(t *testing.T) {
		repo := stageRepo(t)
		var names []string
		for i := range 3000 {
			name := fmt.Sprintf("many/a-rather-long-file-name-to-fill-the-command-line-%04d.txt", i)
			repo.WriteFile(name, "x\n")
			names = append(names, name)
		}

		require.NoError(t, repo.Runner.Stage(ctx, repo.Dir, names))

		assert.Len(t, statusOf(t, repo).Staged(), 3000)
	})

	t.Run("an ignored file is refused", func(t *testing.T) {
		repo := stageRepo(t)
		repo.WriteFile(".gitignore", "*.log\n")
		repo.WriteFile("debug.log", "noise\n")
		repo.WriteFile("ok.txt", "ok\n")

		err := repo.Runner.Stage(ctx, repo.Dir, []string{"debug.log", "ok.txt"})

		assert.Equal(t, git.KindPathIgnored, git.KindOf(err))
		assert.NotContains(t, stageShort(repo), "A  debug.log")
	})

	t.Run("a path that does not exist", func(t *testing.T) {
		repo := stageRepo(t)

		err := repo.Runner.Stage(ctx, repo.Dir, []string{"nope.txt"})

		assert.Equal(t, git.KindPathspecNoMatch, git.KindOf(err))
	})

	t.Run("no paths", func(t *testing.T) {
		repo := stageRepo(t)
		assert.Equal(t, git.ArgNoPaths, stageArgCode(t, repo.Runner.Stage(ctx, repo.Dir, nil)))
	})

	t.Run("a path outside the repository", func(t *testing.T) {
		repo := stageRepo(t)
		for _, bad := range []string{"../outside.txt", "/etc/passwd", "a/../../b", ""} {
			assert.Equal(t, git.ArgInvalidPath, stageArgCode(t, repo.Runner.Stage(ctx, repo.Dir, []string{bad})), bad)
		}
	})

	t.Run("add.ignoreErrors does not hide a failure", func(t *testing.T) {
		repo := stageRepo(t)
		repo.Git("config", "add.ignoreErrors", "true")

		err := repo.Runner.Stage(ctx, repo.Dir, []string{"nope.txt"})

		assert.Equal(t, git.KindPathspecNoMatch, git.KindOf(err))
	})

	t.Run("outside a repository", func(t *testing.T) {
		err := gittest.NewRunner(t).Stage(ctx, gittest.TempDir(t), []string{"a.txt"})
		assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	})
}

func TestStageAll(t *testing.T) {
	ctx := context.Background()
	repo := stageRepo(t, "a.txt", "gone.txt")
	repo.WriteFile("a.txt", "changed\n")
	repo.Remove("gone.txt")
	repo.WriteFile("deep/new.txt", "new\n")

	require.NoError(t, repo.Runner.StageAll(ctx, repo.Dir))

	assert.Equal(t, "M  a.txt\nA  deep/new.txt\nD  gone.txt", stageShort(repo))
}

func TestUnstage(t *testing.T) {
	ctx := context.Background()

	t.Run("keeps the working tree content", func(t *testing.T) {
		repo := stageRepo(t, "a.txt", "b.txt")
		repo.WriteFile("a.txt", "changed\n")
		repo.WriteFile("b.txt", "changed\n")
		repo.Git("add", "--all")

		require.NoError(t, repo.Runner.Unstage(ctx, repo.Dir, []string{"a.txt"}))

		assert.Equal(t, " M a.txt\nM  b.txt", stageShort(repo))
		assert.Equal(t, "changed\n", repo.ReadFile("a.txt"))
	})

	t.Run("on a branch without commits", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("a.txt", "a\n")
		repo.WriteFile("dir/b.txt", "b\n")
		repo.Git("add", "--all")
		// Changed again after staging: git would refuse a plain `rm --cached`.
		repo.WriteFile("a.txt", "a, edited\n")

		require.NoError(t, repo.Runner.Unstage(ctx, repo.Dir, []string{"a.txt"}))

		assert.Equal(t, "A  dir/b.txt\n?? a.txt", stageShort(repo))
		assert.Equal(t, "a, edited\n", repo.ReadFile("a.txt"))
	})

	t.Run("a staged deletion", func(t *testing.T) {
		repo := stageRepo(t, "gone.txt")
		repo.Git("rm", "--quiet", "gone.txt")

		require.NoError(t, repo.Runner.Unstage(ctx, repo.Dir, []string{"gone.txt"}))

		assert.Equal(t, " D gone.txt", stageShort(repo))
	})

	t.Run("a staged rename, both sides", func(t *testing.T) {
		repo := stageRepo(t, "old.txt")
		repo.Git("mv", "old.txt", "new.txt")

		require.NoError(t, repo.Runner.Unstage(ctx, repo.Dir, []string{"old.txt", "new.txt"}))

		assert.Equal(t, " D old.txt\n?? new.txt", stageShort(repo))
		assert.Equal(t, "original\n", repo.ReadFile("new.txt"))
	})

	t.Run("a path git does not know", func(t *testing.T) {
		repo := stageRepo(t)
		err := repo.Runner.Unstage(ctx, repo.Dir, []string{"nope.txt"})
		assert.Equal(t, git.KindPathspecNoMatch, git.KindOf(err))
	})

	t.Run("no paths", func(t *testing.T) {
		repo := stageRepo(t)
		assert.Equal(t, git.ArgNoPaths, stageArgCode(t, repo.Runner.Unstage(ctx, repo.Dir, nil)))
	})
}

func TestUnstageAll(t *testing.T) {
	ctx := context.Background()

	t.Run("with commits", func(t *testing.T) {
		repo := stageRepo(t, "a.txt", "gone.txt")
		repo.WriteFile("a.txt", "changed\n")
		repo.Git("rm", "--quiet", "gone.txt")
		repo.WriteFile("new.txt", "new\n")
		repo.Git("add", "--all")

		require.NoError(t, repo.Runner.UnstageAll(ctx, repo.Dir))

		assert.Equal(t, " M a.txt\n D gone.txt\n?? new.txt", stageShort(repo))
	})

	t.Run("without commits", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("a.txt", "a\n")
		repo.Git("add", "--all")

		require.NoError(t, repo.Runner.UnstageAll(ctx, repo.Dir))

		assert.Equal(t, "?? a.txt", stageShort(repo))
	})

	t.Run("nothing staged is not an error", func(t *testing.T) {
		repo := stageRepo(t)
		require.NoError(t, repo.Runner.UnstageAll(ctx, repo.Dir))
		empty := gittest.NewRepo(t)
		require.NoError(t, empty.Runner.UnstageAll(ctx, empty.Dir))
	})

	// A plain `git reset` would also throw away the merge itself.
	t.Run("leaves a merge in progress alone", func(t *testing.T) {
		repo := statusConflictRepo(t)
		require.Error(t, statusTryGit(repo, "merge", "other"))

		require.NoError(t, repo.Runner.UnstageAll(ctx, repo.Dir))

		op, err := repo.Runner.InProgress(ctx, repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, git.OpMerge, op)
	})
}

func TestRestore(t *testing.T) {
	ctx := context.Background()

	t.Run("discards only the named file and keeps untracked files", func(t *testing.T) {
		repo := stageRepo(t, "a.txt", "b.txt")
		repo.WriteFile("a.txt", "changed\n")
		repo.WriteFile("b.txt", "changed\n")
		repo.WriteFile("untracked.txt", "mine\n")

		require.NoError(t, repo.Runner.Restore(ctx, repo.Dir, git.RestoreOptions{Paths: []string{"a.txt"}}))

		assert.Equal(t, "original\n", repo.ReadFile("a.txt"))
		assert.Equal(t, " M b.txt\n?? untracked.txt", stageShort(repo))
	})

	t.Run("brings back a deleted file", func(t *testing.T) {
		repo := stageRepo(t, "a.txt")
		repo.Remove("a.txt")

		require.NoError(t, repo.Runner.Restore(ctx, repo.Dir, git.RestoreOptions{Paths: []string{"a.txt"}, Worktree: true}))

		assert.Equal(t, "original\n", repo.ReadFile("a.txt"))
	})

	t.Run("staged only leaves the working tree alone", func(t *testing.T) {
		repo := stageRepo(t, "a.txt")
		repo.WriteFile("a.txt", "changed\n")
		repo.Git("add", "a.txt")

		require.NoError(t, repo.Runner.Restore(ctx, repo.Dir, git.RestoreOptions{Paths: []string{"a.txt"}, Staged: true}))

		assert.Equal(t, " M a.txt", stageShort(repo))
		assert.Equal(t, "changed\n", repo.ReadFile("a.txt"))
	})

	t.Run("index and working tree from another revision", func(t *testing.T) {
		repo := stageRepo(t, "a.txt")
		first := repo.Git("rev-parse", "HEAD")
		repo.WriteFile("a.txt", "second\n")
		repo.Commit("second")
		repo.WriteFile("a.txt", "third, uncommitted\n")

		require.NoError(t, repo.Runner.Restore(ctx, repo.Dir, git.RestoreOptions{
			Paths: []string{"a.txt"}, Source: first, Staged: true, Worktree: true,
		}))

		assert.Equal(t, "original\n", repo.ReadFile("a.txt"))
		assert.Equal(t, "M  a.txt", stageShort(repo))
	})

	t.Run("a hostile source is refused before git runs", func(t *testing.T) {
		repo := stageRepo(t, "a.txt")
		repo.WriteFile("a.txt", "changed\n")

		for _, bad := range []string{"--bad", "-s", "a b", "HEAD\n--staged"} {
			err := repo.Runner.Restore(ctx, repo.Dir, git.RestoreOptions{Paths: []string{"a.txt"}, Source: bad})
			assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err), bad)
		}
		assert.Equal(t, "changed\n", repo.ReadFile("a.txt"))
	})

	t.Run("a source that does not exist", func(t *testing.T) {
		repo := stageRepo(t, "a.txt")
		err := repo.Runner.Restore(ctx, repo.Dir, git.RestoreOptions{Paths: []string{"a.txt"}, Source: "nope"})
		require.Error(t, err)
		var gerr *git.Error
		assert.True(t, errors.As(err, &gerr))
	})

	t.Run("no paths", func(t *testing.T) {
		repo := stageRepo(t)
		assert.Equal(t, git.ArgNoPaths, stageArgCode(t, repo.Runner.Restore(ctx, repo.Dir, git.RestoreOptions{})))
	})

	t.Run("a path git does not know", func(t *testing.T) {
		repo := stageRepo(t)
		err := repo.Runner.Restore(ctx, repo.Dir, git.RestoreOptions{Paths: []string{"nope.txt"}})
		assert.Equal(t, git.KindPathspecNoMatch, git.KindOf(err))
	})
}

// stageIgnored reports whether git considers the path ignored. The path goes
// in on stdin, where check-ignore reads it as a plain name.
func stageIgnored(repo *gittest.Repo, path string) bool {
	_, err := repo.Runner.Run(context.Background(), git.Cmd{
		Dir: repo.Dir, Args: []string{"check-ignore", "--quiet", "--no-index", "-z", "--stdin"},
		Stdin: []byte(path + "\x00"),
	})
	return err == nil
}

func TestIgnorePaths(t *testing.T) {
	t.Run("creates .gitignore with anchored patterns", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("build/out.bin", "x")
		repo.WriteFile("sub/build/out.bin", "x")
		repo.WriteFile("notes.txt", "x")

		require.NoError(t, git.IgnorePaths(repo.Dir, []string{"build/out.bin", "notes.txt"}))

		assert.Equal(t, "/build/out.bin\n/notes.txt\n", repo.ReadFile(".gitignore"))
		assert.True(t, stageIgnored(repo, "build/out.bin"))
		assert.True(t, stageIgnored(repo, "notes.txt"))
		assert.False(t, stageIgnored(repo, "sub/build/out.bin"), "the pattern is anchored at the root")
	})

	t.Run("names with pattern characters match only themselves", func(t *testing.T) {
		names := map[string]string{ // ignored name -> a name the pattern must not catch
			"a*.txt":      "ab.txt",
			"what?.md":    "whatx.md",
			"[abc].txt":   "a.txt",
			"#hash.txt":   "hash.txt",
			"!bang.txt":   "bang.txt",
			"trailing  ":  "trailing",
			" leading":    "leading",
			"çalışma.txt": "calisma.txt",
		}
		if runtime.GOOS == "windows" {
			// Windows cannot create these names.
			for _, name := range []string{"a*.txt", "what?.md", "trailing  "} {
				delete(names, name)
			}
		} else {
			names[`back\slash`] = "backslash"
			names[`star\*`] = `star\x`
		}
		repo := gittest.NewRepo(t)
		var paths []string
		for name := range names {
			paths = append(paths, name)
		}

		require.NoError(t, git.IgnorePaths(repo.Dir, paths))

		for name, other := range names {
			assert.True(t, stageIgnored(repo, name), "%q should be ignored", name)
			assert.False(t, stageIgnored(repo, other), "%q should not be ignored because of %q", other, name)
		}
	})

	t.Run("a directory", func(t *testing.T) {
		repo := gittest.NewRepo(t)

		require.NoError(t, git.IgnorePaths(repo.Dir, []string{"node_modules/"}))

		assert.Equal(t, "/node_modules/\n", repo.ReadFile(".gitignore"))
		assert.True(t, stageIgnored(repo, "node_modules/pkg/index.js"))
	})

	t.Run("appends, keeping what is there byte for byte", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile(".gitignore", "# build output\n*.o\n\n  indented\nno newline at end")

		require.NoError(t, git.IgnorePaths(repo.Dir, []string{"a.txt"}))

		assert.Equal(t, "# build output\n*.o\n\n  indented\nno newline at end\n/a.txt\n", repo.ReadFile(".gitignore"))
	})

	t.Run("a CRLF file stays CRLF", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile(".gitignore", "*.o\r\n/old.txt\r\n")

		require.NoError(t, git.IgnorePaths(repo.Dir, []string{"a.txt", "old.txt"}))

		assert.Equal(t, "*.o\r\n/old.txt\r\n/a.txt\r\n", repo.ReadFile(".gitignore"))
		assert.True(t, stageIgnored(repo, "a.txt"))
	})

	t.Run("skips patterns already present", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile(".gitignore", "/a.txt\n")

		require.NoError(t, git.IgnorePaths(repo.Dir, []string{"a.txt", "b.txt", "b.txt", "./b.txt"}))

		assert.Equal(t, "/a.txt\n/b.txt\n", repo.ReadFile(".gitignore"))
	})

	t.Run("nothing new leaves the file untouched", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile(".gitignore", "/a.txt")

		require.NoError(t, git.IgnorePaths(repo.Dir, []string{"a.txt"}))

		assert.Equal(t, "/a.txt", repo.ReadFile(".gitignore"))
	})

	t.Run("refuses paths it cannot express or that leave the repository", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		for _, bad := range []string{"../outside", "/abs", "", ".", "new\nline", "carriage\rreturn"} {
			var argErr *git.ArgError
			require.ErrorAs(t, git.IgnorePaths(repo.Dir, []string{"ok.txt", bad}), &argErr, "%q", bad)
			assert.Equal(t, git.ArgInvalidPath, argErr.Code)
		}
		_, err := os.Stat(repo.Path(".gitignore"))
		assert.True(t, os.IsNotExist(err), "nothing is written when any path is refused")

		var argErr *git.ArgError
		require.ErrorAs(t, git.IgnorePaths(repo.Dir, nil), &argErr)
		assert.Equal(t, git.ArgNoPaths, argErr.Code)
	})

	t.Run("a directory that does not exist", func(t *testing.T) {
		err := git.IgnorePaths(gittest.TempDir(t)+"/nope", []string{"a.txt"})
		require.Error(t, err)
	})
}
