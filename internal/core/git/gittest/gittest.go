// Package gittest builds throw-away git repositories for tests.
//
// Everything it creates lives under the test's temporary directory and runs
// with an isolated git configuration: no user or system config is read, no
// GIT_* variable from the developer's shell is inherited, and the identity is
// fixed. Tests therefore never read or touch real repositories or the real
// ~/.gitconfig (spec §17).
package gittest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
)

// The identity every test commit is made with.
const (
	UserName  = "Test User"
	UserEmail = "test@example.com"
)

type settings struct {
	identity bool
	gitOpts  []git.Option
}

// Option adjusts the isolated environment.
type Option func(*settings)

// WithoutIdentity leaves user.name and user.email unset and stops git from
// guessing them, so commits fail the way they do on a fresh machine.
func WithoutIdentity() Option { return func(s *settings) { s.identity = false } }

// WithGitOptions passes extra options to the git.Runner.
func WithGitOptions(opts ...git.Option) Option {
	return func(s *settings) { s.gitOpts = append(s.gitOpts, opts...) }
}

// TempDir returns a new temporary directory with symbolic links resolved, so
// it compares equal to the paths git prints (on macOS /var is a link).
func TempDir(t testing.TB) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("gittest: resolve temp dir: %v", err)
	}
	return dir
}

// Env returns the variables that isolate git from the machine it runs on. Use
// it with git.WithEnv when a test needs a Runner that still inherits the
// process environment; NewRunner is the right choice otherwise.
func Env(t testing.TB, opts ...Option) []string {
	t.Helper()
	s := settings{identity: true}
	for _, opt := range opts {
		opt(&s)
	}

	home := TempDir(t)
	config := "[init]\n\tdefaultBranch = main\n" +
		"[commit]\n\tgpgsign = false\n" +
		"[tag]\n\tgpgsign = false\n" +
		"[core]\n\tautocrlf = false\n" +
		"[protocol \"file\"]\n\tallow = always\n"
	if s.identity {
		config += "[user]\n\tname = " + UserName + "\n\temail = " + UserEmail + "\n"
	} else {
		config += "[user]\n\tuseConfigOnly = true\n"
	}
	configPath := filepath.Join(home, "gitconfig")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("gittest: write git config: %v", err)
	}

	return []string{
		"HOME=" + home,
		"USERPROFILE=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, "xdg"),
		"GIT_CONFIG_GLOBAL=" + configPath,
		"GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
	}
}

// NewRunner returns a git.Runner that is isolated from the machine it runs on.
func NewRunner(t testing.TB, opts ...Option) *git.Runner {
	t.Helper()
	s := settings{identity: true}
	for _, opt := range opts {
		opt(&s)
	}

	var base []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(name), "GIT_") {
			continue
		}
		base = append(base, entry)
	}

	gitOpts := append([]git.Option{git.WithBaseEnv(base), git.WithEnv(Env(t, opts...)...)}, s.gitOpts...)
	return git.NewRunner(gitOpts...)
}

// Repo is a throw-away repository.
type Repo struct {
	t testing.TB
	// Dir is the working tree root (the repository itself when bare).
	Dir string
	// Runner is the isolated runner the repository was created with.
	Runner *git.Runner
}

// NewRepo creates an empty repository whose first branch will be "main".
func NewRepo(t testing.TB, opts ...Option) *Repo {
	t.Helper()
	r := &Repo{t: t, Dir: TempDir(t), Runner: NewRunner(t, opts...)}
	r.Git("init", "--quiet", "--initial-branch=main")
	return r
}

// NewBareRepo creates an empty bare repository, to play the part of a remote.
func NewBareRepo(t testing.TB, opts ...Option) *Repo {
	t.Helper()
	r := &Repo{t: t, Dir: TempDir(t), Runner: NewRunner(t, opts...)}
	r.Git("init", "--quiet", "--bare", "--initial-branch=main")
	return r
}

// Clone clones origin into a new directory; the clone's "origin" remote points
// at it. The clone shares origin's isolated configuration.
func Clone(t testing.TB, origin *Repo) *Repo {
	t.Helper()
	r := &Repo{t: t, Dir: filepath.Join(TempDir(t), "clone"), Runner: origin.Runner}
	r.gitIn(filepath.Dir(r.Dir), "clone", "--quiet", "--", origin.Dir, r.Dir)
	return r
}

// Git runs git in the repository and returns its standard output without the
// trailing newline. A failing command fails the test.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return r.gitIn(r.Dir, args...)
}

func (r *Repo) gitIn(dir string, args ...string) string {
	r.t.Helper()
	res, err := r.Runner.Run(context.Background(), git.Cmd{Dir: dir, Args: args})
	if err != nil {
		r.t.Fatalf("gittest: %v", err)
	}
	return strings.TrimRight(string(res.Stdout), "\n")
}

// Path returns the absolute path of a file given relative to the repository
// root with forward slashes.
func (r *Repo) Path(rel string) string {
	return filepath.Join(r.Dir, filepath.FromSlash(rel))
}

// WriteFile creates or replaces a file, creating parent directories as needed.
func (r *Repo) WriteFile(rel, content string) {
	r.t.Helper()
	path := r.Path(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatalf("gittest: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatalf("gittest: %v", err)
	}
}

// ReadFile returns the content of a file in the working tree.
func (r *Repo) ReadFile(rel string) string {
	r.t.Helper()
	data, err := os.ReadFile(r.Path(rel))
	if err != nil {
		r.t.Fatalf("gittest: %v", err)
	}
	return string(data)
}

// Remove deletes a file from the working tree.
func (r *Repo) Remove(rel string) {
	r.t.Helper()
	if err := os.Remove(r.Path(rel)); err != nil {
		r.t.Fatalf("gittest: %v", err)
	}
}

// MkdirAll creates a directory and returns its absolute path.
func (r *Repo) MkdirAll(rel string) string {
	r.t.Helper()
	path := r.Path(rel)
	if err := os.MkdirAll(path, 0o755); err != nil {
		r.t.Fatalf("gittest: %v", err)
	}
	return path
}

// Commit stages everything and commits it, returning the new commit's hash.
func (r *Repo) Commit(message string) string {
	r.t.Helper()
	r.Git("add", "--all")
	r.Git("commit", "--quiet", "--message", message)
	return r.Git("rev-parse", "HEAD")
}
