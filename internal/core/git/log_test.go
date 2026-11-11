package git

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logRecord builds one commit record in the layout Log asks git for.
func logRecord(fields ...string) string {
	return strings.Join(fields, "\x00") + "\x00"
}

func TestLogParse(t *testing.T) {
	out := logRecord(
		"aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111", "aaaa111",
		"bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222 cccc3333cccc3333cccc3333cccc3333cccc3333",
		"Ayşe Yılmaz", "ayse@example.com", "2026-03-01T12:30:00+03:00",
		"Committer", "committer@example.com", "2026-03-02T09:00:00Z",
		"HEAD -> main, tag: v1.0, origin/main",
		"feat: 100% tamam %H", "ilk paragraf\n\nikinci paragraf\x1e\n\n",
	) + logRecord(
		"bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222", "bbbb222", "",
		"A", "a@example.com", "2025-12-31T23:59:59-08:00",
		"A", "a@example.com", "2025-12-31T23:59:59-08:00",
		"", "root", "",
	)

	commits, err := ParseLog([]byte(out))
	require.NoError(t, err)
	require.Len(t, commits, 2)

	first := commits[0]
	assert.Equal(t, "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111", first.Hash)
	assert.Equal(t, "aaaa111", first.ShortHash)
	assert.Equal(t, []string{
		"bbbb2222bbbb2222bbbb2222bbbb2222bbbb2222", "cccc3333cccc3333cccc3333cccc3333cccc3333",
	}, first.Parents)
	assert.True(t, first.IsMerge())
	assert.Equal(t, "Ayşe Yılmaz", first.AuthorName)
	assert.Equal(t, "ayse@example.com", first.AuthorEmail)
	assert.Equal(t, "Committer", first.CommitterName)
	assert.Equal(t, "committer@example.com", first.CommitterEmail)
	assert.Equal(t, []string{"HEAD -> main", "tag: v1.0", "origin/main"}, first.Refs)
	assert.Equal(t, "feat: 100% tamam %H", first.Subject)
	assert.Equal(t, "ilk paragraf\n\nikinci paragraf\x1e", first.Body)

	want := time.Date(2026, 3, 1, 12, 30, 0, 0, time.FixedZone("", 3*60*60))
	assert.True(t, first.AuthorDate.Equal(want))
	_, offset := first.AuthorDate.Zone()
	assert.Equal(t, 3*60*60, offset, "the commit's own UTC offset is kept")
	_, offset = first.CommitterDate.Zone()
	assert.Zero(t, offset)

	root := commits[1]
	assert.Empty(t, root.Parents)
	assert.False(t, root.IsMerge())
	assert.Empty(t, root.Refs)
	assert.Empty(t, root.Body)
	_, offset = root.AuthorDate.Zone()
	assert.Equal(t, -8*60*60, offset)
}

func TestLogParseEmpty(t *testing.T) {
	commits, err := ParseLog(nil)
	require.NoError(t, err)
	assert.Empty(t, commits)
}

// A record with the wrong number of fields is an error, not a commit with
// its fields shifted by one.
func TestLogParseRejectsMalformedOutput(t *testing.T) {
	good := []string{
		"aaaa", "aaa", "", "A", "a@example.com", "2026-03-01T12:30:00+03:00",
		"A", "a@example.com", "2026-03-01T12:30:00+03:00", "", "secret subject", "",
	}
	replace := func(i int, value string) string {
		fields := append([]string(nil), good...)
		fields[i] = value
		return logRecord(fields...)
	}
	for name, out := range map[string]string{
		"too few fields":     logRecord(good[:11]...),
		"one field too many": logRecord(append(append([]string(nil), good...), "extra")...),
		"bad author date":    replace(5, "yesterday"),
		"bad committer date": replace(8, "1700000000"),
		"empty hash":         replace(0, ""),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseLog([]byte(out))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}
