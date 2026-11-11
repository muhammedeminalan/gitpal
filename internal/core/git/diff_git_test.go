package git_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
)

func diffOf(t *testing.T, repo *gittest.Repo, opts git.DiffOptions) []git.FileDiff {
	t.Helper()
	files, err := repo.Runner.Diff(context.Background(), repo.Dir, opts)
	require.NoError(t, err)
	return files
}

func diffFile(t *testing.T, files []git.FileDiff, path string) git.FileDiff {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return f
		}
	}
	require.Failf(t, "no diff", "path %q not among %v", path, diffPaths(files))
	return git.FileDiff{}
}

func diffPaths(files []git.FileDiff) []string {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	return paths
}

// diffText renders hunk lines back into "+added" / "-deleted" form.
func diffText(f git.FileDiff) string {
	var b strings.Builder
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			b.WriteByte(byte(l.Kind))
			b.WriteString(l.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func TestDiffWorktreeAndStaged(t *testing.T) {
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "one\ntwo\nthree\n")
	repo.WriteFile("b.txt", "b\n")
	repo.Commit("first")
	repo.WriteFile("a.txt", "one\nTWO\nthree\n")
	repo.Git("add", "a.txt")
	repo.WriteFile("a.txt", "one\nTWO\nthree\nfour\n")
	repo.WriteFile("b.txt", "B\n")

	t.Run("working tree against index", func(t *testing.T) {
		files := diffOf(t, repo, git.DiffOptions{})
		assert.Equal(t, []string{"a.txt", "b.txt"}, diffPaths(files))
		assert.Equal(t, " one\n TWO\n three\n+four\n", diffText(diffFile(t, files, "a.txt")))
	})

	t.Run("index against HEAD", func(t *testing.T) {
		files := diffOf(t, repo, git.DiffOptions{Staged: true})
		require.Equal(t, []string{"a.txt"}, diffPaths(files))
		f := files[0]
		assert.Equal(t, git.ChangeModified, f.Change)
		assert.Equal(t, " one\n-two\n+TWO\n three\n", diffText(f))
		assert.Equal(t, "100644", f.OldMode)
		assert.Equal(t, "100644", f.NewMode)
	})

	t.Run("more context", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("long.txt", "1\n2\n3\n4\n5\n6\n7\n8\n9\n")
		repo.Commit("first")
		repo.WriteFile("long.txt", "1\n2\n3\n4\nfive\n6\n7\n8\n9\n")

		normal := diffOf(t, repo, git.DiffOptions{})[0]
		assert.Equal(t, " 2\n 3\n 4\n-5\n+five\n 6\n 7\n 8\n", diffText(normal))

		narrow := diffOf(t, repo, git.DiffOptions{Context: 1})[0]
		assert.Equal(t, " 4\n-5\n+five\n 6\n", diffText(narrow))
		assert.Equal(t, 4, narrow.Hunks[0].OldStart)
		assert.Equal(t, 5, narrow.Hunks[0].Lines[1].OldLine)
	})

	t.Run("nothing changed", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("a.txt", "a\n")
		repo.Commit("first")
		assert.Empty(t, diffOf(t, repo, git.DiffOptions{}))
		assert.Empty(t, diffOf(t, repo, git.DiffOptions{Staged: true}))
	})
}

func TestDiffStagedBeforeTheFirstCommit(t *testing.T) {
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "one\ntwo\n")
	repo.Git("add", "a.txt")

	files := diffOf(t, repo, git.DiffOptions{Staged: true})

	require.Len(t, files, 1)
	assert.Equal(t, git.ChangeAdded, files[0].Change)
	assert.Equal(t, "+one\n+two\n", diffText(files[0]))
}

func TestDiffChangeTypes(t *testing.T) {
	repo := gittest.NewRepo(t)
	repo.WriteFile("gone.txt", "gone\n")
	repo.WriteFile("old name.txt", "one\ntwo\nthree\nfour\nfive\nsix\n")
	repo.WriteFile("logo.bin", "\x00\x01\x02binary")
	repo.WriteFile("mode.sh", "#!/bin/sh\n")
	repo.WriteFile("noeol.txt", "first\nlast")
	repo.Commit("first")

	repo.WriteFile("added.txt", "new\n")
	repo.WriteFile("empty.txt", "")
	repo.Git("rm", "--quiet", "gone.txt")
	repo.Git("mv", "old name.txt", "new name.txt")
	repo.WriteFile("new name.txt", "one\ntwo\nthree\nfour\nfive\nSIX\n")
	repo.WriteFile("logo.bin", "\x00\x01\x02changed")
	repo.WriteFile("fresh.bin", "\x00fresh")
	repo.WriteFile("noeol.txt", "first\nLAST")
	if runtime.GOOS != "windows" {
		require.NoError(t, os.Chmod(repo.Path("mode.sh"), 0o755))
	}
	repo.Git("add", "--all")

	files := diffOf(t, repo, git.DiffOptions{Staged: true})

	added := diffFile(t, files, "added.txt")
	assert.Equal(t, git.ChangeAdded, added.Change)
	assert.Equal(t, "100644", added.NewMode)
	assert.Empty(t, added.OldMode)

	empty := diffFile(t, files, "empty.txt")
	assert.Equal(t, git.ChangeAdded, empty.Change)
	assert.Empty(t, empty.Hunks)
	assert.False(t, empty.Binary)

	gone := diffFile(t, files, "gone.txt")
	assert.Equal(t, git.ChangeDeleted, gone.Change)
	assert.Equal(t, "-gone\n", diffText(gone))

	renamed := diffFile(t, files, "new name.txt")
	assert.Equal(t, git.ChangeRenamed, renamed.Change)
	assert.Equal(t, "old name.txt", renamed.OldPath)
	assert.Greater(t, renamed.Similarity, 50)
	assert.Less(t, renamed.Similarity, 100)
	assert.Contains(t, diffText(renamed), "-six\n+SIX\n")

	binary := diffFile(t, files, "logo.bin")
	assert.True(t, binary.Binary)
	assert.Empty(t, binary.Hunks)
	assert.Equal(t, git.ChangeModified, binary.Change)

	fresh := diffFile(t, files, "fresh.bin")
	assert.True(t, fresh.Binary)
	assert.Equal(t, git.ChangeAdded, fresh.Change)

	noeol := diffFile(t, files, "noeol.txt")
	assert.Equal(t, []git.DiffLine{
		{Kind: git.LineContext, Text: "first", OldLine: 1, NewLine: 1},
		{Kind: git.LineDeleted, Text: "last", OldLine: 2},
		{Kind: git.LineNoNewline, Text: "No newline at end of file"},
		{Kind: git.LineAdded, Text: "LAST", NewLine: 2},
		{Kind: git.LineNoNewline, Text: "No newline at end of file"},
	}, noeol.Hunks[0].Lines)

	if runtime.GOOS != "windows" {
		mode := diffFile(t, files, "mode.sh")
		assert.Equal(t, git.ChangeModified, mode.Change)
		assert.Equal(t, "100644", mode.OldMode)
		assert.Equal(t, "100755", mode.NewMode)
		assert.Empty(t, mode.Hunks)
	}
}

func TestDiffAwkwardFileNames(t *testing.T) {
	names := []string{"çalışma dosyası.txt", "-dash.txt", "a b/c d.txt", "x b/y.txt", "quote'single.txt", "*.txt"}
	if runtime.GOOS != "windows" {
		names = append(names, "tab\there.txt", "new\nline.txt", `double"quote.txt`, `back\slash.txt`)
	}
	repo := gittest.NewRepo(t)
	for _, name := range names {
		repo.WriteFile(name, "before\n")
	}
	repo.WriteFile("plain.bin", "\x00")
	repo.Commit("first")
	for _, name := range names {
		repo.WriteFile(name, "after\n")
	}

	t.Run("modified", func(t *testing.T) {
		files := diffOf(t, repo, git.DiffOptions{})
		assert.ElementsMatch(t, names, diffPaths(files))
		for _, f := range files {
			assert.Equal(t, "-before\n+after\n", diffText(f), f.Path)
		}
	})

	// With no content lines to read the name from, only the header is left.
	t.Run("added empty and binary", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		for _, name := range names {
			repo.WriteFile(name, "")
			repo.WriteFile(name+".bin", "\x00")
		}
		repo.Git("add", "--all")

		files := diffOf(t, repo, git.DiffOptions{Staged: true})

		var want []string
		for _, name := range names {
			want = append(want, name, name+".bin")
		}
		assert.ElementsMatch(t, want, diffPaths(files))
	})

	t.Run("a path filter is a name, not a pattern", func(t *testing.T) {
		files := diffOf(t, repo, git.DiffOptions{Paths: []string{"*.txt"}})
		assert.Equal(t, []string{"*.txt"}, diffPaths(files))

		files = diffOf(t, repo, git.DiffOptions{Paths: []string{"a b"}})
		assert.Equal(t, []string{"a b/c d.txt"}, diffPaths(files), "a directory limits the diff to what is under it")
	})

	t.Run("a path outside the repository is refused", func(t *testing.T) {
		_, err := repo.Runner.Diff(context.Background(), repo.Dir, git.DiffOptions{Paths: []string{"../x"}})
		assert.Equal(t, git.ArgInvalidPath, stageArgCode(t, err))
	})
}

func TestDiffCommit(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "one\n")
	root := repo.Commit("root")
	repo.WriteFile("a.txt", "two\n")
	second := repo.Commit("second")
	repo.Git("switch", "--quiet", "-c", "side", root)
	repo.WriteFile("side.txt", "side\n")
	repo.Commit("side")
	repo.Git("switch", "--quiet", "main")
	repo.Git("merge", "--quiet", "--no-ff", "--no-edit", "side")
	// Uncommitted work must not leak into a commit's diff.
	repo.WriteFile("a.txt", "uncommitted\n")
	repo.Git("add", "a.txt")

	t.Run("an ordinary commit", func(t *testing.T) {
		files := diffOf(t, repo, git.DiffOptions{Commit: second})
		require.Equal(t, []string{"a.txt"}, diffPaths(files))
		assert.Equal(t, "-one\n+two\n", diffText(files[0]))
	})

	t.Run("the root commit shows everything as added", func(t *testing.T) {
		files := diffOf(t, repo, git.DiffOptions{Commit: root, Staged: true})
		require.Equal(t, []string{"a.txt"}, diffPaths(files))
		assert.Equal(t, git.ChangeAdded, files[0].Change)
		assert.Equal(t, "+one\n", diffText(files[0]))
	})

	t.Run("a merge commit is compared with its first parent", func(t *testing.T) {
		files := diffOf(t, repo, git.DiffOptions{Commit: "HEAD"})
		require.Equal(t, []string{"side.txt"}, diffPaths(files))
		assert.Equal(t, git.ChangeAdded, files[0].Change)
	})

	t.Run("limited to a path", func(t *testing.T) {
		assert.Empty(t, diffOf(t, repo, git.DiffOptions{Commit: second, Paths: []string{"side.txt"}}))
	})

	t.Run("a commit that does not exist", func(t *testing.T) {
		_, err := repo.Runner.Diff(ctx, repo.Dir, git.DiffOptions{Commit: "nope"})
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	})

	t.Run("a hostile revision is refused and has no effect", func(t *testing.T) {
		marker := filepath.Join(gittest.TempDir(t), "written-by-git")
		for _, bad := range []string{"--output=" + marker, "-O" + marker, "a b"} {
			_, err := repo.Runner.Diff(ctx, repo.Dir, git.DiffOptions{Commit: bad})
			assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err), bad)
			_, err = repo.Runner.DiffStat(ctx, repo.Dir, git.DiffOptions{Commit: bad})
			assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err), bad)
		}
		_, err := os.Stat(marker)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestDiffStat(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "one\ntwo\nthree\n")
	repo.WriteFile("old name.txt", "one\ntwo\nthree\nfour\nfive\nsix\n")
	repo.WriteFile("logo.bin", "\x00\x01")
	repo.Commit("first")
	repo.WriteFile("a.txt", "one\nTWO\nthree\nfour\n")
	repo.Git("mv", "old name.txt", "new name.txt")
	repo.WriteFile("logo.bin", "\x00\x02")
	repo.WriteFile("new.txt", "1\n2\n")
	repo.Git("add", "--all")

	stats, err := repo.Runner.DiffStat(ctx, repo.Dir, git.DiffOptions{Staged: true})
	require.NoError(t, err)

	assert.ElementsMatch(t, []git.FileStat{
		{Path: "a.txt", Added: 2, Deleted: 1},
		{Path: "new name.txt", OldPath: "old name.txt"},
		{Path: "logo.bin", Binary: true},
		{Path: "new.txt", Added: 2},
	}, stats)

	stats, err = repo.Runner.DiffStat(ctx, repo.Dir, git.DiffOptions{})
	require.NoError(t, err)
	assert.Empty(t, stats)
}

// DiffRaw must be git's own text: a patch built from it has to apply.
func TestDiffRawCanBeAppliedByGit(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	repo.WriteFile("a b.txt", "one\ntwo\nthree\n")
	repo.WriteFile("noeol.txt", "x")
	repo.Commit("first")
	repo.WriteFile("a b.txt", "one\nTWO\nthree\n")
	repo.WriteFile("noeol.txt", "y")

	raw, err := repo.Runner.DiffRaw(ctx, repo.Dir, git.DiffOptions{})
	require.NoError(t, err)

	_, err = repo.Runner.Run(ctx, git.Cmd{Dir: repo.Dir, Args: []string{"apply", "--cached", "--check"}, Stdin: raw})
	require.NoError(t, err)

	parsed, err := git.ParseDiff(raw)
	require.NoError(t, err)
	assert.Equal(t, diffOf(t, repo, git.DiffOptions{}), parsed)
}

func TestDiffLargeChangeKeepsLineNumbers(t *testing.T) {
	repo := gittest.NewRepo(t)
	var before, after strings.Builder
	for i := 1; i <= 4000; i++ {
		fmt.Fprintf(&before, "line %d\n", i)
		if i%1000 == 0 {
			fmt.Fprintf(&after, "LINE %d\n", i)
		} else {
			fmt.Fprintf(&after, "line %d\n", i)
		}
	}
	repo.WriteFile("big.txt", before.String())
	repo.Commit("first")
	repo.WriteFile("big.txt", after.String())

	f := diffOf(t, repo, git.DiffOptions{})[0]

	require.Len(t, f.Hunks, 4)
	for i, h := range f.Hunks {
		n := (i + 1) * 1000
		for _, l := range h.Lines {
			switch l.Kind {
			case git.LineDeleted:
				assert.Equal(t, n, l.OldLine)
				assert.Equal(t, fmt.Sprintf("line %d", n), l.Text)
			case git.LineAdded:
				assert.Equal(t, n, l.NewLine)
			case git.LineContext:
				assert.Equal(t, l.OldLine, l.NewLine)
				assert.Equal(t, fmt.Sprintf("line %d", l.OldLine), l.Text)
			case git.LineNoNewline:
			}
		}
	}
}

// Settings in the user's git configuration that change what `git diff`
// prints must not change what gitpal reads.
func TestDiffIgnoresHostileConfiguration(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	repo.WriteFile("src/a.txt", "one\n\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\neleven\n")
	repo.WriteFile("src/old name.txt", "one\ntwo\nthree\nfour\nfive\nsix\n")
	repo.WriteFile("src/crlf.txt", "dos\r\nline\r\n")
	repo.Commit("first")
	repo.WriteFile("src/a.txt", "ONE\n\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\nELEVEN\n")
	repo.Git("mv", "src/old name.txt", "src/new name.txt")
	repo.WriteFile("src/crlf.txt", "dos\r\nLINE\r\n")
	repo.WriteFile("top.txt", "top\n")
	repo.Git("add", "--all")

	want := diffOf(t, repo, git.DiffOptions{Staged: true})
	wantStat, err := repo.Runner.DiffStat(ctx, repo.Dir, git.DiffOptions{Staged: true})
	require.NoError(t, err)
	require.Equal(t, git.ChangeRenamed, diffFile(t, want, "src/new name.txt").Change)
	require.Equal(t, "dos\r", diffFile(t, want, "src/crlf.txt").Hunks[0].Lines[0].Text)
	require.Len(t, diffFile(t, want, "src/a.txt").Hunks, 2)

	for key, value := range map[string]string{
		"diff.noprefix":           "true",
		"diff.mnemonicPrefix":     "true",
		"diff.external":           filepath.Join(repo.Dir, "no-such-diff-tool"),
		"color.diff":              "always",
		"color.ui":                "always",
		"diff.renames":            "false",
		"diff.context":            "0",
		"diff.interHunkContext":   "20",
		"diff.suppressBlankEmpty": "true",
		"diff.relative":           "true",
		"diff.submodule":          "log",
		"diff.colorMoved":         "zebra",
		"diff.wsErrorHighlight":   "all",
		"core.abbrev":             "4",
		"core.quotepath":          "true",
	} {
		repo.Git("config", key, value)
	}

	assert.Equal(t, want, diffOf(t, repo, git.DiffOptions{Staged: true}))
	gotStat, err := repo.Runner.DiffStat(ctx, repo.Dir, git.DiffOptions{Staged: true})
	require.NoError(t, err)
	assert.Equal(t, wantStat, gotStat)
}

func TestDiffOutsideARepository(t *testing.T) {
	ctx := context.Background()
	runner := gittest.NewRunner(t)
	dir := gittest.TempDir(t)

	_, err := runner.Diff(ctx, dir, git.DiffOptions{})
	assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	_, err = runner.DiffStat(ctx, dir, git.DiffOptions{Staged: true})
	assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	_, err = runner.DiffRaw(ctx, dir, git.DiffOptions{Commit: "HEAD"})
	assert.Equal(t, git.KindNotARepo, git.KindOf(err))
}
