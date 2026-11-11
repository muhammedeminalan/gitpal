package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func diffParseOne(t *testing.T, raw string) FileDiff {
	t.Helper()
	files, err := ParseDiff([]byte(raw))
	require.NoError(t, err)
	require.Len(t, files, 1)
	return files[0]
}

func TestDiffParseModification(t *testing.T) {
	fd := diffParseOne(t, "diff --git a/src/main.go b/src/main.go\n"+
		"index 1111111..2222222 100644\n"+
		"--- a/src/main.go\n"+
		"+++ b/src/main.go\n"+
		"@@ -3,4 +3,5 @@ func main() {\n"+
		" \tctx := context.Background()\n"+
		"-\told()\n"+
		"+\tnew()\n"+
		"+\tmore()\n"+
		" \n"+
		" }\n"+
		"@@ -20 +21 @@\n"+
		"-last\n"+
		"+LAST\n")

	assert.Equal(t, "src/main.go", fd.Path)
	assert.Empty(t, fd.OldPath)
	assert.Equal(t, ChangeModified, fd.Change)
	assert.False(t, fd.Binary)
	assert.Equal(t, "100644", fd.OldMode)
	assert.Equal(t, "100644", fd.NewMode)
	require.Len(t, fd.Hunks, 2)

	first := fd.Hunks[0]
	assert.Equal(t, Hunk{
		OldStart: 3, OldLines: 4, NewStart: 3, NewLines: 5, Section: "func main() {",
		Lines: []DiffLine{
			{Kind: LineContext, Text: "\tctx := context.Background()", OldLine: 3, NewLine: 3},
			{Kind: LineDeleted, Text: "\told()", OldLine: 4},
			{Kind: LineAdded, Text: "\tnew()", NewLine: 4},
			{Kind: LineAdded, Text: "\tmore()", NewLine: 5},
			{Kind: LineContext, Text: "", OldLine: 5, NewLine: 6},
			{Kind: LineContext, Text: "}", OldLine: 6, NewLine: 7},
		},
	}, first)

	// "@@ -20 +21 @@": a missing count means one line.
	second := fd.Hunks[1]
	assert.Equal(t, 20, second.OldStart)
	assert.Equal(t, 1, second.OldLines)
	assert.Equal(t, 21, second.NewStart)
	assert.Equal(t, 1, second.NewLines)
	assert.Empty(t, second.Section)
	assert.Equal(t, []DiffLine{
		{Kind: LineDeleted, Text: "last", OldLine: 20},
		{Kind: LineAdded, Text: "LAST", NewLine: 21},
	}, second.Lines)
}

func TestDiffParseAddedAndDeleted(t *testing.T) {
	added := diffParseOne(t, "diff --git a/new.txt b/new.txt\n"+
		"new file mode 100755\n"+
		"index 0000000..3333333\n"+
		"--- /dev/null\n"+
		"+++ b/new.txt\n"+
		"@@ -0,0 +1,2 @@\n"+
		"+one\n"+
		"+two\n")
	assert.Equal(t, "new.txt", added.Path)
	assert.Equal(t, ChangeAdded, added.Change)
	assert.Empty(t, added.OldMode)
	assert.Equal(t, "100755", added.NewMode)
	assert.Equal(t, []DiffLine{
		{Kind: LineAdded, Text: "one", NewLine: 1},
		{Kind: LineAdded, Text: "two", NewLine: 2},
	}, added.Hunks[0].Lines)

	deleted := diffParseOne(t, "diff --git a/old.txt b/old.txt\n"+
		"deleted file mode 100644\n"+
		"index 3333333..0000000\n"+
		"--- a/old.txt\n"+
		"+++ /dev/null\n"+
		"@@ -1 +0,0 @@\n"+
		"-one\n")
	assert.Equal(t, "old.txt", deleted.Path, "a deletion is known by its old path")
	assert.Empty(t, deleted.OldPath)
	assert.Equal(t, ChangeDeleted, deleted.Change)
	assert.Equal(t, "100644", deleted.OldMode)
	assert.Empty(t, deleted.NewMode)
	assert.Equal(t, []DiffLine{{Kind: LineDeleted, Text: "one", OldLine: 1}}, deleted.Hunks[0].Lines)
}

func TestDiffParseRenameAndCopy(t *testing.T) {
	files, err := ParseDiff([]byte("diff --git a/old name.txt b/dir/new name.txt\n" +
		"similarity index 87%\n" +
		"rename from old name.txt\n" +
		"rename to dir/new name.txt\n" +
		"index 1111111..2222222 100644\n" +
		"--- a/old name.txt\t\n" +
		"+++ b/dir/new name.txt\t\n" +
		"@@ -1 +1 @@\n" +
		"-a\n" +
		"+b\n" +
		"diff --git a/src.txt b/copy.txt\n" +
		"similarity index 100%\n" +
		"copy from src.txt\n" +
		"copy to copy.txt\n" +
		"diff --git a/a b/b\n" +
		"similarity index 100%\n" +
		"rename from a\n" +
		"rename to b\n"))
	require.NoError(t, err)
	require.Len(t, files, 3)

	assert.Equal(t, "dir/new name.txt", files[0].Path)
	assert.Equal(t, "old name.txt", files[0].OldPath)
	assert.Equal(t, ChangeRenamed, files[0].Change)
	assert.Equal(t, 87, files[0].Similarity)
	assert.Len(t, files[0].Hunks, 1)

	assert.Equal(t, "copy.txt", files[1].Path)
	assert.Equal(t, "src.txt", files[1].OldPath)
	assert.Equal(t, ChangeCopied, files[1].Change)
	assert.Equal(t, 100, files[1].Similarity)
	assert.Empty(t, files[1].Hunks)

	assert.Equal(t, "b", files[2].Path)
	assert.Equal(t, "a", files[2].OldPath)
}

func TestDiffParseHeaderOnlyChanges(t *testing.T) {
	t.Run("mode change", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git a/run.sh b/run.sh\nold mode 100644\nnew mode 100755\n")
		assert.Equal(t, "run.sh", fd.Path)
		assert.Equal(t, ChangeModified, fd.Change)
		assert.Equal(t, "100644", fd.OldMode)
		assert.Equal(t, "100755", fd.NewMode)
		assert.Empty(t, fd.Hunks)
	})

	t.Run("file type change", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git a/link b/link\nold mode 100644\nnew mode 120000\n")
		assert.Equal(t, ChangeTypeChanged, fd.Change)
	})

	t.Run("empty file added", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git a/empty file b/empty file\nnew file mode 100644\nindex 0000000..e69de29\n")
		assert.Equal(t, "empty file", fd.Path)
		assert.Equal(t, ChangeAdded, fd.Change)
		assert.Empty(t, fd.Hunks)
	})

	// Nothing but the "diff --git" line names the file, and it is ambiguous
	// when the name contains " b/". Both names are the same, so it is split
	// in the middle, the way `git apply` does.
	t.Run("name containing the separator", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git a/x b/y b/x b/y\nold mode 100644\nnew mode 100755\n")
		assert.Equal(t, "x b/y", fd.Path)
	})

	t.Run("quoted name", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git \"a/tab\\there\" \"b/tab\\there\"\nold mode 100644\nnew mode 100755\n")
		assert.Equal(t, "tab\there", fd.Path)
	})
}

func TestDiffParseBinary(t *testing.T) {
	t.Run("summary line", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git a/logo.png b/logo.png\n"+
			"index 1111111..2222222 100644\n"+
			"Binary files a/logo.png and b/logo.png differ\n")
		assert.Equal(t, "logo.png", fd.Path)
		assert.True(t, fd.Binary)
		assert.Empty(t, fd.Hunks)
		assert.Equal(t, ChangeModified, fd.Change)
	})

	t.Run("new binary file", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git a/a b/c.bin b/a b/c.bin\n"+
			"new file mode 100644\n"+
			"index 0000000..2222222\n"+
			"Binary files /dev/null and b/a b/c.bin differ\n")
		assert.Equal(t, "a b/c.bin", fd.Path)
		assert.True(t, fd.Binary)
		assert.Equal(t, ChangeAdded, fd.Change)
	})

	t.Run("binary patch, followed by another file", func(t *testing.T) {
		files, err := ParseDiff([]byte("diff --git a/logo.png b/logo.png\n" +
			"index 1111111..2222222 100644\n" +
			"GIT binary patch\n" +
			"literal 10\n" +
			"RcmZQzU|?imU}9imVqjokVPIfjVBlb2\n" +
			"\n" +
			"literal 4\n" +
			"LcmZQzU|?im00jU6\n" +
			"\n" +
			"diff --git a/a.txt b/a.txt\n" +
			"index 1111111..2222222 100644\n" +
			"--- a/a.txt\n" +
			"+++ b/a.txt\n" +
			"@@ -1 +1 @@\n" +
			"-a\n" +
			"+b\n"))
		require.NoError(t, err)
		require.Len(t, files, 2)
		assert.True(t, files[0].Binary)
		assert.Empty(t, files[0].Hunks)
		assert.Equal(t, "a.txt", files[1].Path)
		assert.False(t, files[1].Binary)
	})
}

func TestDiffParseNoNewlineAtEndOfFile(t *testing.T) {
	fd := diffParseOne(t, "diff --git a/a.txt b/a.txt\n"+
		"index 1111111..2222222 100644\n"+
		"--- a/a.txt\n"+
		"+++ b/a.txt\n"+
		"@@ -1,2 +1,2 @@\n"+
		" same\n"+
		"-old end\n"+
		"\\ No newline at end of file\n"+
		"+new end\n"+
		"\\ No newline at end of file\n")

	assert.Equal(t, []DiffLine{
		{Kind: LineContext, Text: "same", OldLine: 1, NewLine: 1},
		{Kind: LineDeleted, Text: "old end", OldLine: 2},
		{Kind: LineNoNewline, Text: "No newline at end of file"},
		{Kind: LineAdded, Text: "new end", NewLine: 2},
		{Kind: LineNoNewline, Text: "No newline at end of file"},
	}, fd.Hunks[0].Lines)
}

func TestDiffParseAwkwardContent(t *testing.T) {
	// Lines of the file that look like parts of a diff are still content: the
	// hunk header says how many lines belong to the hunk.
	t.Run("content that looks like diff syntax", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git a/notes.md b/notes.md\n"+
			"index 1111111..2222222 100644\n"+
			"--- a/notes.md\n"+
			"+++ b/notes.md\n"+
			"@@ -1,3 +1,4 @@\n"+
			"--- a/not-a-header\n"+
			"+++ b/not-a-header\n"+
			"+diff --git a/fake b/fake\n"+
			" @@ -1 +1 @@\n"+
			" \\ not a marker\n")

		require.Len(t, fd.Hunks, 1)
		assert.Equal(t, "notes.md", fd.Path)
		assert.Equal(t, []DiffLine{
			{Kind: LineDeleted, Text: "-- a/not-a-header", OldLine: 1},
			{Kind: LineAdded, Text: "++ b/not-a-header", NewLine: 1},
			{Kind: LineAdded, Text: "diff --git a/fake b/fake", NewLine: 2},
			{Kind: LineContext, Text: "@@ -1 +1 @@", OldLine: 2, NewLine: 3},
			{Kind: LineContext, Text: "\\ not a marker", OldLine: 3, NewLine: 4},
		}, fd.Hunks[0].Lines)
	})

	// diff.suppressBlankEmpty prints a blank context line as a truly empty
	// line, without the leading space.
	t.Run("empty line as blank context", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git a/a b/a\n--- a/a\n+++ b/a\n@@ -1,3 +1,3 @@\n-x\n+y\n\n end\n")
		assert.Equal(t, []DiffLine{
			{Kind: LineDeleted, Text: "x", OldLine: 1},
			{Kind: LineAdded, Text: "y", NewLine: 1},
			{Kind: LineContext, Text: "", OldLine: 2, NewLine: 2},
			{Kind: LineContext, Text: "end", OldLine: 3, NewLine: 3},
		}, fd.Hunks[0].Lines)
	})

	t.Run("carriage returns are content", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git a/a b/a\n--- a/a\n+++ b/a\n@@ -1 +1 @@\n-dos\r\n+unix\n")
		assert.Equal(t, "dos\r", fd.Hunks[0].Lines[0].Text)
		assert.Equal(t, "unix", fd.Hunks[0].Lines[1].Text)
	})

	t.Run("quoted paths", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git \"a/new\\nline \\303\\247 \\\"q\\\" \\\\.txt\" \"b/new\\nline \\303\\247 \\\"q\\\" \\\\.txt\"\n"+
			"index 1111111..2222222 100644\n"+
			"--- \"a/new\\nline \\303\\247 \\\"q\\\" \\\\.txt\"\n"+
			"+++ \"b/new\\nline \\303\\247 \\\"q\\\" \\\\.txt\"\n"+
			"@@ -1 +1 @@\n-a\n+b\n")
		assert.Equal(t, "new\nline ç \"q\" \\.txt", fd.Path)
	})

	t.Run("quoted name with a space keeps git's trailing tab", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git \"a/t\\tx y\" \"b/t\\tx y\"\n"+
			"--- \"a/t\\tx y\"\t\n"+
			"+++ \"b/t\\tx y\"\t\n"+
			"@@ -1 +1 @@\n-a\n+b\n")
		assert.Equal(t, "t\tx y", fd.Path)
	})

	t.Run("quoted rename", func(t *testing.T) {
		fd := diffParseOne(t, "diff --git \"a/a\\tb\" b/plain\n"+
			"similarity index 100%\n"+
			"rename from \"a\\tb\"\n"+
			"rename to plain\n")
		assert.Equal(t, "a\tb", fd.OldPath)
		assert.Equal(t, "plain", fd.Path)
	})
}

func TestDiffParseEmptyInput(t *testing.T) {
	files, err := ParseDiff(nil)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestDiffParseLargeDiffKeepsLineNumbers(t *testing.T) {
	var b strings.Builder
	b.WriteString("diff --git a/big.txt b/big.txt\n--- a/big.txt\n+++ b/big.txt\n@@ -1,5000 +1,5000 @@\n")
	for range 5000 {
		b.WriteString(" line\n")
	}

	fd := diffParseOne(t, b.String())

	lines := fd.Hunks[0].Lines
	require.Len(t, lines, 5000)
	assert.Equal(t, 5000, lines[4999].OldLine)
	assert.Equal(t, 5000, lines[4999].NewLine)
}

// Malformed input is an error, never a panic, and the error does not quote
// file content.
func TestDiffParseRejectsMalformedInput(t *testing.T) {
	const header = "diff --git a/a b/a\n--- a/a\n+++ b/a\n"
	for name, raw := range map[string]string{
		"text before the first file":     "secret garbage\n",
		"hunk without a file":            "@@ -1 +1 @@\n-secret\n+b\n",
		"truncated hunk":                 header + "@@ -1,3 +1,3 @@\n secret\n",
		"hunk header without numbers":    header + "@@ -x +y @@\n",
		"hunk header without end":        header + "@@ -1 +1\n-a\n+b\n",
		"hunk header with a huge number": header + "@@ -99999999999999999999 +1 @@\n",
		"hunk header with negative":      header + "@@ --1 +1 @@\n",
		"hunk header too short":          header + "@@\n",
		"bad line marker in a hunk":      header + "@@ -1,2 +1,2 @@\n a\n?secret\n",
		"more lines than announced":      header + "@@ -1 +1 @@\n-a\n+b\n+secret\n",
		"header that names no file":      "diff --git \n",
		"unterminated quoted path":       "diff --git \"a/x b/x\n",
		"bad escape in a quoted path":    "diff --git \"a/\\q\" \"b/\\q\"\n",
		"bad similarity":                 "diff --git a/a b/b\nsimilarity index lots%\n",
		"combined diff":                  "diff --cc a\nindex 1,2..3\n",
		"names that cannot be split":     "diff --git a/one b/two\nold mode 100644\nnew mode 100755\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseDiff([]byte(raw))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestDiffParseStat(t *testing.T) {
	stats, err := diffParseStat([]byte("3\t1\tsrc/main.go\x00" +
		"-\t-\tlogo.png\x00" +
		"0\t0\t\x00old name.txt\x00new name.txt\x00" +
		"12\t0\ttab\there.txt\x00"))
	require.NoError(t, err)

	assert.Equal(t, []FileStat{
		{Path: "src/main.go", Added: 3, Deleted: 1},
		{Path: "logo.png", Binary: true},
		{Path: "new name.txt", OldPath: "old name.txt"},
		{Path: "tab\there.txt", Added: 12},
	}, stats)

	empty, err := diffParseStat(nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	for name, raw := range map[string]string{
		"missing fields":       "3\tsecret\x00",
		"counts not numbers":   "x\ty\tsecret\x00",
		"rename without paths": "0\t0\t\x00only-one\x00",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := diffParseStat([]byte(raw))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}
