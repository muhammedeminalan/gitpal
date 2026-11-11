package safety_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/history"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
)

func (f *fixture) undo() (history.Entry, error) {
	f.t.Helper()
	return f.guard.Undo(context.Background(), yes, safety.DoOptions{})
}

func (f *fixture) mustUndo() history.Entry {
	f.t.Helper()
	entry, err := f.undo()
	require.NoError(f.t, err)
	return entry
}

func (f *fixture) short() string {
	return f.repo.Git("status", "--porcelain=v1", "--untracked-files=all")
}

func TestUndoNothing(t *testing.T) {
	f := newFixture(t)

	_, err := f.undo()
	require.ErrorIs(t, err, safety.ErrUndoNothing)

	plan, err := f.guard.PlanUndo(context.Background())
	require.NoError(t, err)
	require.ErrorIs(t, plan.Err, safety.ErrUndoNothing)
}

func TestUndoCommit(t *testing.T) {
	f := newFixture(t)
	before := f.head()
	f.stage("a.txt", "two\n")
	done := f.mustDo(safety.Commit(git.CommitOptions{Message: "second"}), never(t))

	plan, err := f.guard.PlanUndo(context.Background())
	require.NoError(t, err)
	require.NoError(t, plan.Err)
	assert.Equal(t, done.Entry.ID, plan.Entry.ID)

	entry := f.mustUndo()

	assert.Equal(t, before, f.head())
	assert.Equal(t, "M  a.txt", f.short(), "the changes are staged again, not lost")
	assert.Equal(t, history.OpUndo, entry.Op)
	assert.Equal(t, done.Entry.ID, entry.Undoes)

	t.Run("and then there is nothing more to undo", func(t *testing.T) {
		_, err := f.undo()
		require.ErrorIs(t, err, safety.ErrUndoNothing)
	})
}

func TestUndoWalksBackOneOperationAtATime(t *testing.T) {
	f := newFixture(t)
	first := f.head()
	f.stage("a.txt", "two\n")
	f.mustDo(safety.Commit(git.CommitOptions{Message: "second"}), never(t))
	second := f.head()
	f.stage("a.txt", "three\n")
	f.mustDo(safety.Commit(git.CommitOptions{Message: "third"}), never(t))

	f.mustUndo()
	assert.Equal(t, second, f.head())

	// The staged change of the undone commit is in the way of nothing: the
	// next undo takes the earlier commit back as well.
	f.mustUndo()
	assert.Equal(t, first, f.head())
	assert.Equal(t, "three\n", f.repo.ReadFile("a.txt"))
}

func TestUndoCommitRefusals(t *testing.T) {
	t.Run("the first commit of a repository", func(t *testing.T) {
		f := guardFor(t, newEmptyRepo(t), safety.Policy{})
		f.stage("a.txt", "one\n")
		f.mustDo(safety.Commit(git.CommitOptions{Message: "first"}), never(t))

		_, err := f.undo()
		require.ErrorIs(t, err, safety.ErrUndoFirstCommit)
	})

	t.Run("a commit that was pushed", func(t *testing.T) {
		f, _, _ := remoteFixture(t)
		f.stage("a.txt", "two\n")
		f.mustDo(safety.Commit(git.CommitOptions{Message: "second"}), never(t))
		head := f.head()
		f.repo.Git("push", "--quiet")

		_, err := f.undo()
		require.ErrorIs(t, err, safety.ErrUndoPushed)
		assert.Equal(t, head, f.head())
	})

	t.Run("a commit followed by another made outside gitpal", func(t *testing.T) {
		f := newFixture(t)
		f.stage("a.txt", "two\n")
		f.mustDo(safety.Commit(git.CommitOptions{Message: "second"}), never(t))
		f.repo.WriteFile("b.txt", "by hand\n")
		head := f.repo.Commit("made with plain git")

		_, err := f.undo()
		require.ErrorIs(t, err, safety.ErrUndoMoved)
		assert.Equal(t, head, f.head())
	})

	t.Run("on another branch by now", func(t *testing.T) {
		f := newFixture(t)
		f.stage("a.txt", "two\n")
		f.mustDo(safety.Commit(git.CommitOptions{Message: "second"}), never(t))
		f.repo.Git("switch", "--quiet", "main")

		_, err := f.undo()
		require.ErrorIs(t, err, safety.ErrUndoMoved)
	})

	t.Run("declined", func(t *testing.T) {
		f := newFixture(t)
		f.stage("a.txt", "two\n")
		f.mustDo(safety.Commit(git.CommitOptions{Message: "second"}), never(t))
		head := f.head()

		_, err := f.guard.Undo(context.Background(), no, safety.DoOptions{})
		require.ErrorIs(t, err, safety.ErrDeclined)
		assert.Equal(t, head, f.head())

		_, err = f.guard.Undo(context.Background(), nil, safety.DoOptions{})
		require.ErrorIs(t, err, safety.ErrConfirmationRequired)

		_, err = f.guard.Undo(context.Background(), nil, safety.DoOptions{AssumeYes: true})
		require.NoError(t, err)
	})
}

func TestUndoAmend(t *testing.T) {
	f := newFixture(t)
	old := f.head()
	f.stage("a.txt", "amended\n")
	f.mustDo(safety.Commit(git.CommitOptions{Message: "first, reworded", Amend: true}), yes)
	require.NotEqual(t, old, f.head())

	f.mustUndo()

	assert.Equal(t, old, f.head())
	assert.Equal(t, "first", f.repo.Git("log", "-1", "--format=%s"))
	assert.Equal(t, "amended\n", f.repo.ReadFile("a.txt"))
}

func TestUndoDiscard(t *testing.T) {
	t.Run("brings the content back", func(t *testing.T) {
		f := newFixture(t)
		f.repo.WriteFile("a.txt", "precious work\n")
		f.repo.WriteFile("b.txt", "other work\n")
		f.mustDo(safety.Discard("a.txt"), yes)
		require.Equal(t, "one\n", f.repo.ReadFile("a.txt"))

		f.mustUndo()

		assert.Equal(t, "precious work\n", f.repo.ReadFile("a.txt"))
		assert.Equal(t, "other work\n", f.repo.ReadFile("b.txt"))
	})

	t.Run("refused when the file was edited again", func(t *testing.T) {
		f := newFixture(t)
		f.repo.WriteFile("a.txt", "precious work\n")
		f.mustDo(safety.Discard("a.txt"), yes)
		f.repo.WriteFile("a.txt", "newer work\n")

		_, err := f.undo()
		require.ErrorIs(t, err, safety.ErrUndoDirty)
		assert.Equal(t, "newer work\n", f.repo.ReadFile("a.txt"))
	})

	t.Run("refused when the backup has expired", func(t *testing.T) {
		f := newFixture(t)
		f.repo.WriteFile("a.txt", "precious work\n")
		res := f.mustDo(safety.Discard("a.txt"), yes)
		f.repo.Git("update-ref", "-d", res.Entry.WorktreeBackup)

		_, err := f.undo()
		require.ErrorIs(t, err, safety.ErrUndoNoBackup)
	})
}

func TestUndoResetHard(t *testing.T) {
	f := newFixture(t)
	first := f.head()
	f.repo.WriteFile("a.txt", "two\n")
	second := f.repo.Commit("second")
	f.repo.WriteFile("b.txt", "uncommitted\n")
	f.mustDo(safety.Reset(git.ResetHard, first), yes)
	require.Equal(t, first, f.head())
	require.Empty(t, f.short())

	t.Run("refused while there are new uncommitted changes", func(t *testing.T) {
		f.repo.WriteFile("a.txt", "in the way\n")
		_, err := f.undo()
		require.ErrorIs(t, err, safety.ErrUndoDirty)
		f.repo.Git("restore", "a.txt")
	})

	entry := f.mustUndo()

	assert.Equal(t, second, f.head(), "the commit is back")
	assert.Equal(t, "uncommitted\n", f.repo.ReadFile("b.txt"), "and so are the uncommitted changes")
	assert.Equal(t, first, f.repo.Git("rev-parse", entry.Backup), "where the undo started from is kept too")
}

func TestUndoResetSoft(t *testing.T) {
	f := newFixture(t)
	first := f.head()
	f.repo.WriteFile("a.txt", "two\n")
	second := f.repo.Commit("second")
	f.repo.WriteFile("b.txt", "uncommitted\n")
	f.mustDo(safety.Reset(git.ResetSoft, first), yes)

	f.mustUndo()

	assert.Equal(t, second, f.head())
	assert.Equal(t, "uncommitted\n", f.repo.ReadFile("b.txt"), "a soft reset never touched the working tree, nor does its undo")
}

func TestUndoDeleteBranch(t *testing.T) {
	f := newFixture(t)
	f.repo.Git("switch", "--quiet", "-c", "feature")
	f.repo.WriteFile("c.txt", "c\n")
	tip := f.repo.Commit("only here")
	f.repo.Git("switch", "--quiet", "work")
	f.mustDo(safety.DeleteBranch("feature", true), yes)

	t.Run("refused when the name is taken again", func(t *testing.T) {
		f.repo.Git("branch", "feature")
		_, err := f.undo()
		require.ErrorIs(t, err, safety.ErrUndoBranchExists)
		f.repo.Git("branch", "-D", "feature")
	})

	f.mustUndo()

	assert.Equal(t, tip, f.repo.Git("rev-parse", "feature"))
	assert.Equal(t, "work", f.repo.Git("branch", "--show-current"), "the current branch stays where it was")
}

func TestUndoPull(t *testing.T) {
	f, _, other := remoteFixture(t)
	before := f.head()
	other.WriteFile("b.txt", "theirs\n")
	other.Commit("their work")
	other.Git("push", "--quiet")
	f.mustDo(safety.Pull(git.PullOptions{}), never(t))
	require.NotEqual(t, before, f.head())

	t.Run("refused with uncommitted changes", func(t *testing.T) {
		f.repo.WriteFile("a.txt", "in the way\n")
		_, err := f.undo()
		require.ErrorIs(t, err, safety.ErrUndoDirty)
		f.repo.Git("restore", "a.txt")
	})

	f.mustUndo()

	assert.Equal(t, before, f.head(), "the branch is back where it was before the pull")
}

func TestUndoSwitch(t *testing.T) {
	f := newFixture(t)
	f.mustDo(safety.Switch("main"), never(t))

	f.mustUndo()

	assert.Equal(t, "work", f.repo.Git("branch", "--show-current"))
}

func TestUndoPushIsNotPossible(t *testing.T) {
	f, origin, _ := remoteFixture(t)
	f.repo.WriteFile("a.txt", "two\n")
	head := f.repo.Commit("second")
	f.mustDo(safety.Push(git.PushOptions{}), never(t))

	_, err := f.undo()

	require.ErrorIs(t, err, safety.ErrUndoPushed)
	assert.Equal(t, head, origin.Git("rev-parse", "refs/heads/work"))
	assert.Contains(t, err.Error(), "revert", "the way out is named")
}

func TestUndoSkipsFailedOperations(t *testing.T) {
	f := newFixture(t)
	before := f.head()
	f.stage("a.txt", "two\n")
	f.mustDo(safety.Commit(git.CommitOptions{Message: "second"}), never(t))
	// Nothing staged: this commit fails and changes nothing.
	_, err := f.do(safety.Commit(git.CommitOptions{Message: "empty"}), never(t))
	require.Equal(t, git.KindNothingToCommit, git.KindOf(err))

	f.mustUndo()

	assert.Equal(t, before, f.head(), "the undo took back the commit that happened")
}

func TestUndoOfAnOperationItDoesNotKnow(t *testing.T) {
	f := newFixture(t)
	_, err := f.guard.History().Append(history.Entry{Op: "from-the-future"})
	require.NoError(t, err)

	_, err = f.undo()
	require.ErrorIs(t, err, safety.ErrUndoNotSupported)
	assert.Contains(t, err.Error(), "from-the-future")
}
