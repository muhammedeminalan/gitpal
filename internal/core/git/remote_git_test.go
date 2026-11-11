package git_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// remoteSetup returns a bare origin and two clones of it. The first clone has
// pushed one commit on main; the second has it too.
func remoteSetup(t *testing.T) (origin, first, second *gittest.Repo) {
	t.Helper()
	origin = gittest.NewBareRepo(t)
	first = gittest.Clone(t, origin)
	first.WriteFile("a.txt", "one\n")
	first.Commit("first")
	first.Git("push", "--quiet", "--set-upstream", "origin", "main")
	second = gittest.Clone(t, origin)
	return origin, first, second
}

// remoteAdvance commits a change to a file in repo and pushes it.
func remoteAdvance(repo *gittest.Repo, file, content string) string {
	repo.WriteFile(file, content)
	hash := repo.Commit("change " + file)
	repo.Git("push", "--quiet")
	return hash
}

func remoteHead(repo *gittest.Repo) string { return repo.Git("rev-parse", "HEAD") }

func TestRemotes(t *testing.T) {
	ctx := context.Background()

	t.Run("none", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		remotes, err := repo.Runner.Remotes(ctx, repo.Dir)
		require.NoError(t, err)
		assert.Empty(t, remotes)
	})

	t.Run("one, and a second with its own push URL", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.Git("remote", "add", "origin", "https://example.com/team/app.git")
		repo.Git("remote", "add", "my.fork/x", "git@example.com:me/app.git")
		repo.Git("remote", "set-url", "--push", "my.fork/x", "ssh://git@example.com/me/app-push.git")

		remotes, err := repo.Runner.Remotes(ctx, repo.Dir)
		require.NoError(t, err)

		assert.Equal(t, []git.Remote{
			{Name: "my.fork/x", FetchURL: "git@example.com:me/app.git", PushURL: "ssh://git@example.com/me/app-push.git"},
			{Name: "origin", FetchURL: "https://example.com/team/app.git", PushURL: "https://example.com/team/app.git"},
		}, remotes)
	})

	t.Run("credentials in a URL are never returned", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.Git("remote", "add", "origin", "https://user:s3cr3t-token@example.com/team/app.git")
		repo.Git("remote", "set-url", "--push", "origin", "https://s3cr3t-token@example.com/team/app.git")

		remotes, err := repo.Runner.Remotes(ctx, repo.Dir)
		require.NoError(t, err)

		require.Len(t, remotes, 1)
		assert.NotContains(t, remotes[0].FetchURL, "s3cr3t-token")
		assert.NotContains(t, remotes[0].PushURL, "s3cr3t-token")
		assert.Contains(t, remotes[0].FetchURL, "example.com/team/app.git")
	})

	// The URL is the one git will really use.
	t.Run("insteadOf rewriting is applied", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.Git("remote", "add", "origin", "gh:team/app.git")
		repo.Git("config", "url.https://github.com/.insteadOf", "gh:")

		remotes, err := repo.Runner.Remotes(ctx, repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, "https://github.com/team/app.git", remotes[0].FetchURL)
	})

	t.Run("outside a repository", func(t *testing.T) {
		_, err := gittest.NewRunner(t).Remotes(ctx, gittest.TempDir(t))
		assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	})
}

func TestFetch(t *testing.T) {
	ctx := context.Background()

	t.Run("updates the remote-tracking branch, not the local one", func(t *testing.T) {
		_, first, second := remoteSetup(t)
		pushed := remoteAdvance(first, "a.txt", "two\n")
		before := remoteHead(second)

		require.NoError(t, second.Runner.Fetch(ctx, second.Dir, git.FetchOptions{}))

		assert.Equal(t, pushed, second.Git("rev-parse", "origin/main"))
		assert.Equal(t, before, remoteHead(second))
	})

	t.Run("prune removes a branch deleted on the remote", func(t *testing.T) {
		_, first, second := remoteSetup(t)
		first.Git("push", "--quiet", "origin", "main:gone")
		require.NoError(t, second.Runner.Fetch(ctx, second.Dir, git.FetchOptions{Remote: "origin"}))
		require.NotEmpty(t, second.Git("rev-parse", "origin/gone"))
		first.Git("push", "--quiet", "origin", "--delete", "gone")

		require.NoError(t, second.Runner.Fetch(ctx, second.Dir, git.FetchOptions{}))
		assert.NotEmpty(t, second.Git("rev-parse", "origin/gone"), "kept without prune")

		require.NoError(t, second.Runner.Fetch(ctx, second.Dir, git.FetchOptions{Prune: true}))
		assert.NotContains(t, second.Git("branch", "--remotes"), "origin/gone")
	})

	t.Run("all remotes", func(t *testing.T) {
		origin, first, second := remoteSetup(t)
		other := gittest.NewBareRepo(t)
		first.Git("push", "--quiet", other.Dir, "main:other-main")
		second.Git("remote", "add", "other", other.Dir)
		pushed := remoteAdvance(first, "a.txt", "two\n")
		_ = origin

		require.NoError(t, second.Runner.Fetch(ctx, second.Dir, git.FetchOptions{All: true}))

		assert.Equal(t, pushed, second.Git("rev-parse", "origin/main"))
		assert.NotEmpty(t, second.Git("rev-parse", "other/other-main"))
	})

	t.Run("a remote that is not configured", func(t *testing.T) {
		_, _, second := remoteSetup(t)
		err := second.Runner.Fetch(ctx, second.Dir, git.FetchOptions{Remote: "nope"})
		assert.Equal(t, git.KindNoRemote, git.KindOf(err))
	})

	t.Run("no remote at all", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		assert.Equal(t, git.KindNoRemote, git.KindOf(repo.Runner.Fetch(ctx, repo.Dir, git.FetchOptions{})))
	})

	t.Run("a hostile remote name", func(t *testing.T) {
		_, _, second := remoteSetup(t)
		marker := filepath.Join(gittest.TempDir(t), "written-by-git")
		for _, bad := range []string{"--upload-pack=touch " + marker, "-f", "a b"} {
			err := second.Runner.Fetch(ctx, second.Dir, git.FetchOptions{Remote: bad})
			assert.Equal(t, git.ArgInvalidRemote, stageArgCode(t, err), bad)
		}
		_, err := os.Stat(marker)
		assert.True(t, os.IsNotExist(err))
	})
}

// A remote URL with a token in it fails to connect; the token must not show
// up anywhere the failure is reported.
func TestRemoteCredentialsNeverLeak(t *testing.T) {
	ctx := context.Background()
	const secret = "s3cr3t-token"
	var events []git.Event
	repo := gittest.NewRepo(t, gittest.WithGitOptions(git.WithObserver(func(e git.Event) { events = append(events, e) })))
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("first")
	// Port 1 refuses connections at once. No proxy: the request must fail
	// here, not at some proxy with an answer of its own.
	repo.Git("remote", "add", "origin", "http://user:"+secret+"@127.0.0.1:1/x.git")
	repo.Git("config", "http.proxy", "")
	events = nil

	fetchErr := repo.Runner.Fetch(ctx, repo.Dir, git.FetchOptions{})
	_, pushErr := repo.Runner.Push(ctx, repo.Dir, git.PushOptions{})
	_, pullErr := repo.Runner.Pull(ctx, repo.Dir, git.PullOptions{Remote: "origin", Branch: "main"})

	for name, err := range map[string]error{"fetch": fetchErr, "push": pushErr, "pull": pullErr} {
		require.Error(t, err, name)
		assert.Equal(t, git.KindRemoteUnreachable, git.KindOf(err), name)
		assert.NotContains(t, err.Error(), secret, name)
		var gerr *git.Error
		require.ErrorAs(t, err, &gerr, name)
		assert.NotContains(t, gerr.Localize(i18n.TR), secret, name)
		assert.NotContains(t, gerr.Localize(i18n.EN), secret, name)
	}
	require.NotEmpty(t, events)
	for _, e := range events {
		assert.NotContains(t, e.String(), secret)
		if e.Err != nil {
			assert.NotContains(t, e.Err.Error(), secret)
		}
	}
	remotes, err := repo.Runner.Remotes(ctx, repo.Dir)
	require.NoError(t, err)
	assert.NotContains(t, remotes[0].FetchURL, secret)
}

func TestPullFastForwardOnly(t *testing.T) {
	ctx := context.Background()

	t.Run("fast-forwards", func(t *testing.T) {
		_, first, second := remoteSetup(t)
		before := remoteHead(second)
		pushed := remoteAdvance(first, "a.txt", "two\n")

		res, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{})
		require.NoError(t, err)

		assert.Equal(t, git.PullResult{Before: before, After: pushed, Updated: true}, res)
		assert.Equal(t, "two\n", second.ReadFile("a.txt"))
	})

	t.Run("already up to date", func(t *testing.T) {
		_, _, second := remoteSetup(t)
		head := remoteHead(second)

		res, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{Strategy: git.PullFFOnly})
		require.NoError(t, err)

		assert.Equal(t, git.PullResult{Before: head, After: head}, res)
	})

	t.Run("diverged history is refused and nothing changes", func(t *testing.T) {
		_, first, second := remoteSetup(t)
		remoteAdvance(first, "a.txt", "two\n")
		second.WriteFile("b.txt", "local\n")
		local := second.Commit("local work")

		_, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{})

		assert.Equal(t, git.KindDiverged, git.KindOf(err))
		assert.Equal(t, local, remoteHead(second))
		assert.True(t, statusOf(t, second).Clean())
	})

	t.Run("from an explicit remote branch", func(t *testing.T) {
		_, first, second := remoteSetup(t)
		first.Git("switch", "--quiet", "-c", "feat")
		first.WriteFile("feat.txt", "feat\n")
		feat := first.Commit("feat work")
		first.Git("push", "--quiet", "--set-upstream", "origin", "feat")

		res, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{Remote: "origin", Branch: "feat"})
		require.NoError(t, err)

		assert.Equal(t, feat, res.After)
		assert.True(t, res.Updated)
	})
}

func TestPullMerge(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a merge commit without an editor", func(t *testing.T) {
		_, first, second := remoteSetup(t)
		pushed := remoteAdvance(first, "a.txt", "two\n")
		second.WriteFile("b.txt", "local\n")
		local := second.Commit("local work")

		res, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{Strategy: git.PullMerge})
		require.NoError(t, err)

		assert.True(t, res.Updated)
		assert.Equal(t, local, res.Before)
		assert.Equal(t, local+" "+pushed, second.Git("log", "-1", "--format=%P"))
		assert.True(t, statusOf(t, second).Clean())
	})

	t.Run("a conflict leaves the merge in progress", func(t *testing.T) {
		_, first, second := remoteSetup(t)
		remoteAdvance(first, "a.txt", "theirs\n")
		second.WriteFile("a.txt", "ours\n")
		second.Commit("local work")

		_, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{Strategy: git.PullMerge})

		assert.Equal(t, git.KindConflict, git.KindOf(err))
		st := statusOf(t, second)
		assert.Equal(t, git.OpMerge, st.Operation)
		assert.Len(t, st.Conflicted(), 1)
	})
}

func TestPullRebase(t *testing.T) {
	ctx := context.Background()

	t.Run("replays local commits on top", func(t *testing.T) {
		_, first, second := remoteSetup(t)
		pushed := remoteAdvance(first, "a.txt", "two\n")
		second.WriteFile("b.txt", "local\n")
		second.Commit("local work")

		res, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{Strategy: git.PullRebase})
		require.NoError(t, err)

		assert.True(t, res.Updated)
		assert.Equal(t, pushed, second.Git("log", "-1", "--format=%P"), "linear: the only parent is the remote commit")
		assert.Equal(t, "local work", second.Git("log", "-1", "--format=%s"))
	})

	t.Run("a conflict leaves the rebase in progress", func(t *testing.T) {
		_, first, second := remoteSetup(t)
		remoteAdvance(first, "a.txt", "theirs\n")
		second.WriteFile("a.txt", "ours\n")
		second.Commit("local work")

		_, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{Strategy: git.PullRebase})

		assert.Equal(t, git.KindConflict, git.KindOf(err))
		st := statusOf(t, second)
		assert.Equal(t, git.OpRebase, st.Operation)
		assert.Len(t, st.Conflicted(), 1)
	})
}

func TestPullRefusals(t *testing.T) {
	ctx := context.Background()

	t.Run("uncommitted work in the way is left untouched", func(t *testing.T) {
		for _, strategy := range []git.PullStrategy{git.PullFFOnly, git.PullMerge, git.PullRebase} {
			_, first, second := remoteSetup(t)
			remoteAdvance(first, "a.txt", "theirs\n")
			second.WriteFile("a.txt", "my uncommitted work\n")
			head := remoteHead(second)

			_, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{Strategy: strategy})

			assert.Equal(t, git.KindDirtyWorktree, git.KindOf(err), strategy)
			assert.Equal(t, "my uncommitted work\n", second.ReadFile("a.txt"), strategy)
			assert.Equal(t, head, remoteHead(second), strategy)
		}
	})

	t.Run("no upstream", func(t *testing.T) {
		_, _, second := remoteSetup(t)
		second.Git("switch", "--quiet", "-c", "local-only")

		_, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{})

		assert.Equal(t, git.KindNoUpstream, git.KindOf(err))
	})

	t.Run("detached HEAD", func(t *testing.T) {
		_, _, second := remoteSetup(t)
		second.Git("switch", "--quiet", "--detach")

		_, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{})
		assert.Equal(t, git.KindDetachedHead, git.KindOf(err))
		_, err = second.Runner.Pull(ctx, second.Dir, git.PullOptions{Remote: "origin", Branch: "main"})
		assert.Equal(t, git.KindDetachedHead, git.KindOf(err))
	})

	t.Run("a remote that is not configured", func(t *testing.T) {
		_, _, second := remoteSetup(t)
		_, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{Remote: "nope", Branch: "main"})
		assert.Equal(t, git.KindNoRemote, git.KindOf(err))
	})

	t.Run("a remote branch that does not exist", func(t *testing.T) {
		_, _, second := remoteSetup(t)
		_, err := second.Runner.Pull(ctx, second.Dir, git.PullOptions{Remote: "origin", Branch: "nope"})
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	})

	t.Run("hostile or incomplete arguments", func(t *testing.T) {
		_, _, second := remoteSetup(t)
		for _, opts := range []git.PullOptions{
			{Remote: "--force", Branch: "main"},
			{Remote: "origin"},
		} {
			_, err := second.Runner.Pull(ctx, second.Dir, opts)
			var argErr *git.ArgError
			assert.ErrorAs(t, err, &argErr, "%+v", opts)
		}
		for _, opts := range []git.PullOptions{
			{Remote: "origin", Branch: "--force"},
			{Branch: "main"},
			{Strategy: "octopus"},
		} {
			_, err := second.Runner.Pull(ctx, second.Dir, opts)
			var argErr *git.ArgError
			assert.ErrorAs(t, err, &argErr, "%+v", opts)
		}
	})
}

// The chosen strategy wins over whatever the user's configuration says, and
// nothing is stashed behind the user's back.
func TestPullIgnoresHostileConfiguration(t *testing.T) {
	ctx := context.Background()

	diverged := func(t *testing.T, config map[string]string) (*gittest.Repo, string, string) {
		_, first, second := remoteSetup(t)
		pushed := remoteAdvance(first, "a.txt", "two\n")
		second.WriteFile("b.txt", "local\n")
		local := second.Commit("local work")
		for key, value := range config {
			second.Git("config", key, value)
		}
		return second, local, pushed
	}

	t.Run("merge despite pull.rebase, pull.ff=only and merge options", func(t *testing.T) {
		repo, local, pushed := diverged(t, map[string]string{
			"pull.rebase":              "true",
			"pull.ff":                  "only",
			"branch.main.rebase":       "true",
			"branch.main.mergeOptions": "--squash --no-commit",
			"merge.ff":                 "only",
		})

		_, err := repo.Runner.Pull(ctx, repo.Dir, git.PullOptions{Strategy: git.PullMerge})
		require.NoError(t, err)

		assert.Equal(t, local+" "+pushed, repo.Git("log", "-1", "--format=%P"), "a real merge commit")
	})

	t.Run("rebase despite pull.rebase=false and pull.ff=only", func(t *testing.T) {
		repo, _, pushed := diverged(t, map[string]string{
			"pull.rebase": "false", "pull.ff": "only", "branch.main.rebase": "false",
		})

		_, err := repo.Runner.Pull(ctx, repo.Dir, git.PullOptions{Strategy: git.PullRebase})
		require.NoError(t, err)

		assert.Equal(t, pushed, repo.Git("log", "-1", "--format=%P"))
	})

	t.Run("an interactive rebase is never started", func(t *testing.T) {
		repo, _, pushed := diverged(t, map[string]string{"pull.rebase": "interactive", "sequence.editor": "false"})

		_, err := repo.Runner.Pull(ctx, repo.Dir, git.PullOptions{Strategy: git.PullRebase})
		require.NoError(t, err)

		assert.Equal(t, pushed, repo.Git("log", "-1", "--format=%P"))
	})

	t.Run("fast-forward only despite pull.rebase and pull.ff", func(t *testing.T) {
		repo, local, _ := diverged(t, map[string]string{"pull.rebase": "true", "pull.ff": "false"})

		_, err := repo.Runner.Pull(ctx, repo.Dir, git.PullOptions{Strategy: git.PullFFOnly})

		assert.Equal(t, git.KindDiverged, git.KindOf(err))
		assert.Equal(t, local, remoteHead(repo))
	})

	t.Run("autostash settings do not stash", func(t *testing.T) {
		for _, strategy := range []git.PullStrategy{git.PullMerge, git.PullRebase} {
			repo, local, _ := diverged(t, map[string]string{"rebase.autoStash": "true", "merge.autoStash": "true"})
			repo.WriteFile("a.txt", "my uncommitted work\n")

			_, err := repo.Runner.Pull(ctx, repo.Dir, git.PullOptions{Strategy: strategy})

			assert.Equal(t, git.KindDirtyWorktree, git.KindOf(err), strategy)
			assert.Equal(t, "my uncommitted work\n", repo.ReadFile("a.txt"), strategy)
			assert.Equal(t, local, remoteHead(repo), strategy)
			assert.Empty(t, repo.Git("stash", "list"), strategy)
		}
	})
}

func TestPush(t *testing.T) {
	ctx := context.Background()

	t.Run("a new branch, setting its upstream", func(t *testing.T) {
		origin, first, _ := remoteSetup(t)
		first.Git("switch", "--quiet", "-c", "feat/yeni")
		first.WriteFile("feat.txt", "feat\n")
		head := first.Commit("feat work")

		res, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{SetUpstream: true})
		require.NoError(t, err)

		assert.Equal(t, git.PushResult{
			Remote: "origin", Branch: "feat/yeni", RemoteBranch: "feat/yeni", Status: git.PushNew, After: head,
		}, res)
		assert.Equal(t, head, origin.Git("rev-parse", "refs/heads/feat/yeni"))
		upstream, ok, err := first.Runner.Upstream(ctx, first.Dir, "")
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "origin/feat/yeni", upstream)
	})

	t.Run("without SetUpstream the branch stays untracked", func(t *testing.T) {
		_, first, _ := remoteSetup(t)
		first.Git("switch", "--quiet", "-c", "feat")

		_, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{})
		require.NoError(t, err)

		_, ok, err := first.Runner.Upstream(ctx, first.Dir, "feat")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("fast-forward", func(t *testing.T) {
		origin, first, _ := remoteSetup(t)
		before := remoteHead(first)
		first.WriteFile("a.txt", "two\n")
		after := first.Commit("second")

		res, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{})
		require.NoError(t, err)

		assert.Equal(t, git.PushResult{
			Remote: "origin", Branch: "main", RemoteBranch: "main", Status: git.PushFastForward, Before: before, After: after,
		}, res)
		assert.Equal(t, after, origin.Git("rev-parse", "refs/heads/main"))
	})

	t.Run("up to date", func(t *testing.T) {
		_, first, _ := remoteSetup(t)
		head := remoteHead(first)

		res, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{})
		require.NoError(t, err)

		assert.Equal(t, git.PushUpToDate, res.Status)
		assert.Equal(t, head, res.Before)
		assert.Equal(t, head, res.After)
	})

	t.Run("a named branch that is not checked out", func(t *testing.T) {
		origin, first, _ := remoteSetup(t)
		first.Git("branch", "other")

		res, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{Branch: "other", Remote: "origin"})
		require.NoError(t, err)

		assert.Equal(t, git.PushNew, res.Status)
		assert.Equal(t, remoteHead(first), origin.Git("rev-parse", "refs/heads/other"))
	})

	t.Run("rejected when the remote has commits we lack", func(t *testing.T) {
		origin, first, second := remoteSetup(t)
		theirs := remoteAdvance(first, "a.txt", "two\n")
		second.WriteFile("b.txt", "local\n")
		second.Commit("local work")

		_, err := second.Runner.Push(ctx, second.Dir, git.PushOptions{})

		assert.Equal(t, git.KindRejected, git.KindOf(err))
		assert.Equal(t, theirs, origin.Git("rev-parse", "refs/heads/main"))

		// Fetching does not make it a fast-forward either.
		second.Git("fetch", "--quiet")
		_, err = second.Runner.Push(ctx, second.Dir, git.PushOptions{})
		assert.Equal(t, git.KindRejected, git.KindOf(err))
	})

	t.Run("a branch tracking an upstream with another name", func(t *testing.T) {
		origin, first, _ := remoteSetup(t)
		first.Git("push", "--quiet", "origin", "main:release")
		first.Git("fetch", "--quiet")
		first.Git("switch", "--quiet", "-c", "local-name", "--track", "origin/release")
		first.WriteFile("a.txt", "two\n")
		head := first.Commit("second")

		res, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{})
		require.NoError(t, err)

		assert.Equal(t, "release", res.RemoteBranch)
		assert.Equal(t, head, origin.Git("rev-parse", "refs/heads/release"))
		_, err = origin.Runner.Run(ctx, git.Cmd{Dir: origin.Dir, Args: []string{"rev-parse", "--verify", "--quiet", "refs/heads/local-name"}})
		assert.Error(t, err, "no branch with the local name appeared on the remote")
	})

	t.Run("detached HEAD", func(t *testing.T) {
		_, first, _ := remoteSetup(t)
		first.Git("switch", "--quiet", "--detach")
		_, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{})
		assert.Equal(t, git.KindDetachedHead, git.KindOf(err))
	})

	t.Run("no remote", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("a.txt", "a\n")
		repo.Commit("first")

		_, err := repo.Runner.Push(ctx, repo.Dir, git.PushOptions{})
		assert.Equal(t, git.KindNoRemote, git.KindOf(err))
		_, err = repo.Runner.Push(ctx, repo.Dir, git.PushOptions{Remote: "nope"})
		assert.Equal(t, git.KindNoRemote, git.KindOf(err))
	})

	t.Run("a branch that does not exist", func(t *testing.T) {
		_, first, _ := remoteSetup(t)
		_, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{Branch: "nope"})
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	})

	t.Run("hostile names are refused and have no effect", func(t *testing.T) {
		origin, first, _ := remoteSetup(t)
		first.WriteFile("a.txt", "two\n")
		first.Commit("second")
		before := origin.Git("rev-parse", "refs/heads/main")

		for _, opts := range []git.PushOptions{
			{Remote: "--force"},
			{Remote: "--receive-pack=touch x"},
			{Remote: "a b"},
			{Branch: "--all"},
			{Branch: "--mirror"},
			{ForceWithLease: true, ExpectedRemote: "--force"},
		} {
			_, err := first.Runner.Push(ctx, first.Dir, opts)
			var argErr *git.ArgError
			assert.ErrorAs(t, err, &argErr, "%+v", opts)
		}
		assert.Equal(t, before, origin.Git("rev-parse", "refs/heads/main"))
	})
}

func TestPushForceWithLease(t *testing.T) {
	ctx := context.Background()

	// Rewriting our own pushed commit: the remote is where we left it.
	t.Run("replaces history we pushed ourselves", func(t *testing.T) {
		origin, first, _ := remoteSetup(t)
		before := remoteHead(first)
		first.Git("commit", "--quiet", "--amend", "-m", "first, reworded")
		after := remoteHead(first)

		_, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{})
		assert.Equal(t, git.KindRejected, git.KindOf(err), "a plain push cannot rewrite")

		res, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{ForceWithLease: true})
		require.NoError(t, err)

		assert.Equal(t, git.PushForced, res.Status)
		assert.Equal(t, before, res.Before)
		assert.Equal(t, after, res.After)
		assert.Equal(t, after, origin.Git("rev-parse", "refs/heads/main"))
	})

	t.Run("refused when the remote moved and we never fetched it", func(t *testing.T) {
		origin, first, second := remoteSetup(t)
		theirs := remoteAdvance(first, "a.txt", "two\n")
		second.Git("commit", "--quiet", "--amend", "-m", "first, reworded")

		_, err := second.Runner.Push(ctx, second.Dir, git.PushOptions{ForceWithLease: true})

		assert.Equal(t, git.KindStaleLease, git.KindOf(err))
		assert.Equal(t, theirs, origin.Git("rev-parse", "refs/heads/main"))
	})

	// A background fetch must not turn "someone else pushed" into permission
	// to overwrite it.
	t.Run("refused when the remote moved and we only fetched it", func(t *testing.T) {
		origin, first, second := remoteSetup(t)
		theirs := remoteAdvance(first, "a.txt", "two\n")
		second.Git("commit", "--quiet", "--amend", "-m", "first, reworded")
		second.Git("fetch", "--quiet")

		_, err := second.Runner.Push(ctx, second.Dir, git.PushOptions{ForceWithLease: true})

		assert.Equal(t, git.KindStaleLease, git.KindOf(err))
		assert.Equal(t, theirs, origin.Git("rev-parse", "refs/heads/main"))
	})

	t.Run("an expected commit that the remote no longer has", func(t *testing.T) {
		origin, first, second := remoteSetup(t)
		expected := remoteHead(second)
		theirs := remoteAdvance(first, "a.txt", "two\n")
		second.Git("commit", "--quiet", "--amend", "-m", "first, reworded")

		_, err := second.Runner.Push(ctx, second.Dir, git.PushOptions{ForceWithLease: true, ExpectedRemote: expected})

		assert.Equal(t, git.KindStaleLease, git.KindOf(err))
		assert.Equal(t, theirs, origin.Git("rev-parse", "refs/heads/main"))
	})

	t.Run("an expected commit that matches", func(t *testing.T) {
		origin, first, second := remoteSetup(t)
		theirs := remoteAdvance(first, "a.txt", "two\n")
		second.Git("commit", "--quiet", "--amend", "-m", "first, reworded")
		after := remoteHead(second)

		res, err := second.Runner.Push(ctx, second.Dir, git.PushOptions{ForceWithLease: true, ExpectedRemote: theirs})
		require.NoError(t, err)

		assert.Equal(t, git.PushForced, res.Status)
		assert.Equal(t, after, origin.Git("rev-parse", "refs/heads/main"))
	})
}

func TestPushDeclinedByTheServer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the server-side hook needs an executable bit")
	}
	ctx := context.Background()
	origin, first, _ := remoteSetup(t)
	hook := filepath.Join(origin.Dir, "hooks", "pre-receive")
	require.NoError(t, os.MkdirAll(filepath.Dir(hook), 0o755))
	require.NoError(t, os.WriteFile(hook, []byte("#!/bin/sh\necho 'branch is protected' >&2\nexit 1\n"), 0o755))
	before := origin.Git("rev-parse", "refs/heads/main")
	first.WriteFile("a.txt", "two\n")
	first.Commit("second")

	_, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{})

	assert.Equal(t, git.KindRemoteDeclined, git.KindOf(err))
	var gerr *git.Error
	require.ErrorAs(t, err, &gerr)
	assert.Contains(t, gerr.Stderr, "branch is protected", "what the server said is there to show")
	assert.Equal(t, before, origin.Git("rev-parse", "refs/heads/main"))
}

func TestPushRejectedByALocalHook(t *testing.T) {
	ctx := context.Background()
	origin, first, _ := remoteSetup(t)
	commitHook(t, first, "pre-push", "echo 'testler geçmedi' >&2\nexit 1\n")
	before := origin.Git("rev-parse", "refs/heads/main")
	first.WriteFile("a.txt", "two\n")
	first.Commit("second")

	_, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{})

	assert.Equal(t, git.KindHookFailed, git.KindOf(err))
	var gerr *git.Error
	require.ErrorAs(t, err, &gerr)
	assert.Contains(t, gerr.Stderr, "testler geçmedi")
	assert.Equal(t, before, origin.Git("rev-parse", "refs/heads/main"))
}

// Push sends exactly one branch, whatever the user's configuration would
// push along with it.
func TestPushIgnoresHostileConfiguration(t *testing.T) {
	ctx := context.Background()
	origin, first, _ := remoteSetup(t)
	first.Git("branch", "must-stay-local")
	first.Git("push", "--quiet", "origin", "main:must-stay-old")
	first.Git("branch", "must-stay-old")
	first.WriteFile("a.txt", "two\n")
	head := first.Commit("second")
	first.Git("branch", "--force", "must-stay-old", head)
	first.Git("tag", "--annotate", "--message", "release", "v1.0")
	for key, value := range map[string]string{
		"push.default":       "matching",
		"push.followTags":    "true",
		"remote.origin.push": "refs/heads/*:refs/heads/*",
		"remote.pushDefault": "origin",
	} {
		first.Git("config", key, value)
	}

	res, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{})
	require.NoError(t, err)

	assert.Equal(t, git.PushFastForward, res.Status)
	assert.Equal(t, head, origin.Git("rev-parse", "refs/heads/main"))
	assert.NotEqual(t, head, origin.Git("rev-parse", "refs/heads/must-stay-old"), "a matching branch was not pushed")
	assert.Equal(t, "refs/heads/main\nrefs/heads/must-stay-old", origin.Git("for-each-ref", "--format=%(refname)"),
		"no other branch and no tag arrived")
}

func TestPushRemoteSelection(t *testing.T) {
	ctx := context.Background()
	origin, first, _ := remoteSetup(t)
	fork := gittest.NewBareRepo(t)
	first.Git("remote", "add", "fork", fork.Dir)
	first.WriteFile("a.txt", "two\n")
	head := first.Commit("second")

	t.Run("branch.<name>.pushRemote wins over the upstream remote", func(t *testing.T) {
		first.Git("config", "branch.main.pushRemote", "fork")

		res, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{})
		require.NoError(t, err)

		assert.Equal(t, "fork", res.Remote)
		assert.Equal(t, head, fork.Git("rev-parse", "refs/heads/main"))
		assert.NotEqual(t, head, origin.Git("rev-parse", "refs/heads/main"))
		first.Git("config", "--unset", "branch.main.pushRemote")
	})

	t.Run("an explicit remote wins over everything", func(t *testing.T) {
		first.Git("config", "remote.pushDefault", "fork")

		res, err := first.Runner.Push(ctx, first.Dir, git.PushOptions{Remote: "origin"})
		require.NoError(t, err)

		assert.Equal(t, "origin", res.Remote)
		assert.Equal(t, head, origin.Git("rev-parse", "refs/heads/main"))
	})
}

func TestUpstream(t *testing.T) {
	ctx := context.Background()
	_, first, _ := remoteSetup(t)

	t.Run("the current branch", func(t *testing.T) {
		name, ok, err := first.Runner.Upstream(ctx, first.Dir, "")
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "origin/main", name)
	})

	t.Run("a named branch without one", func(t *testing.T) {
		first.Git("branch", "--no-track", "plain")
		name, ok, err := first.Runner.Upstream(ctx, first.Dir, "plain")
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Empty(t, name)
	})

	t.Run("detached HEAD", func(t *testing.T) {
		first.Git("switch", "--quiet", "--detach")
		defer first.Git("switch", "--quiet", "main")

		_, ok, err := first.Runner.Upstream(ctx, first.Dir, "")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	// A clone of an empty repository: the branch has no commit yet, but it
	// already knows where it will pull from.
	t.Run("a branch without commits", func(t *testing.T) {
		empty := gittest.Clone(t, gittest.NewBareRepo(t))
		name, ok, err := empty.Runner.Upstream(ctx, empty.Dir, "")
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "origin/main", name)
	})

	t.Run("a hostile name", func(t *testing.T) {
		_, _, err := first.Runner.Upstream(ctx, first.Dir, "--all")
		assert.Equal(t, git.ArgInvalidBranch, stageArgCode(t, err))
	})
}

func TestAheadBehind(t *testing.T) {
	ctx := context.Background()
	_, first, second := remoteSetup(t)
	count := func(t *testing.T) (int, int) {
		t.Helper()
		ahead, behind, err := second.Runner.AheadBehind(ctx, second.Dir, "main", "origin/main")
		require.NoError(t, err)
		return ahead, behind
	}

	ahead, behind := count(t)
	assert.Equal(t, [2]int{0, 0}, [2]int{ahead, behind})

	second.WriteFile("b.txt", "1\n")
	second.Commit("local one")
	second.WriteFile("b.txt", "2\n")
	second.Commit("local two")
	ahead, behind = count(t)
	assert.Equal(t, [2]int{2, 0}, [2]int{ahead, behind})

	remoteAdvance(first, "a.txt", "two\n")
	second.Git("fetch", "--quiet")
	ahead, behind = count(t)
	assert.Equal(t, [2]int{2, 1}, [2]int{ahead, behind}, "diverged")

	second.Git("reset", "--quiet", "--hard", "HEAD~2")
	ahead, behind = count(t)
	assert.Equal(t, [2]int{0, 1}, [2]int{ahead, behind})

	_, _, err := second.Runner.AheadBehind(ctx, second.Dir, "main", "origin/nope")
	assert.Equal(t, git.KindRefNotFound, git.KindOf(err))

	for _, bad := range []string{"--all", "-n1", "a b", ""} {
		_, _, err := second.Runner.AheadBehind(ctx, second.Dir, bad, "origin/main")
		assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err), bad)
		_, _, err = second.Runner.AheadBehind(ctx, second.Dir, "main", bad)
		assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err), bad)
	}
}
