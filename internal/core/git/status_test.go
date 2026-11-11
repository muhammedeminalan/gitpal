package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusRecords joins porcelain v2 records the way `-z` prints them.
func statusRecords(records ...string) []byte {
	return []byte(strings.Join(records, "\x00") + "\x00")
}

const (
	statusTestOID  = "1f9d2c3b4a5968778695a4b3c2d1e0f9a8b7c6d5"
	statusTestZero = "0000000000000000000000000000000000000000"
)

func TestStatusParseBranchHeaders(t *testing.T) {
	t.Run("branch with upstream", func(t *testing.T) {
		st, err := ParseStatus(statusRecords(
			"# branch.oid "+statusTestOID,
			"# branch.head main",
			"# branch.upstream origin/main",
			"# branch.ab +2 -3",
		))
		require.NoError(t, err)
		assert.Equal(t, BranchStatus{
			Head: "main", OID: statusTestOID, Upstream: "origin/main", Ahead: 2, Behind: 3,
		}, st.Branch)
		assert.Empty(t, st.Entries)
		assert.True(t, st.Clean())
	})

	t.Run("unborn", func(t *testing.T) {
		st, err := ParseStatus(statusRecords("# branch.oid (initial)", "# branch.head main"))
		require.NoError(t, err)
		assert.Equal(t, BranchStatus{Head: "main", Unborn: true}, st.Branch)
	})

	t.Run("detached", func(t *testing.T) {
		st, err := ParseStatus(statusRecords("# branch.oid "+statusTestOID, "# branch.head (detached)"))
		require.NoError(t, err)
		assert.Equal(t, BranchStatus{Detached: true, OID: statusTestOID}, st.Branch)
	})

	t.Run("upstream without ahead/behind is gone", func(t *testing.T) {
		st, err := ParseStatus(statusRecords(
			"# branch.oid "+statusTestOID, "# branch.head feat", "# branch.upstream origin/feat",
		))
		require.NoError(t, err)
		assert.True(t, st.Branch.UpstreamGone)
		assert.Equal(t, "origin/feat", st.Branch.Upstream)
	})

	t.Run("headers it does not know are skipped", func(t *testing.T) {
		st, err := ParseStatus(statusRecords("# branch.oid "+statusTestOID, "# branch.head main", "# stash 3"))
		require.NoError(t, err)
		assert.Equal(t, "main", st.Branch.Head)
	})

	t.Run("empty output", func(t *testing.T) {
		st, err := ParseStatus(nil)
		require.NoError(t, err)
		assert.True(t, st.Clean())
	})
}

func TestStatusParseEntries(t *testing.T) {
	st, err := ParseStatus(statusRecords(
		"1 .M N... 100644 100644 100644 "+statusTestOID+" "+statusTestOID+" çalışma dosyası.txt",
		"1 A. N... 000000 100644 100644 "+statusTestZero+" "+statusTestOID+" dir/new file.go",
		"1 MM S.M. 160000 160000 160000 "+statusTestOID+" "+statusTestOID+" vendor/lib",
		"2 R. N... 100644 100644 100644 "+statusTestOID+" "+statusTestOID+" R87 new name.txt", "old name.txt",
		"2 .C N... 100644 100644 100644 "+statusTestOID+" "+statusTestOID+" C100 copy.txt", "source.txt",
		"u UU N... 100644 100644 100644 100644 "+statusTestOID+" "+statusTestOID+" "+statusTestOID+" both changed.txt",
		"u DU N... 100644 000000 100644 100644 "+statusTestOID+" "+statusTestZero+" "+statusTestOID+" deleted by us",
		"? new\nline.txt",
		"? -dash",
		"! build/out.bin",
	))
	require.NoError(t, err)

	assert.Equal(t, []StatusEntry{
		{Kind: EntryChanged, Path: "çalışma dosyası.txt", Index: StateUnmodified, Worktree: StateModified},
		{Kind: EntryChanged, Path: "dir/new file.go", Index: StateAdded, Worktree: StateUnmodified},
		{Kind: EntryChanged, Path: "vendor/lib", Index: StateModified, Worktree: StateModified, Submodule: true},
		{
			Kind: EntryRenamed, Path: "new name.txt", OrigPath: "old name.txt",
			Index: StateRenamed, Worktree: StateUnmodified, Score: 87,
		},
		{
			Kind: EntryRenamed, Path: "copy.txt", OrigPath: "source.txt",
			Index: StateUnmodified, Worktree: StateCopied, Score: 100,
		},
		{Kind: EntryUnmerged, Path: "both changed.txt", Index: StateUnmerged, Worktree: StateUnmerged},
		{Kind: EntryUnmerged, Path: "deleted by us", Index: StateDeleted, Worktree: StateUnmerged},
		{Kind: EntryUntracked, Path: "new\nline.txt"},
		{Kind: EntryUntracked, Path: "-dash"},
		{Kind: EntryIgnored, Path: "build/out.bin"},
	}, st.Entries)

	assert.False(t, st.Clean())
	assert.Equal(t, []string{"dir/new file.go", "vendor/lib", "new name.txt"}, statusPaths(st.Staged()))
	assert.Equal(t, []string{"çalışma dosyası.txt", "vendor/lib", "copy.txt"}, statusPaths(st.Unstaged()))
	assert.Equal(t, []string{"new\nline.txt", "-dash"}, statusPaths(st.Untracked()))
	assert.Equal(t, []string{"both changed.txt", "deleted by us"}, statusPaths(st.Conflicted()))
}

func TestStatusEntryPredicates(t *testing.T) {
	unmerged := StatusEntry{Kind: EntryUnmerged, Index: StateUnmerged, Worktree: StateUnmerged}
	assert.True(t, unmerged.Conflicted())
	assert.False(t, unmerged.Staged(), "an unmerged entry is neither staged nor unstaged")
	assert.False(t, unmerged.Unstaged())

	untracked := StatusEntry{Kind: EntryUntracked}
	assert.False(t, untracked.Staged())
	assert.False(t, untracked.Unstaged())
	assert.False(t, untracked.Conflicted())
}

func TestStatusIgnoredFilesDoNotMakeItDirty(t *testing.T) {
	st, err := ParseStatus(statusRecords("! build/out.bin"))
	require.NoError(t, err)
	assert.True(t, st.Clean())
}

func TestStatusParseRejectsMalformedOutput(t *testing.T) {
	for name, out := range map[string][]byte{
		"unknown record type":       statusRecords("x something secret"),
		"ordinary record too short": statusRecords("1 .M N... 100644 100644"),
		"rename without source":     statusRecords("2 R. N... 100644 100644 100644 " + statusTestOID + " " + statusTestOID + " R87 new.txt"),
		"rename record too short":   statusRecords("2 R. N... 100644", "old.txt"),
		"rename with bad score":     statusRecords("2 R. N... 100644 100644 100644 "+statusTestOID+" "+statusTestOID+" Rxx new.txt", "old.txt"),
		"unmerged record too short": statusRecords("u UU N... 100644 100644"),
		"state field too long":      statusRecords("1 MMM N... 100644 100644 100644 " + statusTestOID + " " + statusTestOID + " a"),
		"ahead/behind not numbers":  statusRecords("# branch.ab +? -?"),
		"ahead/behind one number":   statusRecords("# branch.ab +1"),
		"untracked without a path":  statusRecords("? "),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseStatus(out)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret", "the error must not quote repository content")
		})
	}
}

func statusPaths(entries []StatusEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	return paths
}
