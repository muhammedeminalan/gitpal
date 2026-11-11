// Package cli is gitpal's command line (spec §8).
//
// It is a thin layer: it parses flags, calls internal/core and prints the
// result. It never runs git itself.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/muhammedeminalan/gitpal/internal/core/ai"
	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/logging"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
	"github.com/muhammedeminalan/gitpal/internal/tui"
)

// Exit codes (spec §8).
const (
	// ExitOK: the command succeeded.
	ExitOK = 0
	// ExitError: the command failed.
	ExitError = 1
	// ExitCanceled: the user canceled.
	ExitCanceled = 2
	// ExitBlocked: the safety layer refused the operation.
	ExitBlocked = 3
)

// EnvLogLevel names the environment variable that sets the log level
// (debug, info, warn, error).
const EnvLogLevel = "GITPAL_LOG_LEVEL"

// Env is everything the command line takes from the outside world. Run uses
// nothing else, so tests can replace all of it.
type Env struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	GOOS    string
	Getenv  func(key string) string
	Getwd   func() (string, error)
	HomeDir func() (string, error)

	// NewRunner builds the git runner for a command.
	NewRunner func(opts ...git.Option) *git.Runner
	// Secrets is the keychain.
	Secrets secrets.Store
	// LookPath finds an executable on PATH.
	LookPath func(file string) (string, error)
	// IsTerminal reports whether w is an interactive terminal.
	IsTerminal func(w io.Writer) bool
	// Interactive reports whether a person is at the keyboard: standard
	// input and output are both a terminal. Questions are only asked then.
	Interactive func() bool
	// Pick and Dashboard show the interactive screens.
	Pick         func(ctx context.Context, opts tui.PickOptions) ([]string, error)
	Dashboard    func(ctx context.Context, opts tui.DashboardOptions) error
	RebaseEditor func(ctx context.Context, opts tui.RebaseOptions) ([]git.RebaseStep, error)
	// Editor opens a file in the user's editor.
	Editor func(ctx context.Context, path string) error
	// LaunchGUI starts the desktop app on a repository root ("" for none).
	LaunchGUI func(dir string) error
	// NewAI builds the AI provider.
	NewAI func(s ai.Settings) (ai.Provider, error)
	// ReadSecret reads a key without showing it; nil reads a plain line.
	ReadSecret func() (string, error)
}

// SystemEnv returns the real environment of the running process.
func SystemEnv() Env {
	return Env{
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		GOOS:       runtime.GOOS,
		Getenv:     os.Getenv,
		Getwd:      os.Getwd,
		HomeDir:    os.UserHomeDir,
		NewRunner:  git.NewRunner,
		Secrets:    secrets.Keyring(),
		LookPath:   exec.LookPath,
		IsTerminal: isTerminal,
		Interactive: func() bool {
			return term.IsTerminal(int(os.Stdin.Fd())) && isTerminal(os.Stdout) //nolint:gosec // see isTerminal
		},
		Pick:         tui.Pick,
		Dashboard:    tui.RunDashboard,
		RebaseEditor: tui.EditRebase,
		Editor:       openEditor,
		NewAI:        ai.New,
		LaunchGUI:    launchGUI,
		ReadSecret: func() (string, error) {
			fd := int(os.Stdin.Fd()) //nolint:gosec // a file descriptor always fits in an int
			if !term.IsTerminal(fd) {
				return "", errNotTerminal
			}
			b, err := term.ReadPassword(fd)
			return string(b), err
		},
	}
}

var errNotTerminal = errors.New("standard input is not a terminal")

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(int(file.Fd())) //nolint:gosec // a file descriptor always fits in an int
}

// Run executes gitpal with the given arguments (without the program name) and
// returns the process exit code. It never panics: an unexpected failure is
// written to the log and reported to the user as such (spec §11).
func Run(ctx context.Context, args []string, env Env) (code int) {
	a := &app{
		env:    env,
		cfgEnv: config.Env{GOOS: env.GOOS, Getenv: env.Getenv, HomeDir: env.HomeDir},
		logger: logging.Discard(),
	}
	closeLog := a.openLog()
	defer closeLog()
	a.lang = a.configuredLanguage()

	defer func() {
		if recovered := recover(); recovered != nil {
			a.logger.Error("panic",
				slog.String("value", fmt.Sprint(recovered)),
				slog.String("stack", string(debug.Stack())),
			)
			fmt.Fprintln(env.Stderr, i18n.T(a.lang, i18n.MsgPanic))
			if a.logPath != "" {
				fmt.Fprintln(env.Stderr, i18n.T(a.lang, i18n.MsgPanicLogged, a.logPath))
			}
			code = ExitError
		}
	}()

	root := a.newRootCmd()
	root.SetArgs(args)
	root.SetIn(env.Stdin)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)

	start := time.Now()
	cmd, err := root.ExecuteContextC(ctx)
	code = exitCode(err)
	a.logger.Info("command finished",
		slog.String("command", commandName(cmd)),
		slog.Int("exit", code),
		slog.Duration("took", time.Since(start)),
	)
	a.printError(err)
	return code
}

// app holds the state of one gitpal invocation.
type app struct {
	env    Env
	cfgEnv config.Env

	logger  *slog.Logger
	logPath string

	lang    i18n.Lang
	flags   globalFlags
	session *session

	// input reads answers from standard input; shared by every question so
	// that buffered lines are not lost between them.
	input *bufio.Reader
}

type globalFlags struct {
	json    bool
	noColor bool
	explain bool
	repo    string
	dryRun  bool
	yes     bool
}

// session is what a command works with: where it runs, the git runner and the
// effective settings.
type session struct {
	// dir is the directory the user is working in.
	dir string
	// repoRoot is the root of the repository containing dir; empty outside one.
	repoRoot string
	// repoErr is why there is no repository root.
	repoErr error
	runner  *git.Runner
	// loaded and configErr are the outcome of config.Load. A config error
	// does not stop the session: `doctor` must be able to report it.
	loaded    config.Loaded
	configErr error
}

// openLog points the logger at the log file. Failing to open it is not an
// error for the user: gitpal then simply does not log.
func (a *app) openLog() (closeLog func()) {
	noop := func() {}
	path, err := config.LogPath(a.cfgEnv)
	if err != nil {
		return noop
	}
	level, err := logging.ParseLevel(a.env.Getenv(EnvLogLevel))
	if err != nil {
		level = slog.LevelInfo
	}
	logger, closer, err := logging.Open(logging.Options{Path: path, Level: level})
	if err != nil {
		return noop
	}
	a.logger, a.logPath = logger, path
	return func() { _ = closer.Close() }
}

// configuredLanguage reads the interface language before commands are built,
// so help texts are in the right language. It falls back to the default
// silently; problems with the config file are reported when a command runs.
func (a *app) configuredLanguage() i18n.Lang {
	path, err := config.GlobalPath(a.cfgEnv)
	if err != nil {
		return i18n.Default
	}
	loaded, err := config.Load(config.LoadOptions{GlobalPath: path})
	if err != nil {
		return i18n.Default
	}
	return loaded.Config.Language
}

// startSession resolves the working directory, the repository and the
// settings for the command about to run.
func (a *app) startSession(ctx context.Context) error {
	dir, err := a.env.Getwd()
	if err != nil {
		return fmt.Errorf("find the current directory: %w", err)
	}
	if a.flags.repo != "" {
		if filepath.IsAbs(a.flags.repo) {
			dir = a.flags.repo
		} else {
			dir = filepath.Join(dir, a.flags.repo)
		}
	}

	opts := []git.Option{git.WithLogger(a.logger)}
	if a.flags.explain {
		opts = append(opts, git.WithObserver(a.explain))
	}
	s := &session{dir: dir, runner: a.env.NewRunner(opts...), loaded: config.Loaded{Config: config.Default()}}

	// Outside a repository (or without git) there is simply no repository
	// root; commands that need one report that themselves.
	if root, err := s.runner.TopLevel(ctx, dir); err == nil {
		s.repoRoot = root
	} else {
		s.repoErr = err
	}

	globalPath, err := config.GlobalPath(a.cfgEnv)
	if err != nil {
		s.configErr = err
	} else {
		s.loaded, s.configErr = config.Load(config.LoadOptions{GlobalPath: globalPath, RepoDir: s.repoRoot})
	}
	if s.configErr == nil {
		a.lang = s.loaded.Config.Language
	}
	a.session = s
	return nil
}

// explain prints a finished git command (`--explain`). It goes to stderr so
// that stdout stays clean for `--json`.
func (a *app) explain(event git.Event) {
	style := a.styleFor(a.env.Stderr)
	fmt.Fprintln(a.env.Stderr, style.dim("$ "+event.String()))
}

// exitError carries a specific exit code. A silent exitError has already been
// reported to the user by the command itself.
type exitError struct {
	code   int
	silent bool
	err    error
}

func (e *exitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit status %d", e.code)
}

func (e *exitError) Unwrap() error { return e.err }

func exitCode(err error) int {
	var coded *exitError
	switch {
	case err == nil:
		return ExitOK
	case errors.As(err, &coded):
		return coded.code
	case errors.Is(err, safety.ErrDeclined):
		return ExitCanceled
	case isBlocked(err):
		return ExitBlocked
	case errors.Is(err, context.Canceled):
		return ExitCanceled
	default:
		return ExitError
	}
}

func (a *app) printError(err error) {
	if err == nil {
		return
	}
	var coded *exitError
	if errors.As(err, &coded) && coded.silent {
		return
	}
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(a.env.Stderr, i18n.T(a.lang, i18n.MsgCanceled))
		return
	}
	style := a.styleFor(a.env.Stderr)
	fmt.Fprintf(a.env.Stderr, "%s %s\n", style.red(i18n.T(a.lang, i18n.MsgErrorPrefix)+":"), i18n.Describe(a.lang, err))
}

// commandName returns the command path without the program name, e.g.
// "branch new". Arguments are deliberately left out of the log.
func commandName(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	path := cmd.CommandPath()
	if _, rest, found := strings.Cut(path, " "); found {
		return rest
	}
	return path
}

// isBlocked reports whether the safety layer refused an operation: a rule
// stopped it, or it needed a confirmation nobody could give.
func isBlocked(err error) bool {
	var blocked *safety.BlockedError
	return errors.As(err, &blocked) || errors.Is(err, safety.ErrConfirmationRequired)
}
