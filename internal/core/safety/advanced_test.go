package safety_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
)

// sideFixture adds a branch "side" with one commit next to "work".
func sideFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.repo.Git("switch", "--quiet", "-c", "side")
	f.repo.WriteFile("side.txt", "side\n")
	f.repo.Commit("side work")
	f.repo.Git("switch", "--quiet", "work")
	f.repo.WriteFile("work.txt", "work\n")
	f.repo.Commit("work work")
	return f
}

func TestMergeThroughTheGuard(t *testing.T) {
	f := sideFixture(t)
	before := f.head()
	op := safety.Merge(git.MergeOptions{Rev: "side"})

	a := f.assess(op)
	assert.Equal(t, safety.RiskCaution, a.Risk)
	assert.Equal(t, before, a.BackupCommit)

	res := f.mustDo(op, yes)
	assert.False(t, res.Merge.FastForward)
	assert.Equal(t, "merge", res.Entry.Op)
	assert.Equal(t, "side", res.Entry.Target)

	f.mustUndo()
	assert.Equal(t, before, f.head())
}

func TestRebaseThroughTheGuard(t *testing.T) {
	t.Run("local commits only: caution", func(t *testing.T) {
		f := sideFixture(t)
		before := f.head()
		op := safety.Rebase(git.RebaseOptions{Upstream: "side"})
		a := f.assess(op)
		assert.Equal(t, safety.RiskCaution, a.Risk)
		assert.Equal(t, 1, effect(t, a, safety.EffectCommitsReplayed).Count)

		f.mustDo(op, yes)
		assert.Equal(t, f.repo.Git("rev-parse", "side"), f.repo.Git("rev-parse", "HEAD~1"))

		f.mustUndo()
		assert.Equal(t, before, f.head())
	})

	t.Run("pushed commits: dangerous", func(t *testing.T) {
		f, _, _ := remoteFixture(t)
		f.repo.WriteFile("b.txt", "b\n")
		f.repo.Commit("pushed")
		f.repo.Git("push", "--quiet")
		f.repo.Git("branch", "base", "main")
		f.repo.Git("switch", "--quiet", "base")
		f.repo.WriteFile("c.txt", "c\n")
		f.repo.Commit("base moved")
		f.repo.Git("switch", "--quiet", "work")

		a := f.assess(safety.Rebase(git.RebaseOptions{Upstream: "base"}))
		assert.Equal(t, safety.RiskDangerous, a.Risk)
		assert.Equal(t, 1, finding(t, a, safety.CodeRewritesPushed).Count)
	})
}

func TestCherryPickAndRevertThroughTheGuard(t *testing.T) {
	f := sideFixture(t)
	before := f.head()

	assert.Equal(t, safety.RiskCaution, f.assess(safety.CherryPick("side")).Risk)
	f.mustDo(safety.CherryPick("side"), yes)
	assert.Equal(t, "side work", f.repo.Git("log", "-1", "--format=%s"))

	assert.Equal(t, safety.RiskSafe, f.assess(safety.Revert(0, "HEAD")).Risk)
	f.mustDo(safety.Revert(0, "HEAD"), never(t))
	assert.Equal(t, `Revert "side work"`, f.repo.Git("log", "-1", "--format=%s"))

	f.mustUndo()
	f.mustUndo()
	assert.Equal(t, before, f.head())
}

func TestStashDropIsBackedUpAndUndone(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.repo.WriteFile("a.txt", "stashed work\n")
	_, err := f.repo.Runner.StashPush(ctx, f.repo.Dir, git.StashOptions{Message: "keep me"})
	require.NoError(t, err)

	a := f.assess(safety.StashDrop(0))
	assert.Equal(t, safety.RiskDangerous, a.Risk)
	assert.Contains(t, effect(t, a, safety.EffectStashDropped).Items[0], "keep me")

	_, err = f.guard.Do(ctx, safety.StashDrop(0), nil, safety.DoOptions{AssumeYes: true})
	require.ErrorIs(t, err, safety.ErrConfirmationRequired)

	f.mustDo(safety.StashDrop(0), yes)
	stashes, err := f.repo.Runner.Stashes(ctx, f.repo.Dir)
	require.NoError(t, err)
	assert.Empty(t, stashes)

	f.mustUndo()
	stashes, err = f.repo.Runner.Stashes(ctx, f.repo.Dir)
	require.NoError(t, err)
	require.Len(t, stashes, 1)
	assert.Contains(t, stashes[0].Message, "keep me")
	require.NoError(t, f.repo.Runner.StashApply(ctx, f.repo.Dir, 0, true))
	assert.Equal(t, "stashed work\n", f.repo.ReadFile("a.txt"))

	_, err = f.guard.Assess(ctx, safety.StashDrop(3))
	assert.Equal(t, git.KindRefNotFound, git.KindOf(err))
}

func TestAbortThroughTheGuard(t *testing.T) {
	f := newFixture(t)
	_, err := f.guard.Assess(context.Background(), safety.Abort())
	require.Error(t, err)
	assert.Contains(t, i18n.Describe(i18n.EN, err), "no merge, rebase")

	f.repo.Git("switch", "--quiet", "-c", "other")
	f.repo.WriteFile("a.txt", "theirs\n")
	f.repo.Commit("theirs")
	f.repo.Git("switch", "--quiet", "work")
	f.repo.WriteFile("a.txt", "ours\n")
	f.repo.Commit("ours")
	before := f.head()
	_, err = f.do(safety.Merge(git.MergeOptions{Rev: "other"}), yes)
	require.Equal(t, git.KindConflict, git.KindOf(err))

	a := f.assess(safety.Abort())
	assert.Equal(t, safety.RiskCaution, a.Risk)
	assert.Contains(t, a.Summary(i18n.EN).Effects[0], "merge")
	assert.True(t, f.assess(safety.Merge(git.MergeOptions{Rev: "other"})).Blocked(), "one operation at a time")

	f.mustDo(safety.Abort(), yes)
	assert.Equal(t, before, f.head())
}

func TestContinueIsRecordedAndUndone(t *testing.T) {
	f := newFixture(t)
	f.repo.Git("switch", "--quiet", "-c", "other")
	f.repo.WriteFile("a.txt", "theirs\n")
	f.repo.Commit("theirs")
	f.repo.Git("switch", "--quiet", "work")
	f.repo.WriteFile("a.txt", "ours\n")
	f.repo.Commit("ours")
	before := f.head()
	_, err := f.do(safety.Merge(git.MergeOptions{Rev: "other"}), yes)
	require.Equal(t, git.KindConflict, git.KindOf(err))
	require.NoError(t, f.repo.Runner.ResolveConflict(context.Background(), f.repo.Dir, []string{"a.txt"}, git.SideOurs))

	op, entry, err := f.guard.Continue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, git.OpMerge, op)
	assert.Equal(t, before, entry.Before, "the record reaches back to where the merge began")
	assert.NotEqual(t, before, f.head())

	f.mustUndo()
	assert.Equal(t, before, f.head())

	_, _, err = f.guard.Continue(context.Background())
	assert.True(t, git.ErrNoOperation(err))
}
