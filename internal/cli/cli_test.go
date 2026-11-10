package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eminalan/gitpal/internal/cli"
	"github.com/eminalan/gitpal/internal/core/git"
	"github.com/eminalan/gitpal/internal/core/git/gittest"
	"github.com/eminalan/gitpal/internal/core/secrets"
	"github.com/eminalan/gitpal/internal/core/secrets/secretstest"
)

// harness runs the CLI the way main does, but against a throw-away home
// directory, an isolated git and an in-memory keychain.
type harness struct {
	t       *testing.T
	env     cli.Env
	vars    map[string]string
	home    string
	cwd     string
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	secrets *secretstest.Memory
	gitOpts []gittest.Option
}

// platforms are the operating systems gitpal ships for. Where files live and
// what a terminal can display differ between them, so tests that touch either
// run once per platform — on whatever machine the suite runs on, because the
// platform is part of the injected cli.Env. A Windows-only mistake is then
// caught on a developer's Mac instead of in CI.
var platforms = []string{"linux", "darwin", "windows"}

// onEveryPlatform runs fn once per supported platform, each with a fresh harness.
func onEveryPlatform(t *testing.T, fn func(t *testing.T, h *harness)) {
	t.Helper()
	for _, goos := range platforms {
		t.Run(goos, func(t *testing.T) { fn(t, newHarnessOn(t, goos)) })
	}
}

// newHarness returns a harness for the platform the tests run on.
func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessOn(t, runtime.GOOS)
}

// newHarnessOn returns a harness in which gitpal believes it runs on goos.
func newHarnessOn(t *testing.T, goos string) *harness {
	t.Helper()
	h := &harness{t: t, home: gittest.TempDir(t), cwd: gittest.TempDir(t), secrets: secretstest.NewMemory()}
	h.vars = map[string]string{"GITPAL_CONFIG": filepath.Join(h.home, "config.yaml")}
	h.setStateDir(filepath.Join(h.home, "state"))
	h.env = cli.Env{
		Stdin:   strings.NewReader(""),
		Stdout:  &h.stdout,
		Stderr:  &h.stderr,
		GOOS:    goos,
		Getenv:  func(key string) string { return h.vars[key] },
		Getwd:   func() (string, error) { return h.cwd, nil },
		HomeDir: func() (string, error) { return h.home, nil },
		NewRunner: func(opts ...git.Option) *git.Runner {
			return gittest.NewRunner(t, append(h.gitOpts, gittest.WithGitOptions(opts...))...)
		},
		Secrets:    h.secrets,
		LookPath:   func(string) (string, error) { return "", errors.New("not installed") },
		IsTerminal: func(io.Writer) bool { return false },
	}
	return h
}

func (h *harness) run(args ...string) int {
	h.t.Helper()
	h.stdout.Reset()
	h.stderr.Reset()
	return cli.Run(context.Background(), args, h.env)
}

func (h *harness) writeConfig(content string) {
	h.t.Helper()
	require.NoError(h.t, os.WriteFile(h.vars["GITPAL_CONFIG"], []byte(content), 0o600))
}

// setStateDir points gitpal's per-user state directory, where the log lives,
// at dir. Every platform names it differently (DECISIONS A-006), and a harness
// that sets only one name silently tests nothing on the others.
func (h *harness) setStateDir(dir string) {
	h.vars["XDG_STATE_HOME"] = dir // Linux, macOS
	h.vars["LOCALAPPDATA"] = dir   // Windows
}

// logPath is where gitpal must write its log in this harness.
func (h *harness) logPath() string {
	return filepath.Join(h.home, "state", "gitpal", "gitpal.log")
}

func (h *harness) logFile() string {
	h.t.Helper()
	data, err := os.ReadFile(h.logPath())
	require.NoError(h.t, err)
	return string(data)
}

// logFiles returns every gitpal.log below the throw-away home directory,
// wherever it ended up.
func (h *harness) logFiles() []string {
	h.t.Helper()
	var found []string
	err := filepath.WalkDir(h.home, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() == "gitpal.log" {
			found = append(found, path)
		}
		return err
	})
	require.NoError(h.t, err)
	return found
}

type doctorJSON struct {
	Healthy bool `json:"healthy"`
	Summary struct {
		OK   int `json:"ok"`
		Warn int `json:"warn"`
		Fail int `json:"fail"`
	} `json:"summary"`
	Checks []struct {
		ID      string   `json:"id"`
		Status  string   `json:"status"`
		Message string   `json:"message"`
		Hint    string   `json:"hint"`
		Details []string `json:"details"`
	} `json:"checks"`
}

func (h *harness) doctorJSON(args ...string) (doctorJSON, int) {
	h.t.Helper()
	code := h.run(append([]string{"doctor", "--json"}, args...)...)
	var out doctorJSON
	require.NoError(h.t, json.Unmarshal(h.stdout.Bytes(), &out), "stdout must be exactly one JSON document: %s", h.stdout.String())
	return out, code
}

func statuses(out doctorJSON) map[string]string {
	m := map[string]string{}
	for _, check := range out.Checks {
		m[check.ID] = check.Status
	}
	return m
}

// A fresh machine with git and an identity but no GitHub token or AI key: the
// core workflow is ready, so doctor succeeds and only warns.
func TestDoctorOnAFreshMachine(t *testing.T) {
	h := newHarness(t)

	code := h.run("doctor")

	assert.Equal(t, cli.ExitOK, code)
	out := h.stdout.String()
	assert.Contains(t, out, "✓ git ")
	assert.Contains(t, out, "✓ Git kimliği: Test User <test@example.com>")
	assert.Contains(t, out, h.vars["GITPAL_CONFIG"])
	assert.Contains(t, out, "! GitHub: token yok")
	assert.Contains(t, out, "! AI: anthropic anahtarı yok")
	assert.Contains(t, out, "3 tamam, 2 uyarı, 0 hata")
	assert.NotContains(t, out, "\x1b[", "no color codes when the output is not a terminal")
	assert.Empty(t, h.stderr.String())
}

func TestDoctorJSON(t *testing.T) {
	h := newHarness(t)

	out, code := h.doctorJSON()

	assert.Equal(t, cli.ExitOK, code)
	assert.True(t, out.Healthy)
	assert.Equal(t, map[string]string{
		"git.version":  "ok",
		"git.identity": "ok",
		"config":       "ok",
		"github.auth":  "warn",
		"ai.key":       "warn",
	}, statuses(out))
	assert.Equal(t, 3, out.Summary.OK)
	assert.Equal(t, 2, out.Summary.Warn)
	assert.Equal(t, 0, out.Summary.Fail)
}

func TestDoctorFailsWhenCommitsWouldFail(t *testing.T) {
	h := newHarness(t)
	h.gitOpts = []gittest.Option{gittest.WithoutIdentity()}

	code := h.run("doctor")

	assert.Equal(t, cli.ExitError, code)
	out := h.stdout.String()
	assert.Contains(t, out, "✗ Git kimliği eksik: user.name, user.email")
	assert.Contains(t, out, "git config --global user.name")
	assert.Contains(t, out, "1 hata")
}

func TestDoctorReportsAnInvalidConfigAndStillChecksTheRest(t *testing.T) {
	h := newHarness(t)
	h.writeConfig("pull:\n  strategy: yolo\n")

	out, code := h.doctorJSON()

	assert.Equal(t, cli.ExitError, code)
	assert.False(t, out.Healthy)
	got := statuses(out)
	assert.Equal(t, "fail", got["config"])
	assert.Equal(t, "ok", got["git.version"], "one broken thing must not hide the others")
	assert.Equal(t, "ok", got["git.identity"])
	for _, check := range out.Checks {
		if check.ID == "config" {
			text := check.Message + strings.Join(check.Details, " ") + check.Hint
			assert.Contains(t, text, "pull.strategy")
			assert.Contains(t, text, h.vars["GITPAL_CONFIG"])
		}
	}
}

func TestDoctorSpeaksTheConfiguredLanguage(t *testing.T) {
	h := newHarness(t)
	h.writeConfig("language: en\n")

	code := h.run("doctor")

	assert.Equal(t, cli.ExitOK, code)
	assert.Contains(t, h.stdout.String(), "Git identity: Test User <test@example.com>")
	assert.Contains(t, h.stdout.String(), "3 ok, 2 warnings, 0 failed")
}

func TestDoctorSeesStoredSecretsWithoutPrintingThem(t *testing.T) {
	onEveryPlatform(t, func(t *testing.T, h *harness) {
		require.NoError(t, h.secrets.Set(secrets.GitHubToken, "ghp_supersecrettoken"))
		require.NoError(t, h.secrets.Set(secrets.AIKey("anthropic"), "sk-ant-supersecretkey"))
		h.vars["GITPAL_LOG_LEVEL"] = "debug"

		out, code := h.doctorJSON("--explain")

		assert.Equal(t, cli.ExitOK, code)
		assert.Equal(t, "ok", statuses(out)["github.auth"])
		assert.Equal(t, "ok", statuses(out)["ai.key"])
		for name, text := range map[string]string{"stdout": h.stdout.String(), "stderr": h.stderr.String(), "log": h.logFile()} {
			assert.NotContains(t, text, "ghp_supersecrettoken", "%s leaks the GitHub token", name)
			assert.NotContains(t, text, "sk-ant-supersecretkey", "%s leaks the AI key", name)
		}
	})
}

func TestDoctorPicksUpTheRepositoryConfig(t *testing.T) {
	h := newHarness(t)
	repo := gittest.NewRepo(t)
	repo.WriteFile(".gitpal.yaml", "ai:\n  consent_given: true\n")
	h.cwd = repo.MkdirAll("nested/dir")

	out, code := h.doctorJSON()

	assert.Equal(t, cli.ExitOK, code)
	assert.Equal(t, "warn", statuses(out)["config"])
	for _, check := range out.Checks {
		if check.ID == "config" {
			details := strings.Join(check.Details, "\n")
			assert.Contains(t, details, repo.Path(".gitpal.yaml"))
			assert.Contains(t, details, "ai.consent_given")
		}
	}
}

// --explain is the transparency promise of the spec: the user can always see
// which git commands ran. They go to stderr so --json output stays clean.
func TestExplainShowsTheGitCommandsThatRan(t *testing.T) {
	h := newHarness(t)

	_, code := h.doctorJSON("--explain")

	assert.Equal(t, cli.ExitOK, code)
	assert.Contains(t, h.stderr.String(), "$ git version\n")
	assert.Contains(t, h.stderr.String(), "$ git config --get --null -- user.name\n")
}

func TestWithoutExplainNoCommandsAreShown(t *testing.T) {
	h := newHarness(t)

	h.run("doctor")

	assert.NotContains(t, h.stderr.String(), "$ git")
}

func TestColorOnlyGoesToTerminals(t *testing.T) {
	// What a terminal that understands escape codes looks like to gitpal on
	// every platform. Without it the cases below would not be colored on
	// Windows for a reason of their own (the classic console, tested
	// separately), and the "no color" cases would pass without testing their flag.
	capable := map[string]string{"TERM": "xterm-256color"}
	tests := []struct {
		name      string
		args      []string
		vars      map[string]string // on top of the capable terminal's
		terminal  bool
		wantColor bool
	}{
		{name: "terminal", args: []string{"doctor"}, terminal: true, wantColor: true},
		{name: "pipe", args: []string{"doctor"}, terminal: false, wantColor: false},
		{name: "--no-color", args: []string{"doctor", "--no-color"}, terminal: true, wantColor: false},
		{name: "NO_COLOR", args: []string{"doctor"}, vars: map[string]string{"NO_COLOR": "1"}, terminal: true, wantColor: false},
		{name: "dumb terminal", args: []string{"doctor"}, vars: map[string]string{"TERM": "dumb"}, terminal: true, wantColor: false},
		{name: "json is never colored", args: []string{"doctor", "--json"}, terminal: true, wantColor: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			onEveryPlatform(t, func(t *testing.T, h *harness) {
				maps.Copy(h.vars, capable)
				maps.Copy(h.vars, tt.vars)
				h.env.IsTerminal = func(io.Writer) bool { return tt.terminal }

				h.run(tt.args...)

				assert.Equal(t, tt.wantColor, strings.Contains(h.stdout.String(), "\x1b["))
			})
		})
	}
}

// The classic Windows console prints escape codes as garbage, so on Windows
// color needs a terminal that announces itself: Windows Terminal sets
// WT_SESSION, Git Bash and similar shells set TERM.
func TestColorOnWindowsNeedsACapableTerminal(t *testing.T) {
	tests := []struct {
		name      string
		vars      map[string]string
		wantColor bool
	}{
		{name: "classic console", wantColor: false},
		{name: "Windows Terminal", vars: map[string]string{"WT_SESSION": "2f1d7a5e-0000-4000-8000-000000000000"}, wantColor: true},
		{name: "shell that sets TERM", vars: map[string]string{"TERM": "xterm"}, wantColor: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarnessOn(t, "windows")
			maps.Copy(h.vars, tt.vars)
			h.env.IsTerminal = func(io.Writer) bool { return true }

			h.run("doctor")

			assert.Equal(t, tt.wantColor, strings.Contains(h.stdout.String(), "\x1b["))
		})
	}
}

func TestVersionFlag(t *testing.T) {
	h := newHarness(t)

	code := h.run("--version")

	assert.Equal(t, cli.ExitOK, code)
	assert.True(t, strings.HasPrefix(h.stdout.String(), "gitpal "), "got %q", h.stdout.String())
}

func TestNoArgumentsShowsHelp(t *testing.T) {
	h := newHarness(t)

	code := h.run()

	assert.Equal(t, cli.ExitOK, code)
	assert.Contains(t, h.stdout.String(), "doctor")
	assert.Contains(t, h.stdout.String(), "--explain")
}

func TestUnknownCommandFails(t *testing.T) {
	h := newHarness(t)

	code := h.run("frobnicate")

	assert.Equal(t, cli.ExitError, code)
	assert.Contains(t, h.stderr.String(), "frobnicate")
	assert.Empty(t, h.stdout.String())
}

func TestUnexpectedArgumentsFail(t *testing.T) {
	h := newHarness(t)

	code := h.run("doctor", "extra")

	assert.Equal(t, cli.ExitError, code)
	assert.NotEmpty(t, h.stderr.String())
}

type explodingStore struct{ secrets.Store }

func (explodingStore) Get(string) (string, error) { panic("keychain exploded") }

// Spec §11: a panic is caught, the user gets a pointer to the error report,
// and the process exits cleanly instead of dumping a stack trace.
func TestPanicBecomesAnErrorReport(t *testing.T) {
	onEveryPlatform(t, func(t *testing.T, h *harness) {
		h.env.Secrets = explodingStore{}

		code := h.run("doctor")

		assert.Equal(t, cli.ExitError, code)
		assert.Contains(t, h.stderr.String(), "beklenmeyen bir hata")
		assert.Contains(t, h.stderr.String(), h.logPath())
		assert.NotContains(t, h.stderr.String(), "goroutine", "the stack trace belongs in the log, not on screen")
		log := h.logFile()
		assert.Contains(t, log, "keychain exploded")
		assert.Contains(t, log, "goroutine")
	})
}

func TestLogRecordsCommandsButNotOutput(t *testing.T) {
	onEveryPlatform(t, func(t *testing.T, h *harness) {
		h.vars["GITPAL_LOG_LEVEL"] = "debug"

		h.run("doctor")

		log := h.logFile()
		assert.Contains(t, log, "command=doctor")
		assert.Contains(t, log, "exit=0")
		assert.Contains(t, log, `cmd="config --get --null -- user.email"`)
		assert.NotContains(t, log, "test@example.com", "command output must not be logged")
	})
}

func TestUnwritableLogDoesNotBreakCommands(t *testing.T) {
	onEveryPlatform(t, func(t *testing.T, h *harness) {
		blocker := filepath.Join(h.home, "blocker")
		require.NoError(t, os.WriteFile(blocker, nil, 0o600))
		h.setStateDir(blocker)

		code := h.run("doctor")

		assert.Equal(t, cli.ExitOK, code)
		assert.Contains(t, h.stdout.String(), "3 tamam")
		assert.Empty(t, h.logFiles(), "the log directory is blocked, so no log may be written anywhere else either")
	})
}
