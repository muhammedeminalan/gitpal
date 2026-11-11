package git_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
)

func logOf(t *testing.T, repo *gittest.Repo, opts git.LogOptions) []git.Commit {
	t.Helper()
	commits, err := repo.Runner.Log(context.Background(), repo.Dir, opts)
	require.NoError(t, err)
	return commits
}

func logSubjects(commits []git.Commit) []string {
	subjects := make([]string, 0, len(commits))
	for _, c := range commits {
		subjects = append(subjects, c.Subject)
	}
	return subjects
}

// logCommitAs commits the given file with a chosen author and date.
func logCommitAs(t *testing.T, repo *gittest.Repo, file, message, author, date string) string {
	t.Helper()
	repo.WriteFile(file, message+"\n")
	repo.Git("add", "--all")
	env := []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}
	if author != "" {
		env = append(env,
			"GIT_AUTHOR_NAME="+author, "GIT_AUTHOR_EMAIL="+strings.ToLower(strings.Fields(author)[0])+"@example.com")
	}
	_, err := repo.Runner.Run(context.Background(), git.Cmd{
		Dir: repo.Dir, Args: []string{"commit", "--quiet", "--message", message}, Env: env,
	})
	require.NoError(t, err)
	return repo.Git("rev-parse", "HEAD")
}

// logHistory builds: three commits on main by two authors on three days, a
// side branch with one commit, merged into main.
//
//   - merge side            (2026-03-05, Test User)
//     |\
//     | * side: add feature   (2026-03-04, Zeynep Kaya)
//   - | fix(ui): [wip] kısa (2026-03-03, Zeynep Kaya)
//     |/
//   - feat: ikinci          (2026-03-02, Test User)
//   - ilk commit            (2026-03-01, Test User)
func logHistory(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.NewRepo(t)
	logCommitAs(t, repo, "a.txt", "ilk commit", "", "2026-03-01T10:00:00+03:00")
	second := logCommitAs(t, repo, "b.txt", "feat: ikinci", "", "2026-03-02T10:00:00+03:00")
	logCommitAs(t, repo, "a.txt", "fix(ui): [wip] kısa", "Zeynep Kaya", "2026-03-03T10:00:00+03:00")
	repo.Git("switch", "--quiet", "-c", "side", second)
	logCommitAs(t, repo, "side.txt", "side: add feature", "Zeynep Kaya", "2026-03-04T10:00:00+03:00")
	repo.Git("switch", "--quiet", "main")
	_, err := repo.Runner.Run(context.Background(), git.Cmd{
		Dir:  repo.Dir,
		Args: []string{"merge", "--quiet", "--no-ff", "--message", "merge side", "side"},
		Env:  []string{"GIT_AUTHOR_DATE=2026-03-05T10:00:00+03:00", "GIT_COMMITTER_DATE=2026-03-05T10:00:00+03:00"},
	})
	require.NoError(t, err)
	return repo
}

func TestLogUnbornBranch(t *testing.T) {
	repo := gittest.NewRepo(t)

	assert.Empty(t, logOf(t, repo, git.LogOptions{}))
	assert.Empty(t, logOf(t, repo, git.LogOptions{All: true}))

	messages, err := repo.Runner.RecentMessages(context.Background(), repo.Dir, 10)
	require.NoError(t, err)
	assert.Nil(t, messages)
}

func TestLogRootCommit(t *testing.T) {
	repo := gittest.NewRepo(t)
	hash := logCommitAs(t, repo, "a.txt", "ilk commit", "", "2026-03-01T10:00:00+03:00")

	commits := logOf(t, repo, git.LogOptions{})

	require.Len(t, commits, 1)
	c := commits[0]
	assert.Equal(t, hash, c.Hash)
	assert.True(t, strings.HasPrefix(hash, c.ShortHash))
	assert.Less(t, len(c.ShortHash), len(hash))
	assert.Empty(t, c.Parents)
	assert.False(t, c.IsMerge())
	assert.Equal(t, gittest.UserName, c.AuthorName)
	assert.Equal(t, gittest.UserEmail, c.AuthorEmail)
	assert.Equal(t, gittest.UserName, c.CommitterName)
	assert.Equal(t, gittest.UserEmail, c.CommitterEmail)
	assert.Equal(t, "ilk commit", c.Subject)
	assert.Empty(t, c.Body)
	assert.Equal(t, []string{"HEAD -> main"}, c.Refs)

	assert.True(t, c.AuthorDate.Equal(time.Date(2026, 3, 1, 7, 0, 0, 0, time.UTC)))
	_, offset := c.AuthorDate.Zone()
	assert.Equal(t, 3*60*60, offset, "the commit's own UTC offset is kept")
	assert.True(t, c.CommitterDate.Equal(c.AuthorDate))
}

func TestLogHistory(t *testing.T) {
	repo := logHistory(t)

	t.Run("newest first, with parents", func(t *testing.T) {
		commits := logOf(t, repo, git.LogOptions{})

		assert.Equal(t, []string{
			"merge side", "side: add feature", "fix(ui): [wip] kısa", "feat: ikinci", "ilk commit",
		}, logSubjects(commits))
		merge := commits[0]
		assert.True(t, merge.IsMerge())
		assert.Equal(t, []string{commits[2].Hash, commits[1].Hash}, merge.Parents)
		assert.Equal(t, []string{commits[3].Hash}, commits[1].Parents)
		assert.Equal(t, []string{commits[4].Hash}, commits[3].Parents)
	})

	t.Run("without merges", func(t *testing.T) {
		commits := logOf(t, repo, git.LogOptions{NoMerges: true})
		assert.Len(t, commits, 4)
		assert.NotContains(t, logSubjects(commits), "merge side")
	})

	t.Run("first parent only", func(t *testing.T) {
		assert.Equal(t, []string{"merge side", "fix(ui): [wip] kısa", "feat: ikinci", "ilk commit"},
			logSubjects(logOf(t, repo, git.LogOptions{FirstParent: true})))
	})

	t.Run("paging", func(t *testing.T) {
		assert.Equal(t, []string{"merge side", "side: add feature"}, logSubjects(logOf(t, repo, git.LogOptions{Max: 2})))
		assert.Equal(t, []string{"fix(ui): [wip] kısa", "feat: ikinci"},
			logSubjects(logOf(t, repo, git.LogOptions{Max: 2, Skip: 2})))
		assert.Equal(t, []string{"ilk commit"}, logSubjects(logOf(t, repo, git.LogOptions{Max: 2, Skip: 4})))
		assert.Empty(t, logOf(t, repo, git.LogOptions{Skip: 5}))
	})

	t.Run("by author", func(t *testing.T) {
		assert.Equal(t, []string{"side: add feature", "fix(ui): [wip] kısa"},
			logSubjects(logOf(t, repo, git.LogOptions{Author: "Zeynep"})))
		assert.Equal(t, []string{"side: add feature", "fix(ui): [wip] kısa"},
			logSubjects(logOf(t, repo, git.LogOptions{Author: "zeynep@example.com"})))
		assert.Empty(t, logOf(t, repo, git.LogOptions{Author: "zeynep kaya"}), "case matters unless asked otherwise")
		assert.Len(t, logOf(t, repo, git.LogOptions{Author: "zeynep kaya", IgnoreCase: true}), 2)
	})

	// The search text is what the user typed, not a regular expression.
	t.Run("by message, literally", func(t *testing.T) {
		assert.Equal(t, []string{"fix(ui): [wip] kısa"}, logSubjects(logOf(t, repo, git.LogOptions{Grep: "fix(ui)"})))
		assert.Equal(t, []string{"fix(ui): [wip] kısa"}, logSubjects(logOf(t, repo, git.LogOptions{Grep: "[wip]"})))
		assert.Empty(t, logOf(t, repo, git.LogOptions{Grep: "f.x"}))
		assert.Empty(t, logOf(t, repo, git.LogOptions{Grep: "FEAT"}))
		assert.Equal(t, []string{"feat: ikinci"}, logSubjects(logOf(t, repo, git.LogOptions{Grep: "FEAT:", IgnoreCase: true})))
		assert.Equal(t, []string{"side: add feature"},
			logSubjects(logOf(t, repo, git.LogOptions{Grep: "feature", Author: "Zeynep"})), "author and message together")
	})

	// git filters on the committer date.
	t.Run("by date", func(t *testing.T) {
		day := func(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.FixedZone("", 3*60*60)) }

		assert.Equal(t, []string{"merge side", "side: add feature"},
			logSubjects(logOf(t, repo, git.LogOptions{Since: day(4)})))
		assert.Equal(t, []string{"feat: ikinci", "ilk commit"}, logSubjects(logOf(t, repo, git.LogOptions{Until: day(3)})))
		assert.Equal(t, []string{"fix(ui): [wip] kısa"},
			logSubjects(logOf(t, repo, git.LogOptions{Since: day(3), Until: day(4)})))
	})

	t.Run("by path", func(t *testing.T) {
		assert.Equal(t, []string{"fix(ui): [wip] kısa", "ilk commit"},
			logSubjects(logOf(t, repo, git.LogOptions{Paths: []string{"a.txt"}})))
		_, err := repo.Runner.Log(context.Background(), repo.Dir, git.LogOptions{Paths: []string{"../x"}})
		assert.Equal(t, git.ArgInvalidPath, stageArgCode(t, err))
	})

	t.Run("ranges and several revisions", func(t *testing.T) {
		assert.Equal(t, []string{"side: add feature"}, logSubjects(logOf(t, repo, git.LogOptions{Revs: []string{"main~1..side"}})))
		assert.Equal(t, []string{"side: add feature", "feat: ikinci", "ilk commit"},
			logSubjects(logOf(t, repo, git.LogOptions{Revs: []string{"side"}})))
		assert.Equal(t, []string{"merge side", "fix(ui): [wip] kısa"},
			logSubjects(logOf(t, repo, git.LogOptions{Revs: []string{"main", "^side"}})))
	})

	t.Run("every ref", func(t *testing.T) {
		repo.Git("switch", "--quiet", "--detach", "main~1")
		defer repo.Git("switch", "--quiet", "main")

		assert.Len(t, logOf(t, repo, git.LogOptions{}), 3)
		assert.Len(t, logOf(t, repo, git.LogOptions{All: true}), 5)
	})

	t.Run("a revision that does not exist", func(t *testing.T) {
		_, err := repo.Runner.Log(context.Background(), repo.Dir, git.LogOptions{Revs: []string{"nope"}})
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	})

	t.Run("hostile revisions are refused and have no effect", func(t *testing.T) {
		marker := filepath.Join(gittest.TempDir(t), "written-by-git")
		for _, bad := range []string{"--output=" + marker, "-n1", "--all", "", "a b"} {
			_, err := repo.Runner.Log(context.Background(), repo.Dir, git.LogOptions{Revs: []string{"main", bad}})
			assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err), bad)
			_, err = repo.Runner.CommitInfo(context.Background(), repo.Dir, bad)
			assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err), bad)
		}
		_, err := os.Stat(marker)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestLogFollowsARename(t *testing.T) {
	repo := gittest.NewRepo(t)
	repo.WriteFile("old.txt", "one\ntwo\nthree\nfour\n")
	repo.Commit("add old")
	repo.Git("mv", "old.txt", "new.txt")
	repo.Commit("rename")
	repo.WriteFile("new.txt", "one\ntwo\nthree\nfour\nfive\n")
	repo.Commit("edit new")

	assert.Equal(t, []string{"edit new", "rename"},
		logSubjects(logOf(t, repo, git.LogOptions{Paths: []string{"new.txt"}})))
	assert.Equal(t, []string{"edit new", "rename", "add old"},
		logSubjects(logOf(t, repo, git.LogOptions{Paths: []string{"new.txt"}, Follow: true})))

	// log.follow would turn following on behind the caller's back.
	repo.Git("config", "log.follow", "true")
	assert.Equal(t, []string{"edit new", "rename"},
		logSubjects(logOf(t, repo, git.LogOptions{Paths: []string{"new.txt"}})))
}

func TestLogDecorations(t *testing.T) {
	origin := gittest.NewBareRepo(t)
	repo := gittest.Clone(t, origin)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("first")
	repo.Git("push", "--quiet", "--set-upstream", "origin", "main")
	repo.Git("tag", "v1.0")
	repo.Git("branch", "özellik/giriş")
	repo.WriteFile("a.txt", "b\n")
	repo.Commit("second")

	commits := logOf(t, repo, git.LogOptions{})

	assert.Equal(t, []string{"HEAD -> main"}, commits[0].Refs)
	assert.ElementsMatch(t, []string{"tag: v1.0", "origin/main", "özellik/giriş"}, commits[1].Refs)
}

func TestLogMessages(t *testing.T) {
	ctx := context.Background()
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Git("add", "--all")
	message := "feat: Türkçe karakterler çğıöşü İ %H %s 100%\n\n" +
		"ilk paragraf\ndevamı\n\n" +
		"\x1e\n\x01 kontrol karakterleri\n\n" +
		"son paragraf\n\n\n"
	_, err := repo.Runner.Run(ctx, git.Cmd{
		Dir: repo.Dir, Args: []string{"commit", "--quiet", "--cleanup=verbatim", "--file=-"}, Stdin: []byte(message),
	})
	require.NoError(t, err)

	c := logOf(t, repo, git.LogOptions{})[0]

	assert.Equal(t, "feat: Türkçe karakterler çğıöşü İ %H %s 100%", c.Subject)
	assert.Equal(t, "ilk paragraf\ndevamı\n\n\x1e\n\x01 kontrol karakterleri\n\nson paragraf", c.Body)

	info, err := repo.Runner.CommitInfo(ctx, repo.Dir, "HEAD")
	require.NoError(t, err)
	assert.Equal(t, c, info)

	messages, err := repo.Runner.RecentMessages(ctx, repo.Dir, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{strings.TrimRight(message, "\n")}, messages)
}

func TestCommitInfo(t *testing.T) {
	ctx := context.Background()
	repo := logHistory(t)

	t.Run("by name", func(t *testing.T) {
		c, err := repo.Runner.CommitInfo(ctx, repo.Dir, "side")
		require.NoError(t, err)
		assert.Equal(t, "side: add feature", c.Subject)
		assert.Equal(t, "Zeynep Kaya", c.AuthorName)
		assert.Equal(t, repo.Git("rev-parse", "side"), c.Hash)
	})

	t.Run("a merge", func(t *testing.T) {
		c, err := repo.Runner.CommitInfo(ctx, repo.Dir, "main")
		require.NoError(t, err)
		assert.True(t, c.IsMerge())
	})

	t.Run("a commit that does not exist", func(t *testing.T) {
		_, err := repo.Runner.CommitInfo(ctx, repo.Dir, "nope")
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
		_, err = repo.Runner.CommitInfo(ctx, repo.Dir, "0123456789012345678901234567890123456789")
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	})

	t.Run("outside a repository", func(t *testing.T) {
		_, err := repo.Runner.CommitInfo(ctx, gittest.TempDir(t), "HEAD")
		assert.Equal(t, git.KindNotARepo, git.KindOf(err))
		_, err = repo.Runner.Log(ctx, gittest.TempDir(t), git.LogOptions{})
		assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	})
}

func TestRecentMessages(t *testing.T) {
	ctx := context.Background()
	repo := logHistory(t)

	messages, err := repo.Runner.RecentMessages(ctx, repo.Dir, 3)
	require.NoError(t, err)
	assert.Equal(t, []string{"merge side", "side: add feature", "fix(ui): [wip] kısa"}, messages)

	messages, err = repo.Runner.RecentMessages(ctx, repo.Dir, 100)
	require.NoError(t, err)
	assert.Len(t, messages, 5, "asking for more than there is returns what there is")

	messages, err = repo.Runner.RecentMessages(ctx, repo.Dir, 0)
	require.NoError(t, err)
	assert.Empty(t, messages)
}

// Settings in the user's git configuration that change what `git log` prints
// must not change what gitpal reads.
func TestLogIgnoresHostileConfiguration(t *testing.T) {
	repo := logHistory(t)
	repo.Git("tag", "v1.0", "side")
	want := logOf(t, repo, git.LogOptions{})
	wantSearch := logOf(t, repo, git.LogOptions{Author: "Zeynep", Grep: "[wip]"})
	require.Len(t, wantSearch, 1)

	repo.WriteFile(".mailmap", "Renamed Person <renamed@example.com> Zeynep Kaya <zeynep@example.com>\n")
	for key, value := range map[string]string{
		"log.showSignature":        "true",
		"format.pretty":            "oneline",
		"log.decorate":             "full",
		"log.date":                 "relative",
		"log.abbrevCommit":         "true",
		"log.mailmap":              "true",
		"log.showRoot":             "false",
		"i18n.logOutputEncoding":   "ISO-8859-9",
		"grep.patternType":         "perl",
		"grep.extendedRegexp":      "true",
		"color.ui":                 "always",
		"notes.displayRef":         "refs/notes/*",
		"log.initialDecorationSet": "all",
	} {
		repo.Git("config", key, value)
	}
	repo.Git("notes", "add", "--message", "a note", "HEAD")

	assert.Equal(t, want, logOf(t, repo, git.LogOptions{}))
	assert.Equal(t, wantSearch, logOf(t, repo, git.LogOptions{Author: "Zeynep", Grep: "[wip]"}))
}

func TestLogLongHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a few thousand commits")
	}
	const n = 3000
	repo := gittest.NewRepo(t)
	var stream strings.Builder
	for i := 1; i <= n; i++ {
		message := fmt.Sprintf("commit %d\n\nbody of %d\n", i, i)
		fmt.Fprintf(&stream, "commit refs/heads/main\nmark :%d\n", i)
		fmt.Fprintf(&stream, "author A <a@example.com> %d +0300\n", 1700000000+i)
		fmt.Fprintf(&stream, "committer A <a@example.com> %d +0300\n", 1700000000+i)
		fmt.Fprintf(&stream, "data %d\n%s", len(message), message)
		if i > 1 {
			fmt.Fprintf(&stream, "from :%d\n", i-1)
		}
		stream.WriteString("\n")
	}
	_, err := repo.Runner.Run(context.Background(), git.Cmd{
		Dir: repo.Dir, Args: []string{"fast-import", "--quiet"}, Stdin: []byte(stream.String()),
	})
	require.NoError(t, err)

	start := time.Now()
	commits := logOf(t, repo, git.LogOptions{})

	require.Len(t, commits, n)
	assert.Equal(t, fmt.Sprintf("commit %d", n), commits[0].Subject)
	assert.Equal(t, "body of 1", commits[n-1].Body)
	assert.Equal(t, []string{commits[1].Hash}, commits[0].Parents)
	assert.Less(t, time.Since(start), 3*time.Second)
}
