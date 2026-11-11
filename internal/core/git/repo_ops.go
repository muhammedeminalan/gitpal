package git

import (
	"context"
	"io"
	"strconv"
	"strings"
)

// Init creates a repository in dir (creating dir if needed). branch names the
// first branch; empty leaves it to the user's init.defaultBranch. Running it
// on an existing repository is harmless: git only reinitializes it.
func (r *Runner) Init(ctx context.Context, dir, branch string) error {
	args := []string{"init", "--quiet"}
	if branch != "" {
		if err := checkName(branch, ArgInvalidBranch); err != nil {
			return err
		}
		args = append(args, "--initial-branch="+branch)
	}
	_, err := r.Run(ctx, Cmd{Args: append(args, "--", dir)})
	return err
}

// CloneOptions adjusts Clone.
type CloneOptions struct {
	// Branch checks out this branch instead of the remote's default.
	Branch string
	// Depth makes a shallow clone with that many commits; zero is a full one.
	Depth int
	// Progress receives git's progress output while the clone runs.
	Progress io.Writer
}

// Clone copies the repository at url into dest, which must not exist or be
// empty.
//
// url is refused when it could be read as an option. git itself refuses the
// "ext::" transport, which runs arbitrary commands, unless the user enabled it.
func (r *Runner) Clone(ctx context.Context, url, dest string, opts CloneOptions) error {
	if url == "" || strings.HasPrefix(url, "-") || strings.ContainsFunc(url, func(c rune) bool { return c < ' ' }) {
		return &ArgError{Code: ArgInvalidRemote, Value: redact(url)}
	}
	if dest == "" || strings.HasPrefix(dest, "-") {
		return &ArgError{Code: ArgInvalidPath, Value: dest}
	}
	args := []string{"clone"}
	if opts.Progress != nil {
		args = append(args, "--progress")
	} else {
		args = append(args, "--quiet")
	}
	if opts.Branch != "" {
		if err := checkName(opts.Branch, ArgInvalidBranch); err != nil {
			return err
		}
		args = append(args, "--branch="+opts.Branch)
	}
	if opts.Depth > 0 {
		args = append(args, "--depth="+itoa(opts.Depth))
	}
	cmd := networkCmd("", append(args, "--", url, dest)...)
	cmd.Progress = opts.Progress
	_, err := r.Run(ctx, cmd)
	return err
}

func itoa(n int) string { return strconv.Itoa(n) }
