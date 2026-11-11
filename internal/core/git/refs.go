package git

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"strings"
)

// OwnRefPrefix is the namespace gitpal keeps its own refs in (backups). Refs
// there are invisible to `git branch` and `git tag`, are not pushed or
// fetched by default, and keep their commits from being garbage collected.
const OwnRefPrefix = "refs/gitpal/"

// GitPath returns the absolute path of a file inside the git directory of the
// working tree rooted at dir. A linked worktree has a git directory of its
// own, so each worktree gets its own file.
func (r *Runner) GitPath(ctx context.Context, dir, name string) (string, error) {
	if !validRelPath(name) || strings.HasPrefix(name, "-") {
		return "", &ArgError{Code: ArgInvalidPath, Value: name}
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"rev-parse", "--git-path", name}})
	if err != nil {
		return "", err
	}
	p := filepath.FromSlash(strings.TrimRight(string(res.Stdout), "\r\n"))
	if !filepath.IsAbs(p) {
		// git prints it relative to the directory it ran in.
		p = filepath.Join(dir, p)
	}
	return p, nil
}

// ResolveCommit returns the full hash of the commit a revision names. The
// error has kind KindRefNotFound when it names none.
func (r *Runner) ResolveCommit(ctx context.Context, dir, rev string) (string, error) {
	return r.resolveCommit(ctx, dir, rev)
}

// Ref is a ref and the object it points at.
type Ref struct {
	Name string
	Hash string
}

// checkOwnRef refuses any ref outside gitpal's own namespace, so the
// functions that write refs directly can never move a branch or a tag.
func checkOwnRef(ref string) error {
	invalid := &ArgError{Code: ArgInvalidRev, Value: ref}
	if err := checkName(ref, ArgInvalidRev); err != nil {
		return err
	}
	if !strings.HasPrefix(ref, OwnRefPrefix) || len(ref) == len(OwnRefPrefix) ||
		strings.HasSuffix(ref, "/") || path.Clean(ref) != ref {
		return invalid
	}
	return nil
}

// Refs lists gitpal's own refs whose names start with prefix, sorted by name.
func (r *Runner) Refs(ctx context.Context, dir, prefix string) ([]Ref, error) {
	if err := checkName(prefix, ArgInvalidRev); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(prefix, OwnRefPrefix) {
		return nil, &ArgError{Code: ArgInvalidRev, Value: prefix}
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{
		"for-each-ref", "--sort=refname", "--format=%(objectname) %(refname)", prefix,
	}})
	if err != nil {
		return nil, err
	}
	var refs []Ref
	for line := range strings.SplitSeq(string(res.Stdout), "\n") {
		hash, name, ok := strings.Cut(strings.TrimSuffix(line, "\r"), " ")
		if !ok {
			continue
		}
		refs = append(refs, Ref{Name: name, Hash: hash})
	}
	return refs, nil
}

// CreateRef makes a new ref under refs/gitpal/ pointing at a commit. It never
// overwrites an existing ref.
func (r *Runner) CreateRef(ctx context.Context, dir, ref, rev string) error {
	if err := checkOwnRef(ref); err != nil {
		return err
	}
	hash, err := r.resolveCommit(ctx, dir, rev)
	if err != nil {
		return err
	}
	// The empty old value means "only if the ref does not exist yet".
	_, err = r.Run(ctx, Cmd{Dir: dir, Args: []string{"update-ref", ref, hash, ""}})
	return err
}

// DeleteRef removes a ref under refs/gitpal/.
func (r *Runner) DeleteRef(ctx context.Context, dir, ref string) error {
	if err := checkOwnRef(ref); err != nil {
		return err
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"update-ref", "-d", ref}})
	return err
}

// WorktreeSnapshot records the uncommitted changes to tracked files (staged
// and unstaged) as a commit, without touching the index, the working tree or
// the user's stash. ok is false when there is nothing to record.
//
// The commit is not referenced by anything yet; give it a ref with CreateRef
// to keep it. Untracked files are not included: nothing gitpal does deletes
// them.
func (r *Runner) WorktreeSnapshot(ctx context.Context, dir string) (hash string, ok bool, err error) {
	_, born, err := r.Head(ctx, dir)
	if err != nil || !born {
		// A stash is built on a commit; without one there is nothing to base
		// it on.
		return "", false, err
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"stash", "create"}})
	if err != nil {
		return "", false, err
	}
	hash = strings.TrimSpace(string(res.Stdout))
	return hash, hash != "", nil
}

// ApplySnapshot puts the changes recorded by WorktreeSnapshot back, staged
// state included. It fails, changing nothing, when they no longer apply to
// the files as they are now.
func (r *Runner) ApplySnapshot(ctx context.Context, dir, snapshot string) error {
	if err := checkName(snapshot, ArgInvalidRev); err != nil {
		return err
	}
	// snapshot cannot start with "-" (checkName), so it cannot be read as an option.
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"stash", "apply", "--quiet", "--index", snapshot}})
	return err
}

// ResetMode says how much Reset rewinds.
type ResetMode string

// Reset modes.
const (
	// ResetSoft moves the branch only; index and working tree are kept.
	ResetSoft ResetMode = "soft"
	// ResetMixed also resets the index; the working tree is kept.
	ResetMixed ResetMode = "mixed"
	// ResetHard also resets the working tree: uncommitted changes to tracked
	// files are lost for good.
	ResetHard ResetMode = "hard"
)

// Reset moves the current branch to rev.
//
// It does exactly what it is asked. ResetHard destroys uncommitted work;
// asking the user and keeping a backup is the safety layer's job.
func (r *Runner) Reset(ctx context.Context, dir string, mode ResetMode, rev string) error {
	if mode != ResetSoft && mode != ResetMixed && mode != ResetHard {
		return &ArgError{Code: ArgInvalidRev, Value: string(mode)}
	}
	if err := checkName(rev, ArgInvalidRev); err != nil {
		return err
	}
	hash, err := r.resolveCommit(ctx, dir, rev)
	if err != nil {
		return err
	}
	if hash == "" {
		return errors.New("git reset: revision resolved to nothing")
	}
	_, err = r.Run(ctx, Cmd{Dir: dir, Args: []string{"reset", "--quiet", "--" + string(mode), hash, "--"}})
	return err
}
