package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBranchSuggestName(t *testing.T) {
	for input, want := range map[string]string{
		"feat/Yeni Özellik!":           "feat/yeni-ozellik",
		"Giriş ekranı düzeltmesi":      "giris-ekrani-duzeltmesi",
		"ÇĞIİÖŞÜ çğıiöşü":              "cgiiosu-cgiiosu",
		"IŞIK":                         "isik",
		"fix: login_button  crash #42": "fix-login-button-crash-42",
		"  spaces   around  ":          "spaces-around",
		"release/v1.2.3":               "release/v1.2.3",
		"a..b":                         "a.b",
		"x.lock":                       "x",
		"x.lock.lock":                  "x",
		"feat/.hidden/-dash-/end.":     "feat/hidden/dash/end",
		"//double//slash//":            "double/slash",
		"--force":                      "force",
		"-D":                           "d",
		"@{-1}":                        "1",
		"HEAD":                         "head",
		"refs/heads/x":                 "heads/x",
		"refs/refs/x":                  "x",
		"a~b^c:d?e*f[g\\h":             "a-b-c-d-e-f-g-h",
		"café naïve über straße":       "cafe-naive-uber-strasse",
		"日本語 branch":                   "branch",
		"emoji 🚀 launch":               "emoji-launch",
		"tab\tand\nnewline":            "tab-and-newline",
		"":                             "",
		"   ":                          "",
		"!!!":                          "",
		"日本語":                          "",
		"/":                            "",
		".lock":                        "",
		"...":                          "",
	} {
		assert.Equal(t, want, SuggestBranchName(input), "input %q", input)
	}
}

func TestBranchPrefixesAreUsable(t *testing.T) {
	require.NotEmpty(t, BranchPrefixes)
	for _, prefix := range BranchPrefixes {
		assert.Equal(t, prefix+"x", SuggestBranchName(prefix+"x"), "a prefix survives suggestion unchanged")
	}
}

func TestBranchParseTrack(t *testing.T) {
	for track, want := range map[string]struct {
		ahead, behind int
		gone          bool
	}{
		"":                     {},
		"[ahead 1]":            {ahead: 1},
		"[behind 12]":          {behind: 12},
		"[ahead 3, behind 14]": {ahead: 3, behind: 14},
		"[gone]":               {gone: true},
	} {
		ahead, behind, gone, err := branchParseTrack(track)
		require.NoError(t, err, track)
		assert.Equal(t, want.ahead, ahead, track)
		assert.Equal(t, want.behind, behind, track)
		assert.Equal(t, want.gone, gone, track)
	}

	for _, bad := range []string{"ahead 1", "[ahead x]", "[sideways 2]", "[ahead]"} {
		_, _, _, err := branchParseTrack(bad)
		assert.Error(t, err, bad)
	}
}

func TestBranchParseListRejectsMalformedOutput(t *testing.T) {
	_, err := branchParseList([]byte("refs/heads/main\x00only-two-fields\n"), nil)
	require.Error(t, err)

	_, err = branchParseList([]byte("refs/heads/main\x00abc\x00*\x00\x00\x00\x00not-a-date\x00subject\n"), nil)
	require.Error(t, err)

	_, err = branchParseList([]byte("refs/heads/main\x00abc\x00*\x00\x00[sideways]\x00\x002026-03-01T10:00:00+03:00\x00s\n"), nil)
	require.Error(t, err)
}

// The remote a tracking branch belongs to is the longest configured remote
// name its ref starts with: remote names may contain slashes themselves.
func TestBranchParseListFindsTheRemote(t *testing.T) {
	line := func(ref string) string {
		return ref + "\x00abc\x00 \x00\x00\x00\x002026-03-01T10:00:00+03:00\x00subject\n"
	}
	out := line("refs/remotes/origin/main") + line("refs/remotes/team/fork/feat/x") +
		line("refs/remotes/team/other") + line("refs/remotes/removed/old")

	branches, err := branchParseList([]byte(out), []string{"origin", "team", "team/fork"})
	require.NoError(t, err)
	require.Len(t, branches, 4)

	assert.Equal(t, "origin", branches[0].RemoteName)
	assert.Equal(t, "origin/main", branches[0].Name)
	assert.True(t, branches[0].Remote)
	assert.Equal(t, "team/fork", branches[1].RemoteName)
	assert.Equal(t, "team/fork/feat/x", branches[1].Name)
	assert.Equal(t, "team", branches[2].RemoteName)
	assert.Equal(t, "removed", branches[3].RemoteName, "a remote that is no longer configured: the first segment")
}
