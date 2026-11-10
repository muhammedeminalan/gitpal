package git

import (
	"context"
	"strings"
)

// Identity is who git records as the author of a commit.
type Identity struct {
	Name  string
	Email string
}

// Complete reports whether both parts git needs for a commit are set.
func (i Identity) Complete() bool { return i.Name != "" && i.Email != "" }

// ConfigGet reads one git configuration value as git itself would resolve it
// in dir (repository, then global, then system). A key that is not set is
// reported with found=false, not as an error.
func (r *Runner) ConfigGet(ctx context.Context, dir, key string) (value string, found bool, err error) {
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"config", "--get", "--null", "--", key}})
	if err != nil {
		// Exit status 1 with nothing on stderr is git's way of saying "not set".
		var gerr *Error
		if code, ok := ExitCode(err); ok && code == 1 && errorsAs(err, &gerr) && gerr.Stderr == "" {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSuffix(string(res.Stdout), "\x00"), true, nil
}

// Identity returns the configured user.name and user.email. Unset parts are
// empty; use Identity.Complete to check.
func (r *Runner) Identity(ctx context.Context, dir string) (Identity, error) {
	name, _, err := r.ConfigGet(ctx, dir, "user.name")
	if err != nil {
		return Identity{}, err
	}
	email, _, err := r.ConfigGet(ctx, dir, "user.email")
	if err != nil {
		return Identity{}, err
	}
	return Identity{Name: name, Email: email}, nil
}
