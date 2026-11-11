package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
)

// commitRepo returns a repository with one commit and a staged change.
func commitRepo(t *testing.T, opts ...gittest.Option) *gittest.Repo {
	t.Helper()
	repo := gittest.NewRepo(t, opts...)
	repo.WriteFile("a.txt", "one\n")
	repo.Git("add", "a.txt")
	repo.Git("-c", "user.name=Setup", "-c", "user.email=setup@example.com", "commit", "--quiet", "-m", "first")
	repo.WriteFile("a.txt", "two\n")
	repo.Git("add", "a.txt")
	return repo
}

// commitHook installs a hook script. Git for Windows runs hooks through its
// bundled sh, so the same script works everywhere.
func commitHook(t *testing.T, repo *gittest.Repo, name, script string) {
	t.Helper()
	dir := filepath.Join(repo.Dir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755))
}

func commitMessage(repo *gittest.Repo) string {
	// %B ends with the message's own newline; Git() trims the one log adds.
	return repo.Git("log", "-1", "--format=%B")
}

func commitCount(repo *gittest.Repo) string {
	return repo.Git("rev-list", "--count", "HEAD")
}

func TestCommit(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the hash of the new commit", func(t *testing.T) {
		repo := commitRepo(t)

		res, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "feat: second"})
		require.NoError(t, err)

		assert.Equal(t, repo.Git("rev-parse", "HEAD"), res.Hash)
		assert.Equal(t, "feat: second", commitMessage(repo))
		assert.Equal(t, "2", commitCount(repo))
		assert.True(t, statusOf(t, repo).Clean())
	})

	t.Run("the first commit of a repository", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("a.txt", "a\n")
		repo.Git("add", "a.txt")

		res, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "first"})
		require.NoError(t, err)

		assert.Equal(t, repo.Git("rev-parse", "HEAD"), res.Hash)
	})

	t.Run("the message is kept as written", func(t *testing.T) {
		repo := commitRepo(t)
		message := "feat(giriş): çıkış düğmesi eklendi\n\n" +
			"# Bu satır yorum değil, mesajın parçası\n" +
			"- madde bir\n- %s %H 100% tamam\n\n" +
			"    girintili satır\n" +
			"------------------------ >8 ------------------------\n" +
			"kesme çizgisinden sonrası da kalır"

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: message})
		require.NoError(t, err)

		assert.Equal(t, message, commitMessage(repo))
	})

	t.Run("surrounding blank lines are trimmed", func(t *testing.T) {
		repo := commitRepo(t)

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "\n\n  subject  \n\nbody\n\n\n"})
		require.NoError(t, err)

		assert.Equal(t, "  subject\n\nbody", commitMessage(repo))
	})

	t.Run("an empty message is refused before git runs", func(t *testing.T) {
		repo := commitRepo(t)
		for _, message := range []string{"", "   ", "\n\t\n"} {
			_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: message})
			assert.Equal(t, git.ArgEmptyMessage, stageArgCode(t, err))
		}
		assert.Equal(t, "1", commitCount(repo))
	})

	t.Run("nothing staged", func(t *testing.T) {
		repo := commitRepo(t)
		repo.Git("reset", "--quiet")

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "empty"})

		assert.Equal(t, git.KindNothingToCommit, git.KindOf(err))
		assert.Equal(t, "1", commitCount(repo))
	})

	t.Run("nothing staged in a repository without commits", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("untracked.txt", "x\n")

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "empty"})

		assert.Equal(t, git.KindNothingToCommit, git.KindOf(err))
	})

	t.Run("an empty commit when asked for", func(t *testing.T) {
		repo := commitRepo(t)
		repo.Git("reset", "--quiet")

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "chore: trigger CI", AllowEmpty: true})
		require.NoError(t, err)

		assert.Equal(t, "2", commitCount(repo))
	})

	t.Run("on a detached HEAD", func(t *testing.T) {
		repo := commitRepo(t)
		repo.Git("switch", "--quiet", "--detach")

		res, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "detached"})
		require.NoError(t, err)

		assert.Equal(t, repo.Git("rev-parse", "HEAD"), res.Hash)
	})

	t.Run("without an identity", func(t *testing.T) {
		repo := commitRepo(t, gittest.WithoutIdentity())

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "second"})

		assert.Equal(t, git.KindIdentityMissing, git.KindOf(err))
		assert.Equal(t, "1", commitCount(repo))
	})

	t.Run("with unresolved conflicts", func(t *testing.T) {
		repo := statusConflictRepo(t)
		require.Error(t, statusTryGit(repo, "merge", "other"))

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "merge"})

		assert.Equal(t, git.KindConflict, git.KindOf(err))
	})

	// Resolving a conflict in favour of our side leaves nothing staged, yet
	// the merge still has to be concluded with a commit.
	t.Run("concludes a merge that changes nothing", func(t *testing.T) {
		repo := statusConflictRepo(t)
		require.Error(t, statusTryGit(repo, "merge", "other"))
		repo.WriteFile("f.txt", "ours\n")
		repo.Git("add", "f.txt")

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "merge other, keeping ours"})
		require.NoError(t, err)

		assert.Len(t, repo.Git("rev-list", "--parents", "-1", "HEAD"), 40*3+2, "a merge commit has two parents")
	})

	t.Run("outside a repository", func(t *testing.T) {
		_, err := gittest.NewRunner(t).Commit(ctx, gittest.TempDir(t), git.CommitOptions{Message: "x"})
		assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	})
}

func TestCommitAmend(t *testing.T) {
	ctx := context.Background()

	t.Run("with a new message", func(t *testing.T) {
		repo := commitRepo(t)

		res, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "first, reworded", Amend: true})
		require.NoError(t, err)

		assert.Equal(t, repo.Git("rev-parse", "HEAD"), res.Hash)
		assert.Equal(t, "first, reworded", commitMessage(repo))
		assert.Equal(t, "1", commitCount(repo))
		assert.Equal(t, "two", repo.Git("show", "HEAD:a.txt"), "the staged change went into the amended commit")
	})

	t.Run("keeping the message", func(t *testing.T) {
		repo := commitRepo(t)

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Amend: true})
		require.NoError(t, err)

		assert.Equal(t, "first", commitMessage(repo))
		assert.Equal(t, "1", commitCount(repo))
	})

	t.Run("message only, nothing staged", func(t *testing.T) {
		repo := commitRepo(t)
		repo.Git("reset", "--quiet")

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "reworded", Amend: true})
		require.NoError(t, err)

		assert.Equal(t, "reworded", commitMessage(repo))
	})

	t.Run("a blank message is refused", func(t *testing.T) {
		repo := commitRepo(t)
		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "  \n", Amend: true})
		assert.Equal(t, git.ArgEmptyMessage, stageArgCode(t, err))
	})

	t.Run("without a commit to amend", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("a.txt", "a\n")
		repo.Git("add", "a.txt")

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "x", Amend: true})

		assert.Equal(t, git.KindUnbornBranch, git.KindOf(err))
	})
}

func TestCommitHooks(t *testing.T) {
	ctx := context.Background()

	t.Run("a pre-commit hook that rejects", func(t *testing.T) {
		repo := commitRepo(t)
		commitHook(t, repo, "pre-commit", "echo 'lint: a.txt:1 satır çok uzun'\necho 'lint: 1 hata' >&2\nexit 1\n")

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "second"})

		assert.Equal(t, git.KindHookFailed, git.KindOf(err))
		var gerr *git.Error
		require.ErrorAs(t, err, &gerr)
		assert.Contains(t, gerr.Stderr, "lint: a.txt:1 satır çok uzun", "what the hook printed is there to show")
		assert.Contains(t, gerr.Stderr, "lint: 1 hata")
		assert.Equal(t, "1", commitCount(repo))
	})

	t.Run("a commit-msg hook that rejects", func(t *testing.T) {
		repo := commitRepo(t)
		commitHook(t, repo, "commit-msg", "echo 'mesaj conventional commits biçiminde değil' >&2\nexit 1\n")

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "second"})

		assert.Equal(t, git.KindHookFailed, git.KindOf(err))
		assert.Equal(t, "1", commitCount(repo))
	})

	t.Run("hooks from core.hooksPath", func(t *testing.T) {
		repo := commitRepo(t)
		hooks := repo.MkdirAll(".githooks")
		require.NoError(t, os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nexit 1\n"), 0o755))
		repo.Git("config", "core.hooksPath", ".githooks")

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "second"})

		assert.Equal(t, git.KindHookFailed, git.KindOf(err))
	})

	t.Run("a hook that changes files and succeeds", func(t *testing.T) {
		repo := commitRepo(t)
		commitHook(t, repo, "pre-commit", "echo formatted > a.txt\ngit add a.txt\n")

		res, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "second"})
		require.NoError(t, err)

		assert.Equal(t, repo.Git("rev-parse", "HEAD"), res.Hash)
		assert.Equal(t, "formatted", repo.Git("show", "HEAD:a.txt"))
	})

	t.Run("a failure that is not the hook's keeps its own kind", func(t *testing.T) {
		repo := commitRepo(t, gittest.WithoutIdentity())
		commitHook(t, repo, "pre-commit", "exit 0\n")

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "second"})

		assert.Equal(t, git.KindIdentityMissing, git.KindOf(err))
	})
}

// What the user's configuration says about commits must not change what
// Commit does, and no editor may ever be started.
func TestCommitIgnoresHostileConfiguration(t *testing.T) {
	ctx := context.Background()

	// GIT_EDITOR=false: any attempt to open an editor fails the command.
	noEditor := gittest.WithGitOptions(git.WithEnv("GIT_EDITOR=false", "GIT_SEQUENCE_EDITOR=false"))

	t.Run("template, cleanup, verbose, comment character, editor", func(t *testing.T) {
		repo := commitRepo(t, noEditor)
		repo.WriteFile(".git/template.txt", "# Konu satırı\n\nTEMPLATE\n")
		for key, value := range map[string]string{
			"commit.template":  repo.Path(".git/template.txt"),
			"commit.cleanup":   "strip",
			"commit.verbose":   "true",
			"commit.status":    "true",
			"core.commentChar": ";",
			"core.editor":      "false",
		} {
			repo.Git("config", key, value)
		}
		message := "# Başlık\n\n; noktalı virgülle başlayan satır\n" +
			"; ------------------------ >8 ------------------------\n" +
			"kesme çizgisinden sonrası da mesajın parçası"

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: message})
		require.NoError(t, err)
		assert.Equal(t, message, commitMessage(repo))

		_, err = repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Amend: true})
		require.NoError(t, err)
		assert.Equal(t, message, commitMessage(repo))
	})

	t.Run("a message identical to the template is still a message", func(t *testing.T) {
		repo := commitRepo(t, noEditor)
		repo.WriteFile(".git/template.txt", "wip\n")
		repo.Git("config", "commit.template", repo.Path(".git/template.txt"))

		_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "wip"})
		require.NoError(t, err)
		assert.Equal(t, "wip", commitMessage(repo))
	})

	t.Run("signing", func(t *testing.T) {
		broken := func(t *testing.T) *gittest.Repo {
			repo := commitRepo(t)
			repo.Git("config", "gpg.program", filepath.Join(repo.Dir, "no-such-gpg"))
			return repo
		}

		t.Run("never: commits even when the configuration says to sign", func(t *testing.T) {
			repo := broken(t)
			repo.Git("config", "commit.gpgsign", "true")
			_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "second", Sign: git.SignNever})
			require.NoError(t, err)
		})

		t.Run("always: signing is really attempted", func(t *testing.T) {
			repo := broken(t)
			_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "second", Sign: git.SignAlways})
			require.Error(t, err)
			assert.Equal(t, "1", commitCount(repo))
		})

		t.Run("default: follows the configuration", func(t *testing.T) {
			repo := broken(t)
			_, err := repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "second"})
			require.NoError(t, err, "commit.gpgsign is off")

			repo.WriteFile("a.txt", "three\n")
			repo.Git("add", "a.txt")
			repo.Git("config", "commit.gpgsign", "true")
			_, err = repo.Runner.Commit(ctx, repo.Dir, git.CommitOptions{Message: "third"})
			require.Error(t, err, "commit.gpgsign is on and gpg is broken")
		})
	})
}

func TestHasStagedChanges(t *testing.T) {
	ctx := context.Background()

	t.Run("without commits", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		repo.WriteFile("a.txt", "a\n")

		staged, err := repo.Runner.HasStagedChanges(ctx, repo.Dir)
		require.NoError(t, err)
		assert.False(t, staged)

		repo.Git("add", "a.txt")
		staged, err = repo.Runner.HasStagedChanges(ctx, repo.Dir)
		require.NoError(t, err)
		assert.True(t, staged)
	})

	t.Run("with commits", func(t *testing.T) {
		repo := commitRepo(t)

		staged, err := repo.Runner.HasStagedChanges(ctx, repo.Dir)
		require.NoError(t, err)
		assert.True(t, staged)

		repo.Git("reset", "--quiet")
		staged, err = repo.Runner.HasStagedChanges(ctx, repo.Dir)
		require.NoError(t, err)
		assert.False(t, staged, "an unstaged change is not a staged one")
	})

	t.Run("an external diff tool is not consulted", func(t *testing.T) {
		repo := commitRepo(t)
		repo.Git("config", "diff.external", filepath.Join(repo.Dir, "no-such-tool"))

		staged, err := repo.Runner.HasStagedChanges(ctx, repo.Dir)
		require.NoError(t, err)
		assert.True(t, staged)
	})

	t.Run("outside a repository", func(t *testing.T) {
		_, err := gittest.NewRunner(t).HasStagedChanges(ctx, gittest.TempDir(t))
		assert.Equal(t, git.KindNotARepo, git.KindOf(err))
	})
}

func TestLastCommitMessage(t *testing.T) {
	ctx := context.Background()

	t.Run("the whole message", func(t *testing.T) {
		repo := commitRepo(t)
		repo.Git("config", "log.showSignature", "true")
		repo.Git("config", "format.pretty", "oneline")
		message := "feat: başlık\n\ngövde satırı\n\nCo-authored-by: Biri <biri@example.com>"
		repo.Git("commit", "--quiet", "-m", message)

		got, err := repo.Runner.LastCommitMessage(ctx, repo.Dir)
		require.NoError(t, err)
		assert.Equal(t, message, got)
	})

	t.Run("without commits", func(t *testing.T) {
		repo := gittest.NewRepo(t)
		_, err := repo.Runner.LastCommitMessage(ctx, repo.Dir)
		assert.Equal(t, git.KindUnbornBranch, git.KindOf(err))
	})
}

func TestRemoteContains(t *testing.T) {
	ctx := context.Background()
	origin := gittest.NewBareRepo(t)
	repo := gittest.Clone(t, origin)
	repo.WriteFile("a.txt", "a\n")
	pushed := repo.Commit("first")
	repo.Git("push", "--quiet", "--set-upstream", "origin", "main")
	repo.WriteFile("a.txt", "b\n")
	local := repo.Commit("second, not pushed")

	t.Run("a pushed commit", func(t *testing.T) {
		got, err := repo.Runner.RemoteContains(ctx, repo.Dir, pushed)
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("a commit that only exists locally", func(t *testing.T) {
		got, err := repo.Runner.RemoteContains(ctx, repo.Dir, local)
		require.NoError(t, err)
		assert.False(t, got)

		got, err = repo.Runner.RemoteContains(ctx, repo.Dir, "HEAD")
		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("after pushing it", func(t *testing.T) {
		repo.Git("push", "--quiet")
		got, err := repo.Runner.RemoteContains(ctx, repo.Dir, local)
		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("a revision that does not exist", func(t *testing.T) {
		_, err := repo.Runner.RemoteContains(ctx, repo.Dir, "nope")
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	})

	t.Run("a hostile revision", func(t *testing.T) {
		for _, bad := range []string{"--format=%(objectname)", "-h", "", "a b"} {
			_, err := repo.Runner.RemoteContains(ctx, repo.Dir, bad)
			assert.Equal(t, git.ArgInvalidRev, stageArgCode(t, err), bad)
		}
	})
}
