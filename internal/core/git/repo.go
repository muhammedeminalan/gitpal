package git

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

// TopLevel returns the root of the working tree that contains dir. It fails
// with KindNotARepo when dir is not inside a repository.
func (r *Runner) TopLevel(ctx context.Context, dir string) (string, error) {
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"rev-parse", "--show-toplevel"}})
	if err != nil {
		return "", err
	}
	return filepath.FromSlash(strings.TrimSuffix(string(res.Stdout), "\n")), nil
}

func errorsAs(err error, target **Error) bool { return errors.As(err, target) }

// Head returns the commit HEAD points at. ok is false when the current branch
// has no commits yet (a freshly initialized repository).
func (r *Runner) Head(ctx context.Context, dir string) (hash string, ok bool, err error) {
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}})
	if err != nil {
		// With --quiet, an unresolvable HEAD is exit status 1 and no message.
		if code, exited := ExitCode(err); exited && code == 1 {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(string(res.Stdout)), true, nil
}
