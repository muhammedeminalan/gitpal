package history_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/history"
)

func newStore(t *testing.T) (*history.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gitpal", "history.jsonl")
	return history.Open(path), path
}

func TestEmptyStore(t *testing.T) {
	store, path := newStore(t)

	entries, err := store.Entries()
	require.NoError(t, err)
	assert.Empty(t, entries)

	_, ok, err := store.LastUndoable()
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err), "reading does not create the file")
}

func TestAppendAndRead(t *testing.T) {
	store, path := newStore(t)
	when := time.Date(2026, 11, 11, 8, 30, 0, 0, time.UTC)

	first, err := store.Append(history.Entry{
		Time: when, Op: "discard", Branch: "main", Before: "aaa", After: "aaa",
		WorktreeBackup: "refs/gitpal/backup-worktree/x", Paths: []string{"çalışma dosyası.txt", "a\nb"},
	})
	require.NoError(t, err)
	second, err := store.Append(history.Entry{Time: when.Add(time.Second), Op: "commit", Before: "aaa", After: "bbb"})
	require.NoError(t, err)

	assert.NotEmpty(t, first.ID)
	assert.NotEqual(t, first.ID, second.ID)

	entries, err := store.Entries()
	require.NoError(t, err)
	assert.Equal(t, []history.Entry{first, second}, entries, "oldest first, exactly as written")

	info, err := os.Stat(path)
	require.NoError(t, err)
	if info.Mode().Perm() != 0o666 { // Windows reports no Unix permissions
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "the history is the user's own")
	}
}

func TestAppendFillsInTimeAndID(t *testing.T) {
	store, _ := newStore(t)

	a, err := store.Append(history.Entry{Op: "commit"})
	require.NoError(t, err)
	b, err := store.Append(history.Entry{Op: "commit"})
	require.NoError(t, err)

	assert.False(t, a.Time.IsZero())
	assert.NotEqual(t, a.ID, b.ID, "two entries in the same instant still differ")
}

func TestAppendRequiresAnOperation(t *testing.T) {
	store, _ := newStore(t)
	_, err := store.Append(history.Entry{})
	assert.Error(t, err)
}

// A damaged line (a crash half way through a write, a manual edit) costs that
// line, not the history.
func TestDamagedLinesAreSkipped(t *testing.T) {
	store, path := newStore(t)
	good, err := store.Append(history.Entry{Op: "commit", After: "bbb"})
	require.NoError(t, err)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString("{\"op\": \"half a li\n\nnot json at all\n{\"id\":\"no-op\"}\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	later, err := store.Append(history.Entry{Op: "push"})
	require.NoError(t, err)

	entries, err := store.Entries()
	require.NoError(t, err)
	assert.Equal(t, []history.Entry{good, later}, entries)
}

func TestLastUndoable(t *testing.T) {
	store, _ := newStore(t)
	commit, err := store.Append(history.Entry{Op: "commit", After: "bbb"})
	require.NoError(t, err)
	discard, err := store.Append(history.Entry{Op: "discard"})
	require.NoError(t, err)

	got, ok, err := store.LastUndoable()
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, discard.ID, got.ID)

	t.Run("a failed operation changed nothing to undo", func(t *testing.T) {
		_, err := store.Append(history.Entry{Op: "push", Failed: "push_rejected"})
		require.NoError(t, err)

		got, ok, err := store.LastUndoable()
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, discard.ID, got.ID)
	})

	t.Run("an undone operation is skipped, and so is the undo itself", func(t *testing.T) {
		_, err := store.Append(history.Entry{Op: "undo", Undoes: discard.ID})
		require.NoError(t, err)

		got, ok, err := store.LastUndoable()
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, commit.ID, got.ID)
	})

	t.Run("nothing left", func(t *testing.T) {
		_, err := store.Append(history.Entry{Op: "undo", Undoes: commit.ID})
		require.NoError(t, err)

		_, ok, err := store.LastUndoable()
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestConcurrentAppends(t *testing.T) {
	store, _ := newStore(t)
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_, err := store.Append(history.Entry{Op: "commit", Paths: []string{"a fairly long path/to/make/the/line/longer.txt"}})
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	entries, err := store.Entries()
	require.NoError(t, err)
	assert.Len(t, entries, 50, "no line was torn or lost")
	ids := map[string]bool{}
	for _, e := range entries {
		ids[e.ID] = true
	}
	assert.Len(t, ids, 50)
}

func TestUnreadableStore(t *testing.T) {
	dir := t.TempDir()
	// A directory where the file should be.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "history.jsonl"), 0o700))
	store := history.Open(filepath.Join(dir, "history.jsonl"))

	_, err := store.Entries()
	require.Error(t, err)
	_, err = store.Append(history.Entry{Op: "commit"})
	require.Error(t, err)
}

func TestHistoryKeepsOnlyTheNewestEntriesOnceItGrowsLarge(t *testing.T) {
	store, path := newStore(t)
	store.MaxBytes = 2000
	var last history.Entry
	for i := range 40 {
		e, err := store.Append(history.Entry{Op: "commit", Branch: "b", After: string(rune('a' + i%26))})
		require.NoError(t, err)
		last = e
	}
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.LessOrEqual(t, info.Size(), int64(2000))

	entries, err := store.Entries()
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	assert.Less(t, len(entries), 40)
	assert.Equal(t, last.ID, entries[len(entries)-1].ID, "the newest entry is always kept")
	undoable, ok, err := store.LastUndoable()
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, last.ID, undoable.ID)
}
