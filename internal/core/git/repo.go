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
