package git

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SignMode says whether a commit is signed.
type SignMode int

// Sign modes.
const (
	// SignDefault follows the user's git configuration (commit.gpgsign).
	SignDefault SignMode = iota
	// SignAlways signs the commit whatever the configuration says.
	SignAlways
	// SignNever does not sign the commit whatever the configuration says.
	SignNever
)

// CommitOptions describes a commit to make.
type CommitOptions struct {
	// Message is the commit message. Leading and trailing blank lines are
	// trimmed; everything else, lines starting with "#" included, is kept.
	// With Amend, an empty Message keeps the message the commit already has.
	Message string
	// Amend replaces the last commit instead of adding a new one.
	Amend bool
	// AllowEmpty permits a commit that changes nothing.
	AllowEmpty bool
	Sign       SignMode
}

// CommitResult is the commit that was made.
type CommitResult struct {
	// Hash is the full hash of the new commit.
	Hash string
}

// commitHooks are the hooks git runs while committing and that can refuse the
// commit.
var commitHooks = []string{"pre-commit", "prepare-commit-msg", "commit-msg"}

// Commit records the staged changes.
//
// The message reaches git on standard input, so no editor is ever needed and
// nothing in it is interpreted: `--cleanup=whitespace` keeps "#" lines
// (commit.cleanup, core.commentChar) and `--no-verbose` keeps what follows a
// line that looks like a scissors mark (commit.verbose). git itself ignores
// commit.template when the message comes from a file.
//
// Hooks always run. When one of them refuses the commit, the error has kind
// KindHookFailed and Stderr holds what the hook printed.
//
// With nothing staged the commit is not attempted and the error has kind
// KindNothingToCommit, unless AllowEmpty or Amend is set or a merge is waiting
// to be concluded.
func (r *Runner) Commit(ctx context.Context, dir string, opts CommitOptions) (CommitResult, error) {
	keepMessage := opts.Amend && opts.Message == ""
	if !keepMessage && strings.TrimSpace(opts.Message) == "" {
		return CommitResult{}, &ArgError{Code: ArgEmptyMessage}
	}

	if !opts.AllowEmpty && !opts.Amend {
		if err := r.commitRequireStaged(ctx, dir); err != nil {
			return CommitResult{}, err
		}
	}

	cmd := Cmd{Dir: dir, Args: []string{"commit", "--quiet", "--cleanup=whitespace", "--no-verbose"}}
	if opts.Amend {
		cmd.Args = append(cmd.Args, "--amend")
	}
	if opts.AllowEmpty {
		cmd.Args = append(cmd.Args, "--allow-empty")
	}
	switch opts.Sign {
	case SignAlways:
		cmd.Args = append(cmd.Args, "--gpg-sign")
	case SignNever:
		cmd.Args = append(cmd.Args, "--no-gpg-sign")
	case SignDefault:
	}
	if keepMessage {
		cmd.Args = append(cmd.Args, "--no-edit")
	} else {
		cmd.Args = append(cmd.Args, "--file=-")
		cmd.Stdin = []byte(opts.Message)
	}

	if _, err := r.Run(ctx, cmd); err != nil {
		var gerr *Error
		if errorsAs(err, &gerr) && gerr.Kind == KindUnknown && r.hookInstalled(ctx, dir, commitHooks...) {
			// git adds nothing of its own when a hook exits non-zero, so an
			// unexplained failure with a hook in place is the hook's doing.
			gerr.Kind = KindHookFailed
		}
		return CommitResult{}, err
	}

	hash, _, err := r.Head(ctx, dir)
	if err != nil {
		return CommitResult{}, err
	}
	return CommitResult{Hash: hash}, nil
}

// commitRequireStaged returns a KindNothingToCommit error when there is
// nothing to record. A merge being concluded is exempt: resolving every
// conflict in favour of our side leaves nothing staged, yet it still needs
// its commit.
func (r *Runner) commitRequireStaged(ctx context.Context, dir string) error {
	staged, err := r.HasStagedChanges(ctx, dir)
	if err != nil || staged {
		return err
	}
	op, err := r.InProgress(ctx, dir)
	if err != nil || op == OpMerge {
		return err
	}
	return &Error{Kind: KindNothingToCommit, Args: []string{"commit"}, ExitCode: -1}
}

// hookInstalled reports whether any of the named hooks exists and can run.
// `--git-path` honours core.hooksPath and linked worktrees. It answers false
// when it cannot tell: the caller is already reporting another error.
func (r *Runner) hookInstalled(ctx context.Context, dir string, names ...string) bool {
	args := []string{"rev-parse"}
	for _, name := range names {
		args = append(args, "--git-path", "hooks/"+name)
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: args})
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(strings.TrimSuffix(string(res.Stdout), "\n"), "\n") {
		path := filepath.FromSlash(strings.TrimSuffix(line, "\r"))
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		// Windows has no executable bit; git runs any hook file that exists.
		if runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0 {
			return true
		}
	}
	return false
}

// HasStagedChanges reports whether the index differs from HEAD, i.e. whether
// a commit would record anything. It also works before the first commit.
func (r *Runner) HasStagedChanges(ctx context.Context, dir string) (bool, error) {
	// Outside a repository `git diff` quietly turns into a file comparison
	// tool with exit statuses of its own, so make sure there is one first.
	if _, _, err := r.Head(ctx, dir); err != nil {
		return false, err
	}
	// --quiet turns the exit status into the answer: 0 same, 1 different.
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"diff", "--cached", "--quiet", "--no-ext-diff"}})
	if err == nil {
		return false, nil
	}
	if code, exited := ExitCode(err); exited && code == 1 {
		return true, nil
	}
	return false, err
}

// LastCommitMessage returns the full message of the commit at HEAD, without
// trailing newlines. The error has kind KindUnbornBranch when there is no
// commit yet.
func (r *Runner) LastCommitMessage(ctx context.Context, dir string) (string, error) {
	_, born, err := r.Head(ctx, dir)
	if err != nil {
		return "", err
	}
	if !born {
		return "", &Error{Kind: KindUnbornBranch, Args: []string{"log", "-1"}, ExitCode: -1}
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{
		"log", "-1", "--format=%B", "--no-show-signature", "--no-notes", "--encoding=UTF-8", "HEAD",
	}})
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(res.Stdout), "\n"), nil
}

// RemoteContains reports whether rev is reachable from any remote-tracking
// branch — in other words, whether it has already been pushed (or fetched
// from) somewhere. Rewriting such a commit rewrites shared history.
//
// It answers from what the last fetch saw; it does not talk to the remote.
func (r *Runner) RemoteContains(ctx context.Context, dir, rev string) (bool, error) {
	if err := checkName(rev, ArgInvalidRev); err != nil {
		return false, err
	}
	// rev is the value of a single "--contains=<rev>" argument, so it cannot
	// be read as an option of its own.
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{
		"for-each-ref", "--count=1", "--format=%(refname)", "--contains=" + rev, "refs/remotes/",
	}})
	if err != nil {
		return false, err
	}
	return len(res.Stdout) > 0, nil
}
