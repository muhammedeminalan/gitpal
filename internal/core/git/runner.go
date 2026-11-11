// Package git wraps the system git executable.
//
// It is the only place in gitpal that starts git (spec §3, §17). Arguments are
// always passed as separate values to the process, never through a shell, so
// nothing a user types can be interpreted as a command.
package git

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// Timeouts.
const (
	// DefaultTimeout bounds commands that only touch the local repository.
	DefaultTimeout = 2 * time.Minute
	// NetworkTimeout bounds commands that talk to a remote.
	NetworkTimeout = 10 * time.Minute
	// killGrace is how long a killed git may keep its output pipes open
	// (through a helper process it started) before they are closed for it.
	killGrace = 5 * time.Second
)

// baseArgs precede every command so output can be parsed whatever the user's
// own git configuration says: no color codes, and non-ASCII paths printed
// as-is instead of as octal escapes.
var baseArgs = []string{"-c", "color.ui=never", "-c", "core.quotepath=false"}

// baseEnv is set for every command:
//
//   - LC_MESSAGES/LANGUAGE make git print English messages, which the error
//     table matches on. LC_ALL is deliberately left alone: forcing it to "C"
//     would also change the character encoding seen by the user's hooks.
//   - GIT_TERMINAL_PROMPT=0 makes git fail instead of waiting for a user name
//     or password on a terminal gitpal is not showing.
//   - GIT_EDITOR=true makes an unexpected editor request accept the default
//     text instead of opening an editor nobody can see.
var baseEnv = []string{
	"LC_MESSAGES=C",
	"LANGUAGE=en",
	"GIT_TERMINAL_PROMPT=0",
	"GIT_EDITOR=true",
}

// scrubbedEnv are variables git exports to hooks and `rebase --exec` commands
// to point child gits at the current repository's files (the list printed by
// `git rev-parse --local-env-vars`, minus the ones that carry configuration).
// Inheriting them would silently redirect gitpal to another repository.
var scrubbedEnv = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_COMMON_DIR",
	"GIT_CONFIG",
	"GIT_DIR",
	"GIT_GRAFT_FILE",
	"GIT_IMPLICIT_WORK_TREE",
	"GIT_INDEX_FILE",
	"GIT_NO_REPLACE_OBJECTS",
	"GIT_OBJECT_DIRECTORY",
	"GIT_PREFIX",
	"GIT_REPLACE_REF_BASE",
	"GIT_SHALLOW_FILE",
	"GIT_WORK_TREE",
}

// Runner starts git commands. The zero value is not usable; call NewRunner.
type Runner struct {
	bin      string
	baseEnv  func() []string
	env      []string
	timeout  time.Duration
	observer func(Event)
	logger   *slog.Logger
	// self is gitpal's executable, used as git's editor during an
	// interactive rebase; empty means the running program.
	self string
}

// Option configures a Runner.
type Option func(*Runner)

// WithBinary sets the git executable (default: "git" from PATH).
func WithBinary(path string) Option { return func(r *Runner) { r.bin = path } }

// WithBaseEnv replaces the process environment as the environment commands
// start from. Runner still removes scrubbed variables from it and adds its own.
func WithBaseEnv(env []string) Option {
	env = slices.Clone(env)
	return func(r *Runner) { r.baseEnv = func() []string { return env } }
}

// WithEnv adds "KEY=value" entries to the environment of every command. They
// take precedence over the base environment and over Runner's own defaults.
func WithEnv(env ...string) Option {
	return func(r *Runner) { r.env = append(r.env, env...) }
}

// WithTimeout sets the default time limit for a command. Zero or negative
// means no limit.
func WithTimeout(d time.Duration) Option { return func(r *Runner) { r.timeout = d } }

// WithObserver registers fn to be called after every command, successful or
// not. It powers `--explain`, the GUI command panel and the operation history.
func WithObserver(fn func(Event)) Option { return func(r *Runner) { r.observer = fn } }

// WithSelf names the executable git starts as its editor during an
// interactive rebase (see RunEditorHelper). The default is the running
// program.
func WithSelf(path string) Option { return func(r *Runner) { r.self = path } }

// WithLogger sets the logger for command metadata. Command output is never
// logged.
func WithLogger(logger *slog.Logger) Option { return func(r *Runner) { r.logger = logger } }

// NewRunner returns a Runner for the git found on PATH.
func NewRunner(opts ...Option) *Runner {
	r := &Runner{
		bin:     "git",
		baseEnv: os.Environ,
		timeout: DefaultTimeout,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Cmd is one git invocation.
type Cmd struct {
	// Dir is the directory to run in; empty means the current directory.
	Dir string
	// Args are git's arguments, without the leading "git".
	Args []string
	// Stdin is fed to the command. When nil the command reads nothing.
	Stdin []byte
	// Env adds "KEY=value" entries for this command only.
	Env []string
	// Timeout overrides the Runner's default; negative means no limit.
	Timeout time.Duration
	// Progress, when set, also receives what git writes to standard error
	// while it runs — line by line, with credentials redacted — so that a
	// long command (clone, fetch) can show its progress.
	Progress io.Writer
}

// Result is what a command produced.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Duration time.Duration
}

// Event describes a finished command to an observer.
type Event struct {
	Dir string
	// Args are the git arguments as given by the caller, credentials redacted.
	Args     []string
	ExitCode int
	Duration time.Duration
	// Err is nil when the command succeeded.
	Err error
}

// String renders the command the way it would be typed in a shell.
func (e Event) String() string { return commandLine(e.Args) }

// Run executes one git command and waits for it.
//
// A command that exits non-zero, cannot be started, times out or is canceled
// returns an *Error; the Result still carries whatever output was produced.
func (r *Runner) Run(ctx context.Context, cmd Cmd) (Result, error) {
	start := time.Now()
	res, err := r.run(ctx, cmd)
	res.Duration = time.Since(start)

	args := redactAll(cmd.Args)
	if r.observer != nil {
		r.observer(Event{Dir: cmd.Dir, Args: args, ExitCode: res.ExitCode, Duration: res.Duration, Err: err})
	}
	r.logger.Debug("git",
		slog.String("cmd", strings.Join(args, " ")),
		slog.String("dir", cmd.Dir),
		slog.Int("exit", res.ExitCode),
		slog.Duration("took", res.Duration),
		slog.String("kind", string(KindOf(err))),
	)
	return res, err
}

func (r *Runner) run(ctx context.Context, cmd Cmd) (Result, error) {
	fail := func(kind ErrorKind, err error) (Result, error) {
		return Result{ExitCode: -1}, &Error{Kind: kind, Args: redactAll(cmd.Args), ExitCode: -1, Err: err}
	}

	if cmd.Dir != "" {
		if _, err := os.Stat(cmd.Dir); err != nil {
			return fail(KindDirNotFound, err)
		}
	}

	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = r.timeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var stdout, stderr bytes.Buffer
	// The one audited place where a process is started: arguments go to git as
	// separate argv entries, never through a shell.
	c := exec.CommandContext(ctx, r.bin, append(slices.Clone(baseArgs), cmd.Args...)...) //nolint:gosec // G204: see above
	c.Dir = cmd.Dir
	c.Env = r.environ(cmd.Env)
	c.Stdout, c.Stderr = &stdout, &stderr
	if cmd.Progress != nil {
		progress := &redactingWriter{out: cmd.Progress}
		defer progress.flush()
		c.Stderr = io.MultiWriter(&stderr, progress)
	}
	c.WaitDelay = killGrace
	configureProcess(c)
	if cmd.Stdin != nil {
		c.Stdin = bytes.NewReader(cmd.Stdin)
	}

	err := c.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: -1}
	if c.ProcessState != nil {
		res.ExitCode = c.ProcessState.ExitCode()
	}
	if err == nil {
		return res, nil
	}

	gerr := &Error{
		Kind:     KindUnknown,
		Args:     redactAll(cmd.Args),
		ExitCode: res.ExitCode,
		Stderr:   redact(strings.TrimSpace(stderr.String())),
		Err:      err,
	}
	var exitErr *exec.ExitError
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		gerr.Kind, gerr.Err, gerr.ExitCode = KindTimeout, ctx.Err(), -1
	case errors.Is(ctx.Err(), context.Canceled):
		gerr.Kind, gerr.Err, gerr.ExitCode = KindCanceled, ctx.Err(), -1
	case errors.As(err, &exitErr):
		gerr.Kind, gerr.Err = classify(gerr.Stderr), nil
	case isNotFound(err):
		gerr.Kind = KindGitNotFound
	}
	res.ExitCode = gerr.ExitCode
	return res, gerr
}

// environ builds the child environment. Later entries win, so the order is:
// inherited (scrubbed), Runner defaults, Runner options, per-command.
func (r *Runner) environ(extra []string) []string {
	inherited := r.baseEnv()
	env := make([]string, 0, len(inherited)+len(baseEnv)+len(r.env)+len(extra))
	for _, entry := range inherited {
		name, _, _ := strings.Cut(entry, "=")
		if isScrubbed(name) {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, baseEnv...)
	env = append(env, r.env...)
	env = append(env, extra...)
	return env
}

func isScrubbed(name string) bool {
	// Environment names are case-insensitive on Windows.
	return slices.ContainsFunc(scrubbedEnv, func(s string) bool { return strings.EqualFold(s, name) })
}

func isNotFound(err error) bool {
	var lookupErr *exec.Error
	return errors.As(err, &lookupErr) || errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist)
}

// commandLine renders args as a shell command for display. It is never
// executed.
func commandLine(args []string) string {
	var b strings.Builder
	b.WriteString("git")
	for _, arg := range args {
		b.WriteByte(' ')
		b.WriteString(shellQuote(arg))
	}
	return b.String()
}

func shellQuote(s string) string {
	if s != "" && !strings.ContainsFunc(s, needsQuoting) && s[0] != '~' {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func needsQuoting(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	case strings.ContainsRune("-_./=:@%+,~", r):
		return false
	default:
		return true
	}
}

// redactingWriter passes text through one line at a time with credentials
// removed. git ends progress updates with "\r" and other lines with "\n";
// both end a line here.
type redactingWriter struct {
	out     io.Writer
	pending []byte
}

func (w *redactingWriter) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for {
		i := bytes.IndexAny(w.pending, "\r\n")
		if i < 0 {
			break
		}
		line := redact(string(w.pending[:i])) + string(w.pending[i])
		w.pending = w.pending[i+1:]
		// A failing progress display must not fail the command.
		_, _ = io.WriteString(w.out, line)
	}
	return len(p), nil
}

func (w *redactingWriter) flush() {
	if len(w.pending) > 0 {
		_, _ = io.WriteString(w.out, redact(string(w.pending)))
		w.pending = nil
	}
}
