package safety_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
)

// testClock starts at a fixed instant and moves one minute per reading.
type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time {
	c.now = c.now.Add(time.Minute)
	return c.now
}

type fixture struct {
	t     *testing.T
	repo  *gittest.Repo
	guard *safety.Guard
	clock *testClock
}

// newFixture returns a repository with one commit, checked out on the branch
// "work" (not protected), and a guard with the default policy.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "one\n")
	repo.WriteFile("b.txt", "one\n")
	repo.Commit("first")
	repo.Git("switch", "--quiet", "-c", "work")
	return guardFor(t, repo, safety.DefaultPolicy())
}

func guardFor(t *testing.T, repo *gittest.Repo, policy safety.Policy) *fixture {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)}
	guard, err := safety.NewGuard(context.Background(), repo.Runner, repo.Dir, policy, safety.WithClock(clock.Now))
	require.NoError(t, err)
	return &fixture{t: t, repo: repo, guard: guard, clock: clock}
}

// Approvers.
func yes(safety.Assessment) (safety.Answer, error) { return safety.Answer{Approved: true}, nil }
func no(safety.Assessment) (safety.Answer, error)  { return safety.Answer{}, nil }

func typed(text string) safety.Approver {
	return func(safety.Assessment) (safety.Answer, error) { return safety.Answer{Approved: true, Typed: text}, nil }
}

// never fails the test when the user is asked although nothing needs asking.
func never(t *testing.T) safety.Approver {
	return func(a safety.Assessment) (safety.Answer, error) {
		t.Errorf("asked for confirmation of a %s operation that needs none", a.Risk)
		return safety.Answer{}, nil
	}
}

func (f *fixture) assess(op safety.Operation) safety.Assessment {
	f.t.Helper()
	a, err := f.guard.Assess(context.Background(), op)
	require.NoError(f.t, err)
	return a
}

func (f *fixture) do(op safety.Operation, approve safety.Approver) (safety.Result, error) {
	f.t.Helper()
	return f.guard.Do(context.Background(), op, approve, safety.DoOptions{})
}

func (f *fixture) mustDo(op safety.Operation, approve safety.Approver) safety.Result {
	f.t.Helper()
	res, err := f.do(op, approve)
	require.NoError(f.t, err)
	return res
}

func (f *fixture) head() string { return f.repo.Git("rev-parse", "HEAD") }

func (f *fixture) stage(file, content string) {
	f.repo.WriteFile(file, content)
	f.repo.Git("add", file)
}

func codes(a safety.Assessment) []safety.Code {
	out := make([]safety.Code, 0, len(a.Findings))
	for _, finding := range a.Findings {
		out = append(out, finding.Code)
	}
	return out
}

func finding(t *testing.T, a safety.Assessment, code safety.Code) safety.Finding {
	t.Helper()
	for _, f := range a.Findings {
		if f.Code == code {
			return f
		}
	}
	require.Failf(t, "no finding", "%s not among %v", code, codes(a))
	return safety.Finding{}
}

func effect(t *testing.T, a safety.Assessment, kind safety.EffectKind) safety.Effect {
	t.Helper()
	for _, e := range a.Effects {
		if e.Kind == kind {
			return e
		}
	}
	require.Failf(t, "no effect", "%s not among %+v", kind, a.Effects)
	return safety.Effect{}
}

func TestCommitIsSafeAndRecorded(t *testing.T) {
	f := newFixture(t)
	before := f.head()
	f.stage("a.txt", "two\n")

	a := f.assess(safety.Commit(git.CommitOptions{Message: "second"}))
	assert.Equal(t, safety.RiskSafe, a.Risk)
	assert.Equal(t, safety.ConfirmNone, a.Confirm)
	assert.Empty(t, a.Findings)

	res := f.mustDo(safety.Commit(git.CommitOptions{Message: "second"}), never(t))

	assert.Equal(t, f.head(), res.Commit.Hash)
	entry := res.Entry
	assert.Equal(t, "commit", entry.Op)
	assert.Equal(t, "work", entry.Branch)
	assert.Equal(t, before, entry.Before)
	assert.Equal(t, f.head(), entry.After)
	assert.Empty(t, entry.Failed)

	entries, err := f.guard.History().Entries()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, entry, entries[0])
}

func TestCommitWarnings(t *testing.T) {
	t.Run("directly on a protected branch", func(t *testing.T) {
		f := newFixture(t)
		f.repo.Git("switch", "--quiet", "main")
		f.stage("a.txt", "two\n")
		op := safety.Commit(git.CommitOptions{Message: "second"})

		a := f.assess(op)
		assert.Equal(t, safety.RiskCaution, a.Risk)
		assert.Equal(t, safety.ConfirmSimple, a.Confirm)
		assert.Equal(t, "main", finding(t, a, safety.CodeProtectedBranchCommit).Detail)

		before := f.head()
		_, err := f.do(op, no)
		require.ErrorIs(t, err, safety.ErrDeclined)
		assert.Equal(t, before, f.head(), "declining changes nothing")

		// --yes is enough for a caution.
		_, err = f.guard.Do(context.Background(), op, never(t), safety.DoOptions{AssumeYes: true})
		require.NoError(t, err)
		assert.NotEqual(t, before, f.head())
	})

	t.Run("a protected pattern", func(t *testing.T) {
		policy := safety.DefaultPolicy()
		policy.ProtectedBranches = []string{"release/*", "[bad"}
		assert.True(t, policy.Protected("release/1.2"))
		assert.False(t, policy.Protected("release"))
		assert.False(t, policy.Protected("main"))
		assert.True(t, policy.Protected("[bad"), "an unreadable pattern is still a name")
	})

	t.Run("on a detached HEAD", func(t *testing.T) {
		f := newFixture(t)
		f.repo.Git("switch", "--quiet", "--detach")
		f.stage("a.txt", "two\n")

		a := f.assess(safety.Commit(git.CommitOptions{Message: "second"}))
		assert.Equal(t, []safety.Code{safety.CodeDetachedHeadCommit}, codes(a))
		assert.Equal(t, safety.ConfirmSimple, a.Confirm)
	})

	t.Run("secrets, large files and build output", func(t *testing.T) {
		f := newFixture(t)
		token := "ghp" + "_" + strings.Repeat("a1B2", 9)
		f.stage("config.go", "package config\n\nconst name = \"app\"\nconst token = \""+token+"\"\n")
		f.stage(".env", "DEBUG=1\n")
		f.stage("big.bin", strings.Repeat("0123456789abcdef", 200))
		f.stage("web/node_modules/x/index.js", "x\n")
		f.stage("web/node_modules/y/index.js", "y\n")
		policy := safety.DefaultPolicy()
		policy.LargeFileBytes = 1024
		f = guardFor(t, f.repo, policy)

		a := f.assess(safety.Commit(git.CommitOptions{Message: "oops"}))

		assert.Equal(t, safety.RiskCaution, a.Risk)
		secret := finding(t, a, safety.CodeSecret)
		assert.Equal(t, "config.go", secret.Path)
		assert.Equal(t, 4, secret.Line)
		assert.Equal(t, "GitHub token", secret.Detail)
		assert.Equal(t, ".env", finding(t, a, safety.CodeSecretFile).Path)
		assert.Equal(t, "big.bin", finding(t, a, safety.CodeLargeFile).Path)
		junk := finding(t, a, safety.CodeJunkFile)
		assert.Equal(t, "web/node_modules", junk.Path)
		assert.Equal(t, 2, junk.Count)

		for _, finding := range a.Findings {
			for _, lang := range []i18n.Lang{i18n.TR, i18n.EN} {
				text := finding.Localize(lang)
				assert.NotContains(t, text, token, "a finding never repeats the secret")
				assert.NotContains(t, text, "%!", "every placeholder is filled in")
			}
		}

		t.Run("the secret scan can be turned off, the rest stays", func(t *testing.T) {
			policy.WarnOnSecrets = false
			quiet := guardFor(t, f.repo, policy).assess(safety.Commit(git.CommitOptions{Message: "oops"}))
			assert.NotContains(t, codes(quiet), safety.CodeSecret)
			assert.NotContains(t, codes(quiet), safety.CodeSecretFile)
			assert.Contains(t, codes(quiet), safety.CodeLargeFile)
		})
	})

	t.Run("a secret being removed is not a finding", func(t *testing.T) {
		f := newFixture(t)
		token := "ghp" + "_" + strings.Repeat("a1B2", 9)
		f.repo.WriteFile("config.txt", "token="+token+"\n")
		f.repo.Commit("leaked earlier")
		f.stage("config.txt", "token is read from the environment\n")

		assert.Empty(t, f.assess(safety.Commit(git.CommitOptions{Message: "remove token"})).Findings)
	})
}

func TestAmend(t *testing.T) {
	t.Run("a commit that was never pushed", func(t *testing.T) {
		f := newFixture(t)
		old := f.head()
		op := safety.Commit(git.CommitOptions{Message: "first, reworded", Amend: true})

		a := f.assess(op)
		assert.Equal(t, safety.RiskCaution, a.Risk)
		assert.Equal(t, 1, effect(t, a, safety.EffectCommitRewritten).Count)

		res := f.mustDo(op, yes)
		assert.Equal(t, "amend", res.Entry.Op)
		assert.Equal(t, old, res.Entry.Before)
		assert.Equal(t, old, f.repo.Git("rev-parse", res.Entry.Backup), "the replaced commit is kept")
	})

	t.Run("a commit that is already on the remote", func(t *testing.T) {
		origin := gittest.NewBareRepo(t)
		repo := gittest.Clone(t, origin)
		repo.WriteFile("a.txt", "one\n")
		repo.Commit("first")
		repo.Git("switch", "--quiet", "-c", "work")
		repo.Git("push", "--quiet", "--set-upstream", "origin", "work")
		f := guardFor(t, repo, safety.DefaultPolicy())
		op := safety.Commit(git.CommitOptions{Message: "reworded", Amend: true})

		a := f.assess(op)
		assert.Equal(t, safety.RiskDangerous, a.Risk)
		assert.Equal(t, safety.ConfirmExplicit, a.Confirm)
		assert.Equal(t, safety.SeverityWarn, finding(t, a, safety.CodeRewritesPushed).Severity)

		old := f.head()
		_, err := f.guard.Do(context.Background(), op, nil, safety.DoOptions{AssumeYes: true})
		require.ErrorIs(t, err, safety.ErrConfirmationRequired, "--yes does not cover a dangerous operation")
		assert.Equal(t, old, f.head())

		f.mustDo(op, yes)
		assert.NotEqual(t, old, f.head())
	})
}

func TestDiscard(t *testing.T) {
	f := newFixture(t)
	f.repo.WriteFile("a.txt", "precious work\n")
	f.repo.WriteFile("b.txt", "more work\n")
	f.repo.WriteFile("untracked.txt", "mine\n")
	op := safety.Discard("a.txt")

	a := f.assess(op)
	assert.Equal(t, safety.RiskDangerous, a.Risk)
	assert.Equal(t, safety.ConfirmExplicit, a.Confirm)
	assert.True(t, a.BackupWorktree)
	assert.Equal(t, []string{"a.txt"}, effect(t, a, safety.EffectFilesDiscarded).Items)

	t.Run("declined: nothing happens", func(t *testing.T) {
		_, err := f.do(op, no)
		require.ErrorIs(t, err, safety.ErrDeclined)
		assert.Equal(t, "precious work\n", f.repo.ReadFile("a.txt"))
		backups, err := f.guard.Backups(context.Background())
		require.NoError(t, err)
		assert.Empty(t, backups, "no backup is left behind by an operation that did not run")
	})

	t.Run("--yes is not enough", func(t *testing.T) {
		_, err := f.guard.Do(context.Background(), op, nil, safety.DoOptions{AssumeYes: true})
		require.ErrorIs(t, err, safety.ErrConfirmationRequired)
		assert.Equal(t, "precious work\n", f.repo.ReadFile("a.txt"))
	})

	t.Run("confirmed: the file is reset and its content kept in a backup", func(t *testing.T) {
		res := f.mustDo(op, yes)

		assert.Equal(t, "one\n", f.repo.ReadFile("a.txt"))
		assert.Equal(t, "more work\n", f.repo.ReadFile("b.txt"), "other files are left alone")
		assert.Equal(t, "mine\n", f.repo.ReadFile("untracked.txt"))
		require.NotEmpty(t, res.Entry.WorktreeBackup)
		assert.Equal(t, "precious work", f.repo.Git("show", res.Entry.WorktreeBackup+":a.txt"))
		assert.Equal(t, []string{"a.txt"}, res.Entry.Paths)
	})

	t.Run("a file without changes is nothing to worry about", func(t *testing.T) {
		clean := f.assess(safety.Discard("a.txt"))
		assert.Equal(t, safety.RiskSafe, clean.Risk)
	})

	t.Run("a directory covers the files under it", func(t *testing.T) {
		f.repo.WriteFile("dir/x.txt", "x\n")
		f.repo.Commit("add dir")
		f.repo.WriteFile("dir/x.txt", "changed\n")
		assert.Equal(t, []string{"dir/x.txt"}, effect(t, f.assess(safety.Discard("dir")), safety.EffectFilesDiscarded).Items)
	})
}

func TestResetHard(t *testing.T) {
	f := newFixture(t)
	first := f.head()
	f.repo.WriteFile("a.txt", "two\n")
	second := f.repo.Commit("second")
	f.repo.WriteFile("a.txt", "three\n")
	third := f.repo.Commit("third")
	f.repo.WriteFile("b.txt", "uncommitted\n")
	op := safety.Reset(git.ResetHard, first)

	a := f.assess(op)
	assert.Equal(t, safety.RiskDangerous, a.Risk)
	assert.Equal(t, 2, effect(t, a, safety.EffectCommitsDropped).Count)
	assert.Equal(t, []string{"b.txt"}, effect(t, a, safety.EffectFilesDiscarded).Items)
	assert.Equal(t, third, a.BackupCommit)
	assert.True(t, a.BackupWorktree)
	assert.Equal(t, 1, finding(t, a, safety.CodeUncommittedChanges).Count)

	res := f.mustDo(op, yes)

	assert.Equal(t, first, f.head())
	assert.Equal(t, "one\n", f.repo.ReadFile("b.txt"))
	assert.Equal(t, third, f.repo.Git("rev-parse", res.Entry.Backup))
	assert.Equal(t, "uncommitted", f.repo.Git("show", res.Entry.WorktreeBackup+":b.txt"))
	assert.Equal(t, "hard", res.Entry.Detail)
	_ = second

	t.Run("soft keeps everything and only needs a yes", func(t *testing.T) {
		f.repo.WriteFile("a.txt", "again\n")
		f.repo.Commit("again")
		soft := f.assess(safety.Reset(git.ResetSoft, first))
		assert.Equal(t, safety.RiskCaution, soft.Risk)
		assert.False(t, soft.BackupWorktree)
	})

	t.Run("resetting to where you are with nothing to lose is safe", func(t *testing.T) {
		f.repo.Git("reset", "--quiet", "--hard")
		assert.Equal(t, safety.RiskSafe, f.assess(safety.Reset(git.ResetHard, "HEAD")).Risk)
	})

	t.Run("a revision that does not exist", func(t *testing.T) {
		_, err := f.guard.Assess(context.Background(), safety.Reset(git.ResetHard, "nope"))
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	})
}

func TestDeleteBranch(t *testing.T) {
	setup := func(t *testing.T) (*fixture, string) {
		f := newFixture(t)
		f.repo.Git("branch", "merged")
		f.repo.Git("switch", "--quiet", "-c", "unmerged")
		f.repo.WriteFile("c.txt", "c\n")
		tip := f.repo.Commit("only here")
		f.repo.Git("switch", "--quiet", "work")
		return f, tip
	}

	t.Run("a merged branch", func(t *testing.T) {
		f, _ := setup(t)
		a := f.assess(safety.DeleteBranch("merged", false))
		assert.Equal(t, safety.RiskCaution, a.Risk)
		assert.Equal(t, []string{"merged"}, effect(t, a, safety.EffectBranchDeleted).Items)

		f.mustDo(safety.DeleteBranch("merged", false), yes)
		assert.Equal(t, "main\nunmerged\nwork", f.repo.Git("for-each-ref", "--format=%(refname:short)", "refs/heads/"))
	})

	t.Run("an unmerged branch is blocked without force", func(t *testing.T) {
		f, _ := setup(t)
		a := f.assess(safety.DeleteBranch("unmerged", false))
		assert.True(t, a.Blocked())
		assert.Equal(t, 1, finding(t, a, safety.CodeUnmergedBranch).Count)

		_, err := f.do(safety.DeleteBranch("unmerged", false), yes)
		var blocked *safety.BlockedError
		require.ErrorAs(t, err, &blocked)
		assert.Contains(t, f.repo.Git("branch", "--list"), "unmerged")
	})

	t.Run("an unmerged branch, forced: its commits are kept in a backup", func(t *testing.T) {
		f, tip := setup(t)
		a := f.assess(safety.DeleteBranch("unmerged", true))
		assert.Equal(t, safety.RiskDangerous, a.Risk)
		assert.False(t, a.Blocked())

		res := f.mustDo(safety.DeleteBranch("unmerged", true), yes)

		assert.NotContains(t, f.repo.Git("branch", "--list"), "unmerged")
		assert.Equal(t, tip, f.repo.Git("rev-parse", res.Entry.Backup))
		assert.Equal(t, "unmerged", res.Entry.Target)
	})

	t.Run("a protected branch needs an explicit yes", func(t *testing.T) {
		f, _ := setup(t)
		a := f.assess(safety.DeleteBranch("main", false))
		assert.Equal(t, safety.RiskDangerous, a.Risk)
		assert.Equal(t, "main", finding(t, a, safety.CodeProtectedBranchDelete).Detail)
	})

	t.Run("a branch that does not exist", func(t *testing.T) {
		f, _ := setup(t)
		_, err := f.guard.Assess(context.Background(), safety.DeleteBranch("nope", false))
		assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
	})
}

// remoteFixture returns a guard on a clone whose branch "work" is pushed, and
// a second clone to play someone else.
func remoteFixture(t *testing.T) (f *fixture, origin, other *gittest.Repo) {
	t.Helper()
	origin = gittest.NewBareRepo(t)
	repo := gittest.Clone(t, origin)
	repo.WriteFile("a.txt", "one\n")
	repo.Commit("first")
	repo.Git("push", "--quiet", "--set-upstream", "origin", "main")
	repo.Git("switch", "--quiet", "-c", "work")
	repo.Git("push", "--quiet", "--set-upstream", "origin", "work")
	other = gittest.Clone(t, origin)
	other.Git("switch", "--quiet", "work")
	return guardFor(t, repo, safety.DefaultPolicy()), origin, other
}

func TestPush(t *testing.T) {
	t.Run("new commits go out without questions", func(t *testing.T) {
		f, origin, _ := remoteFixture(t)
		f.repo.WriteFile("a.txt", "two\n")
		head := f.repo.Commit("second")

		a := f.assess(safety.Push(git.PushOptions{}))
		assert.Equal(t, safety.RiskSafe, a.Risk)
		assert.Equal(t, 1, effect(t, a, safety.EffectCommitsPushed).Count)

		res := f.mustDo(safety.Push(git.PushOptions{}), never(t))
		assert.Equal(t, git.PushFastForward, res.Push.Status)
		assert.Equal(t, head, origin.Git("rev-parse", "refs/heads/work"))
		assert.Equal(t, "push", res.Entry.Op)
	})

	t.Run("blocked when the remote has commits we lack", func(t *testing.T) {
		f, origin, other := remoteFixture(t)
		other.WriteFile("b.txt", "theirs\n")
		theirs := other.Commit("their work")
		other.Git("push", "--quiet")
		f.repo.WriteFile("a.txt", "two\n")
		f.repo.Commit("second")
		f.repo.Git("fetch", "--quiet")

		a := f.assess(safety.Push(git.PushOptions{}))
		assert.True(t, a.Blocked())
		assert.Equal(t, 1, finding(t, a, safety.CodeBehindRemote).Count)

		_, err := f.do(safety.Push(git.PushOptions{}), yes)
		var blocked *safety.BlockedError
		require.ErrorAs(t, err, &blocked)
		assert.Contains(t, blocked.Localize(i18n.TR), "pull")
		assert.Equal(t, theirs, origin.Git("rev-parse", "refs/heads/work"))
	})

	t.Run("a failed push is recorded as failed", func(t *testing.T) {
		f, _, other := remoteFixture(t)
		other.WriteFile("b.txt", "theirs\n")
		other.Commit("their work")
		other.Git("push", "--quiet")
		f.repo.WriteFile("a.txt", "two\n")
		f.repo.Commit("second")
		// Not fetched: the guard cannot know, git finds out.

		_, err := f.do(safety.Push(git.PushOptions{}), never(t))
		assert.Equal(t, git.KindRejected, git.KindOf(err))

		entries, herr := f.guard.History().Entries()
		require.NoError(t, herr)
		require.Len(t, entries, 1)
		assert.Equal(t, string(git.KindRejected), entries[0].Failed)
	})
}

func TestForcePush(t *testing.T) {
	t.Run("to a protected branch: always blocked", func(t *testing.T) {
		f, origin, _ := remoteFixture(t)
		f.repo.Git("switch", "--quiet", "main")
		before := origin.Git("rev-parse", "refs/heads/main")
		f.repo.Git("commit", "--quiet", "--amend", "-m", "rewritten")
		op := safety.Push(git.PushOptions{ForceWithLease: true})

		a := f.assess(op)
		assert.True(t, a.Blocked())
		assert.Equal(t, "main", finding(t, a, safety.CodeProtectedForcePush).Detail)

		_, err := f.do(op, typed("main"))
		var blocked *safety.BlockedError
		require.ErrorAs(t, err, &blocked)
		assert.Equal(t, before, origin.Git("rev-parse", "refs/heads/main"))
	})

	t.Run("a local branch that pushes to a protected one is blocked too", func(t *testing.T) {
		f, _, _ := remoteFixture(t)
		f.repo.Git("switch", "--quiet", "-c", "sneaky", "--track", "origin/main")
		f.repo.Git("commit", "--quiet", "--amend", "-m", "rewritten")

		assert.True(t, f.assess(safety.Push(git.PushOptions{ForceWithLease: true})).Blocked())
	})

	t.Run("elsewhere: the branch name must be typed", func(t *testing.T) {
		f, origin, _ := remoteFixture(t)
		before := origin.Git("rev-parse", "refs/heads/work")
		f.repo.Git("commit", "--quiet", "--amend", "-m", "rewritten")
		after := f.head()
		op := safety.Push(git.PushOptions{ForceWithLease: true})

		a := f.assess(op)
		assert.Equal(t, safety.RiskCritical, a.Risk)
		assert.Equal(t, safety.ConfirmTypeBranch, a.Confirm)
		assert.Equal(t, "work", a.ConfirmText)
		assert.Equal(t, 1, effect(t, a, safety.EffectRemoteCommitsReplaced).Count)
		assert.Equal(t, 1, finding(t, a, safety.CodeForcePush).Count)

		for name, approve := range map[string]safety.Approver{
			"a plain yes": yes, "the wrong name": typed("main"), "a refusal": no,
		} {
			_, err := f.do(op, approve)
			require.ErrorIs(t, err, safety.ErrDeclined, name)
			assert.Equal(t, before, origin.Git("rev-parse", "refs/heads/work"), name)
		}
		_, err := f.guard.Do(context.Background(), op, nil, safety.DoOptions{AssumeYes: true})
		require.ErrorIs(t, err, safety.ErrConfirmationRequired)

		res := f.mustDo(op, typed("work"))

		assert.Equal(t, git.PushForced, res.Push.Status)
		assert.Equal(t, after, origin.Git("rev-parse", "refs/heads/work"))
		assert.Equal(t, "force-push", res.Entry.Op)
		assert.Equal(t, before, f.repo.Git("rev-parse", res.Entry.Backup), "what was on the remote is kept locally")
	})
}

func TestPull(t *testing.T) {
	t.Run("fast-forward is safe", func(t *testing.T) {
		f, _, other := remoteFixture(t)
		other.WriteFile("b.txt", "theirs\n")
		theirs := other.Commit("their work")
		other.Git("push", "--quiet")

		res := f.mustDo(safety.Pull(git.PullOptions{}), never(t))

		assert.True(t, res.Pull.Updated)
		assert.Equal(t, theirs, f.head())
		assert.Equal(t, "pull", res.Entry.Op)
		assert.Equal(t, theirs, res.Entry.After)
	})

	t.Run("rebase is summarized and backed up", func(t *testing.T) {
		f, _, other := remoteFixture(t)
		other.WriteFile("b.txt", "theirs\n")
		other.Commit("their work")
		other.Git("push", "--quiet")
		f.repo.WriteFile("c.txt", "mine\n")
		mine := f.repo.Commit("my work")
		f.repo.WriteFile("d.txt", "tracked later\n")
		f.repo.Git("add", "d.txt")
		op := safety.Pull(git.PullOptions{Strategy: git.PullRebase})

		a := f.assess(op)
		assert.Equal(t, safety.RiskCaution, a.Risk)
		assert.Equal(t, mine, a.BackupCommit)
		assert.True(t, a.BackupWorktree, "uncommitted changes are backed up before a pull (rule 4)")
		assert.Equal(t, safety.SeverityInfo, finding(t, a, safety.CodeUncommittedChanges).Severity)

		res, err := f.do(op, yes)
		// git refuses to rebase over a dirty index; the backups exist anyway.
		assert.Equal(t, git.KindDirtyWorktree, git.KindOf(err))
		_ = res
		entries, herr := f.guard.History().Entries()
		require.NoError(t, herr)
		require.Len(t, entries, 1)
		assert.Equal(t, mine, f.repo.Git("rev-parse", entries[0].Backup))
		assert.Equal(t, "tracked later", f.repo.Git("show", entries[0].WorktreeBackup+":d.txt"))
	})
}

func TestOperationInProgressBlocksOthers(t *testing.T) {
	f := newFixture(t)
	f.repo.Git("switch", "--quiet", "-c", "other")
	f.repo.WriteFile("a.txt", "theirs\n")
	f.repo.Commit("theirs")
	f.repo.Git("switch", "--quiet", "work")
	f.repo.WriteFile("a.txt", "ours\n")
	f.repo.Commit("ours")
	_, err := f.repo.Runner.Run(context.Background(), git.Cmd{Dir: f.repo.Dir, Args: []string{"merge", "other"}})
	require.Error(t, err)

	for name, op := range map[string]safety.Operation{
		"switch":     safety.Switch("main"),
		"pull":       safety.Pull(git.PullOptions{}),
		"amend":      safety.Commit(git.CommitOptions{Message: "x", Amend: true}),
		"reset":      safety.Reset(git.ResetSoft, "HEAD"),
		"force push": safety.Push(git.PushOptions{ForceWithLease: true}),
	} {
		a := f.assess(op)
		assert.True(t, a.Blocked(), name)
		assert.Equal(t, "merge", finding(t, a, safety.CodeOperationInProgress).Detail, name)
	}

	t.Run("the commit that concludes the merge is allowed", func(t *testing.T) {
		f.repo.WriteFile("a.txt", "resolved\n")
		f.repo.Git("add", "a.txt")
		assert.False(t, f.assess(safety.Commit(git.CommitOptions{Message: "merge"})).Blocked())
	})
}

func TestSwitch(t *testing.T) {
	f := newFixture(t)
	f.repo.WriteFile("a.txt", "uncommitted\n")

	a := f.assess(safety.Switch("main"))
	assert.Equal(t, safety.RiskSafe, a.Risk)
	assert.True(t, a.BackupWorktree)

	res := f.mustDo(safety.Switch("main"), never(t))

	assert.Equal(t, "main", f.repo.Git("branch", "--show-current"))
	assert.Equal(t, "work", res.Entry.Branch)
	assert.Equal(t, "main", res.Entry.Target)
	assert.Equal(t, "uncommitted", f.repo.Git("show", res.Entry.WorktreeBackup+":a.txt"))
}

// The user is shown one state and agrees to it; if the repository is in
// another state by the time they answer, the answer no longer counts.
func TestRepositoryChangedWhileConfirming(t *testing.T) {
	f := newFixture(t)
	f.repo.WriteFile("a.txt", "work\n")
	sneaky := func(safety.Assessment) (safety.Answer, error) {
		f.repo.Git("commit", "--quiet", "--allow-empty", "-m", "someone else was here")
		return safety.Answer{Approved: true}, nil
	}

	_, err := f.do(safety.Discard("a.txt"), sneaky)

	require.ErrorIs(t, err, safety.ErrRepositoryChanged)
	assert.Equal(t, "work\n", f.repo.ReadFile("a.txt"))
}

func TestApproverErrorStopsTheOperation(t *testing.T) {
	f := newFixture(t)
	f.repo.WriteFile("a.txt", "work\n")
	boom := errors.New("terminal went away")

	_, err := f.do(safety.Discard("a.txt"), func(safety.Assessment) (safety.Answer, error) { return safety.Answer{}, boom })

	require.ErrorIs(t, err, boom)
	assert.Equal(t, "work\n", f.repo.ReadFile("a.txt"))
}

func TestGuardOutsideARepository(t *testing.T) {
	_, err := safety.NewGuard(context.Background(), gittest.NewRunner(t), gittest.TempDir(t), safety.DefaultPolicy())
	assert.Equal(t, git.KindNotARepo, git.KindOf(err))
}

func TestBackups(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.repo.WriteFile("a.txt", "work one\n")
	first := f.mustDo(safety.Discard("a.txt"), yes)
	f.repo.Git("branch", "side")
	second := f.mustDo(safety.DeleteBranch("side", false), yes)

	backups, err := f.guard.Backups(ctx)
	require.NoError(t, err)
	require.Len(t, backups, 2)

	t.Run("newest first, with what they are", func(t *testing.T) {
		assert.Equal(t, second.Entry.Backup, backups[0].Ref)
		assert.Equal(t, "delete-branch", backups[0].Op)
		assert.False(t, backups[0].Worktree)
		assert.Equal(t, first.Entry.WorktreeBackup, backups[1].Ref)
		assert.Equal(t, "discard", backups[1].Op)
		assert.True(t, backups[1].Worktree)
		assert.True(t, backups[0].Time.After(backups[1].Time))
		assert.Equal(t, time.UTC, backups[0].Time.Location())
	})

	t.Run("restoring a deleted branch", func(t *testing.T) {
		_, err := f.guard.RestoreBackup(ctx, second.Entry.Backup, yes, safety.DoOptions{})
		require.NoError(t, err)
		assert.Equal(t, f.head(), f.repo.Git("rev-parse", "side"))
	})

	t.Run("restoring uncommitted changes", func(t *testing.T) {
		_, err := f.guard.RestoreBackup(ctx, first.Entry.WorktreeBackup, yes, safety.DoOptions{})
		require.NoError(t, err)
		assert.Equal(t, "work one\n", f.repo.ReadFile("a.txt"))
	})

	t.Run("a backup that does not exist", func(t *testing.T) {
		_, err := f.guard.RestoreBackup(ctx, "refs/gitpal/backup/nope", yes, safety.DoOptions{})
		require.ErrorIs(t, err, safety.ErrBackupNotFound)
		_, err = f.guard.RestoreBackup(ctx, "refs/heads/main", yes, safety.DoOptions{})
		require.ErrorIs(t, err, safety.ErrBackupNotFound)
	})

	t.Run("prune removes only what is older than the retention", func(t *testing.T) {
		removed, err := f.guard.PruneBackups(ctx)
		require.NoError(t, err)
		assert.Empty(t, removed)

		// The next reading of the clock is one minute past the older
		// backup's 30 days; the newer one is exactly 30 days old.
		f.clock.now = backups[1].Time.Add(30 * 24 * time.Hour)
		removed, err = f.guard.PruneBackups(ctx)
		require.NoError(t, err)
		require.Len(t, removed, 1, "the first backup just crossed 30 days")
		assert.Equal(t, first.Entry.WorktreeBackup, removed[0].Ref)

		left, err := f.guard.Backups(ctx)
		require.NoError(t, err)
		require.Len(t, left, 1)
		assert.Equal(t, second.Entry.Backup, left[0].Ref)
	})
}

func TestRestoreBackupOfACommit(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	first := f.head()
	f.repo.WriteFile("a.txt", "two\n")
	second := f.repo.Commit("second")
	reset := f.mustDo(safety.Reset(git.ResetHard, first), yes)

	a, err := f.guard.RestoreBackup(ctx, reset.Entry.Backup, yes, safety.DoOptions{})
	require.NoError(t, err)

	assert.Equal(t, second, f.head())
	assert.Equal(t, "reset", a.Entry.Op, "restoring is a reset like any other, with its own record")
}

func TestEveryFindingExplainsItself(t *testing.T) {
	for _, code := range safety.AllCodes() {
		for _, lang := range []i18n.Lang{i18n.TR, i18n.EN} {
			text := safety.Finding{Code: code, Path: "p", Line: 1, Detail: "d", Count: 2}.Localize(lang)
			assert.NotEqual(t, string(code), text, "%s has no %s message", code, lang)
			assert.NotContains(t, text, "%!", "%s: %s", code, text)
		}
	}
	assert.Equal(t, "unknown_code", safety.Finding{Code: "unknown_code"}.Localize(i18n.EN))
	for _, risk := range []safety.Risk{safety.RiskSafe, safety.RiskCaution, safety.RiskDangerous, safety.RiskCritical} {
		assert.NotEqual(t, "unknown", risk.String())
	}
}

// newEmptyRepo returns a repository without commits, on the branch "work".
func newEmptyRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.NewRepo(t)
	repo.Git("switch", "--quiet", "-c", "work")
	return repo
}

func TestSummaryPutsAnAssessmentIntoWords(t *testing.T) {
	f := newFixture(t)
	for i := range 7 {
		f.repo.WriteFile("f"+string(rune('a'+i))+".txt", "x\n")
	}
	f.repo.Commit("many files")
	for i := range 7 {
		f.repo.WriteFile("f"+string(rune('a'+i))+".txt", "changed\n")
	}

	for _, lang := range []i18n.Lang{i18n.TR, i18n.EN} {
		s := f.assess(safety.Discard(".")).Summary(lang)
		assert.NotEmpty(t, s.Risk)
		require.Len(t, s.Effects, 1)
		assert.Contains(t, s.Effects[0], "7")
		assert.Contains(t, s.Effects[0], "…", "a long list is cut short")
		assert.Len(t, s.Backups, 1)
		for _, line := range append(append(s.Effects, s.Backups...), s.Risk) {
			assert.NotContains(t, line, "%!")
			assert.NotContains(t, line, "cli.", "every text comes from the catalog")
		}
	}
}
