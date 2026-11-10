package git

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner returns a Runner whose "git" is this test binary in the given mode.
func fakeRunner(mode string, opts ...Option) *Runner {
	opts = append([]Option{WithBinary(os.Args[0]), WithEnv(fakeGitEnv + "=" + mode)}, opts...)
	return NewRunner(opts...)
}

func runReport(t *testing.T, r *Runner, cmd Cmd) fakeReport {
	t.Helper()
	res, err := r.Run(context.Background(), cmd)
	require.NoError(t, err)
	var report fakeReport
	require.NoError(t, json.Unmarshal(res.Stdout, &report), "stdout: %s", res.Stdout)
	return report
}

// userArgs strips the options the runner puts in front of every command.
func userArgs(t *testing.T, args []string) []string {
	t.Helper()
	require.GreaterOrEqual(t, len(args), len(baseArgs))
	require.Equal(t, baseArgs, args[:len(baseArgs)])
	return args[len(baseArgs):]
}

// Arguments reach git exactly as given: no shell ever sees them, so nothing a
// user types (a branch name, a path, a message) can become a second command.
func TestRunPassesArgumentsVerbatim(t *testing.T) {
	hostile := []string{
		"commit", "-m", "it's \"quoted\"",
		"$(touch pwned)", "; rm -rf /", "a b\tc", "`id`", "--", "*.go", "",
	}

	report := runReport(t, fakeRunner("report"), Cmd{Args: hostile})

	assert.Equal(t, hostile, userArgs(t, report.Args))
}

func TestRunStartsInTheGivenDirectory(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	report := runReport(t, fakeRunner("report"), Cmd{Dir: dir, Args: []string{"status"}})

	got, err := filepath.EvalSymlinks(report.Dir)
	require.NoError(t, err)
	assert.Equal(t, dir, got)
}

func TestRunFeedsStdin(t *testing.T) {
	report := runReport(t, fakeRunner("report"), Cmd{Args: []string{"commit", "-F", "-"}, Stdin: []byte("feat: mesaj\n\ngövde")})

	assert.Equal(t, "feat: mesaj\n\ngövde", report.Stdin)
}

func TestRunGivesNoStdinByDefault(t *testing.T) {
	report := runReport(t, fakeRunner("report"), Cmd{Args: []string{"status"}})

	assert.Empty(t, report.Stdin, "the child must not inherit the terminal's stdin")
}

// git's messages are parsed to explain errors, so they must be English no
// matter how the user's shell is localized; and git must never stop to ask a
// question on a terminal gitpal is not showing.
func TestRunMakesGitParseableAndNonInteractive(t *testing.T) {
	t.Setenv("LANGUAGE", "tr")
	t.Setenv("LC_MESSAGES", "tr_TR.UTF-8")
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("GIT_EDITOR", "vim")

	report := runReport(t, fakeRunner("report"), Cmd{Args: []string{"status"}})

	assert.Equal(t, "en", report.Env["LANGUAGE"])
	assert.Equal(t, "C", report.Env["LC_MESSAGES"])
	assert.Equal(t, "0", report.Env["GIT_TERMINAL_PROMPT"])
	assert.Equal(t, "true", report.Env["GIT_EDITOR"])
}

// When gitpal is started from a git hook or `git rebase --exec`, git exports
// variables that point every child git at the outer repository's files.
// Passing them on would make gitpal operate on the wrong repository.
func TestRunScrubsRepositoryRedirectingEnvironment(t *testing.T) {
	t.Setenv("GIT_DIR", "/outer/.git")
	t.Setenv("GIT_WORK_TREE", "/outer")
	t.Setenv("GIT_INDEX_FILE", "/outer/.git/index")
	t.Setenv("GIT_OBJECT_DIRECTORY", "/outer/.git/objects")
	t.Setenv("GIT_COMMON_DIR", "/outer/.git")

	report := runReport(t, fakeRunner("report"), Cmd{Args: []string{"status"}})

	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR"} {
		assert.NotContains(t, report.Env, key)
	}
}

func TestRunKeepsTheUsersOwnEnvironment(t *testing.T) {
	t.Setenv("GIT_SSH_COMMAND", "ssh -i ~/.ssh/work")

	report := runReport(t, fakeRunner("report"), Cmd{Args: []string{"fetch"}})

	assert.Equal(t, "ssh -i ~/.ssh/work", report.Env["GIT_SSH_COMMAND"])
}

func TestEnvironmentPrecedence(t *testing.T) {
	t.Setenv("GITPAL_TEST_VALUE", "from-process")

	t.Run("runner env beats the process", func(t *testing.T) {
		r := fakeRunner("report", WithEnv("GITPAL_TEST_VALUE=from-runner"))
		report := runReport(t, r, Cmd{Args: []string{"status"}})
		assert.Equal(t, "from-runner", report.Env["GITPAL_TEST_VALUE"])
	})

	t.Run("command env beats the runner", func(t *testing.T) {
		r := fakeRunner("report", WithEnv("GITPAL_TEST_VALUE=from-runner"))
		report := runReport(t, r, Cmd{Args: []string{"status"}, Env: []string{"GITPAL_TEST_VALUE=from-cmd"}})
		assert.Equal(t, "from-cmd", report.Env["GITPAL_TEST_VALUE"])
	})

	t.Run("a caller can re-enable prompts", func(t *testing.T) {
		report := runReport(t, fakeRunner("report"), Cmd{Args: []string{"push"}, Env: []string{"GIT_TERMINAL_PROMPT=1"}})
		assert.Equal(t, "1", report.Env["GIT_TERMINAL_PROMPT"])
	})
}

func TestRunReturnsOutputAndTypedErrorOnFailure(t *testing.T) {
	r := fakeRunner("fail", WithEnv(
		"GITPAL_FAKE_STDOUT=partial output",
		"GITPAL_FAKE_STDERR=fatal: something broke\n",
		"GITPAL_FAKE_EXIT=3",
	))

	res, err := r.Run(context.Background(), Cmd{Args: []string{"merge", "topic"}})

	var gerr *Error
	require.ErrorAs(t, err, &gerr)
	assert.Equal(t, 3, gerr.ExitCode)
	assert.Equal(t, "fatal: something broke", gerr.Stderr)
	assert.Equal(t, []string{"merge", "topic"}, gerr.Args)
	assert.Equal(t, KindUnknown, gerr.Kind)
	assert.Equal(t, 3, res.ExitCode)
	assert.Equal(t, "partial output", string(res.Stdout))
	assert.Contains(t, err.Error(), "git merge topic")
	assert.Contains(t, err.Error(), "fatal: something broke")
}

func TestExitCodeTellsExpectedFailuresApart(t *testing.T) {
	r := fakeRunner("fail", WithEnv("GITPAL_FAKE_EXIT=1"))

	_, err := r.Run(context.Background(), Cmd{Args: []string{"diff", "--quiet"}})

	code, ok := ExitCode(err)
	assert.True(t, ok)
	assert.Equal(t, 1, code)

	_, ok = ExitCode(nil)
	assert.False(t, ok)
	_, ok = ExitCode(context.Canceled)
	assert.False(t, ok)
}

func TestRunKillsCommandsThatOutliveTheirTimeout(t *testing.T) {
	start := time.Now()

	_, err := fakeRunner("hang").Run(context.Background(), Cmd{Args: []string{"fetch"}, Timeout: 200 * time.Millisecond})

	assert.Equal(t, KindTimeout, KindOf(err))
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 20*time.Second, "the child must be killed, not waited for")
}

func TestRunnerDefaultTimeoutApplies(t *testing.T) {
	r := fakeRunner("hang", WithTimeout(200*time.Millisecond))

	_, err := r.Run(context.Background(), Cmd{Args: []string{"fetch"}})

	assert.Equal(t, KindTimeout, KindOf(err))
}

func TestRunStopsWhenTheCallerCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	start := time.Now()

	_, err := fakeRunner("hang").Run(ctx, Cmd{Args: []string{"clone"}})

	assert.Equal(t, KindCanceled, KindOf(err))
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), 20*time.Second)
}

func TestRunReportsAMissingGitBinary(t *testing.T) {
	r := NewRunner(WithBinary(filepath.Join(t.TempDir(), "no-such-git")))

	_, err := r.Run(context.Background(), Cmd{Args: []string{"version"}})

	assert.Equal(t, KindGitNotFound, KindOf(err))
}

func TestRunReportsAMissingDirectoryAsSuch(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")

	_, err := fakeRunner("report").Run(context.Background(), Cmd{Dir: missing, Args: []string{"status"}})

	require.Error(t, err)
	assert.NotEqual(t, KindGitNotFound, KindOf(err), "a missing directory is not a missing git")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestObserverSeesEveryCommandWithSecretsRedacted(t *testing.T) {
	var events []Event
	r := fakeRunner("fail",
		WithEnv("GITPAL_FAKE_EXIT=1", "GITPAL_FAKE_STDERR=fatal: unable to access 'https://emin:hunter2@example.com/r.git/'"),
		WithObserver(func(e Event) { events = append(events, e) }),
	)

	_, err := r.Run(context.Background(), Cmd{Dir: "", Args: []string{"push", "https://emin:hunter2@example.com/r.git", "main"}})

	require.Len(t, events, 1)
	event := events[0]
	assert.Equal(t, []string{"push", "https://[REDACTED]@example.com/r.git", "main"}, event.Args)
	assert.Equal(t, 1, event.ExitCode)
	assert.Error(t, event.Err)
	assert.Equal(t, "git push 'https://[REDACTED]@example.com/r.git' main", event.String())

	var gerr *Error
	require.ErrorAs(t, err, &gerr)
	assert.NotContains(t, err.Error(), "hunter2")
	assert.NotContains(t, gerr.Stderr, "hunter2")
	assert.NotContains(t, gerr.Args, "https://emin:hunter2@example.com/r.git")
}

func TestObserverSeesSuccessfulCommands(t *testing.T) {
	var events []Event
	r := fakeRunner("report", WithObserver(func(e Event) { events = append(events, e) }))

	_, err := r.Run(context.Background(), Cmd{Args: []string{"status", "--porcelain=v2"}})
	require.NoError(t, err)

	require.Len(t, events, 1)
	assert.Equal(t, "git status --porcelain=v2", events[0].String())
	assert.Equal(t, 0, events[0].ExitCode)
	assert.NoError(t, events[0].Err)
}

func TestRedact(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "user and password", in: "https://emin:hunter2@github.com/a/b.git", want: "https://[REDACTED]@github.com/a/b.git"},
		{name: "token as user name", in: "https://ghp_abc123@github.com/a/b.git", want: "https://[REDACTED]@github.com/a/b.git"},
		{name: "inside a sentence", in: "fatal: unable to access 'http://u:p@host/x': 403", want: "fatal: unable to access 'http://[REDACTED]@host/x': 403"},
		{name: "several urls", in: "https://a:b@x.io and https://c:d@y.io", want: "https://[REDACTED]@x.io and https://[REDACTED]@y.io"},
		{name: "ssh user without password is not a secret", in: "ssh://git@github.com/a/b.git", want: "ssh://git@github.com/a/b.git"},
		{name: "ssh with password", in: "ssh://emin:pw@host/a.git", want: "ssh://[REDACTED]@host/a.git"},
		{name: "scp-like address", in: "git@github.com:a/b.git", want: "git@github.com:a/b.git"},
		{name: "plain https", in: "https://github.com/a/b.git", want: "https://github.com/a/b.git"},
		{name: "extra header option", in: "http.extraheader=Authorization: Basic Zm9v", want: "http.extraheader=[REDACTED]"},
		{name: "authorization header in output", in: "> Authorization: Bearer abc.def\n> Accept: */*", want: "> Authorization: [REDACTED]\n> Accept: */*"},
		{name: "nothing to hide", in: "status --porcelain=v2", want: "status --porcelain=v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, redact(tt.in))
		})
	}
}

func TestEventStringQuotesOnlyWhatNeedsQuoting(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "plain", args: []string{"status", "-z"}, want: "git status -z"},
		{name: "space", args: []string{"add", "--", "my file.txt"}, want: "git add -- 'my file.txt'"},
		{name: "single quote", args: []string{"add", "it's"}, want: `git add 'it'\''s'`},
		{name: "empty", args: []string{"commit", "-m", ""}, want: "git commit -m ''"},
		{name: "glob", args: []string{"add", "*.go"}, want: "git add '*.go'"},
		{name: "ref syntax stays readable", args: []string{"rev-list", "--count", "HEAD..origin/main", "feature/x@{upstream}"}, want: "git rev-list --count HEAD..origin/main 'feature/x@{upstream}'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Event{Args: tt.args}.String())
		})
	}
}

func TestErrorTruncatesLongOutput(t *testing.T) {
	long := ""
	for range 400 {
		long += "error: line of hook output\n"
	}
	err := &Error{Kind: KindUnknown, Args: []string{"commit"}, ExitCode: 1, Stderr: long}

	assert.Less(t, len(err.Error()), 2500)
	assert.Equal(t, long, err.Stderr, "the full output stays available to callers")
}
