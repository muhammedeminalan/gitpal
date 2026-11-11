package git

import (
	"context"
	"strings"
)

// checkURL refuses a URL that could be read as an option or that holds a
// control character.
func checkURL(url string) error {
	if url == "" || strings.HasPrefix(url, "-") || strings.ContainsFunc(url, func(c rune) bool { return c < ' ' }) {
		return &ArgError{Code: ArgInvalidRemote, Value: redact(url)}
	}
	return nil
}

// AddRemote configures a new remote.
func (r *Runner) AddRemote(ctx context.Context, dir, name, url string) error {
	if err := checkName(name, ArgInvalidRemote); err != nil {
		return err
	}
	if err := checkURL(url); err != nil {
		return err
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"remote", "add", "--", name, url}})
	return err
}

// RemoveRemote deletes a remote and its remote-tracking branches. Nothing on
// the remote itself changes.
func (r *Runner) RemoveRemote(ctx context.Context, dir, name string) error {
	if err := r.remoteRequire(ctx, dir, name); err != nil {
		return err
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"remote", "remove", "--", name}})
	return err
}

// RenameRemote renames a remote.
func (r *Runner) RenameRemote(ctx context.Context, dir, oldName, newName string) error {
	if err := r.remoteRequire(ctx, dir, oldName); err != nil {
		return err
	}
	if err := checkName(newName, ArgInvalidRemote); err != nil {
		return err
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"remote", "rename", "--", oldName, newName}})
	return err
}

// SetRemoteURL changes a remote's address; push changes only the push URL.
func (r *Runner) SetRemoteURL(ctx context.Context, dir, name, url string, push bool) error {
	if err := r.remoteRequire(ctx, dir, name); err != nil {
		return err
	}
	if err := checkURL(url); err != nil {
		return err
	}
	args := []string{"remote", "set-url"}
	if push {
		args = append(args, "--push")
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: append(args, "--", name, url)})
	return err
}

// Submodule is one submodule and how it stands.
type Submodule struct {
	Path string
	// Hash is the commit checked out (or recorded, when not initialized).
	Hash string
	// Initialized is false until `submodule update --init`.
	Initialized bool
	// Modified is set when the checked-out commit differs from the recorded
	// one; Conflicted during a merge conflict.
	Modified, Conflicted bool
}

// Submodules lists the submodules, recursively.
func (r *Runner) Submodules(ctx context.Context, dir string) ([]Submodule, error) {
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"submodule", "status", "--recursive"}})
	if err != nil {
		return nil, err
	}
	var subs []Submodule
	for line := range strings.SplitSeq(strings.TrimRight(string(res.Stdout), "\n"), "\n") {
		if len(line) < 42 {
			continue
		}
		// "<state><hash> <path>[ (<describe>)]"
		state, rest := line[0], line[1:]
		hash, path, _ := strings.Cut(rest, " ")
		if i := strings.LastIndex(path, " ("); i >= 0 && strings.HasSuffix(path, ")") {
			path = path[:i]
		}
		subs = append(subs, Submodule{
			Path: path, Hash: hash, Initialized: state != '-', Modified: state == '+', Conflicted: state == 'U',
		})
	}
	return subs, nil
}

// UpdateSubmodules checks out the recorded commit of every submodule,
// initializing and fetching them as needed.
func (r *Runner) UpdateSubmodules(ctx context.Context, dir string) error {
	_, err := r.Run(ctx, networkCmd(dir, "submodule", "update", "--init", "--recursive"))
	return err
}

// SetIdentity sets user.name and user.email, for this repository or
// globally. An empty value leaves that setting alone.
func (r *Runner) SetIdentity(ctx context.Context, dir string, global bool, id Identity) error {
	for key, value := range map[string]string{"user.name": id.Name, "user.email": id.Email} {
		if value == "" {
			continue
		}
		args := []string{"config"}
		if global {
			args = append(args, "--global")
		}
		if _, err := r.Run(ctx, Cmd{Dir: dir, Args: append(args, "--", key, value)}); err != nil {
			return err
		}
	}
	return nil
}
