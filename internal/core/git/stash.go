package git

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Stash is one entry of the stash list.
type Stash struct {
	// Index is the N of stash@{N}; 0 is the newest.
	Index   int
	Ref     string
	Hash    string
	Message string
	Time    time.Time
}

// StashOptions describes what to stash.
type StashOptions struct {
	Message string
	// IncludeUntracked also stashes (and removes) untracked files.
	IncludeUntracked bool
	// Paths limits the stash to these files.
	Paths []string
}

// stashRef is the ref holding the stash.
const stashRef = "refs/stash"

func (r *Runner) stashTip(ctx context.Context, dir string) (string, error) {
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"rev-parse", "--verify", "--quiet", stashRef}})
	if err != nil {
		if code, exited := ExitCode(err); exited && code == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(res.Stdout)), nil
}

// StashPush puts the uncommitted changes aside and cleans the working tree.
// created is false when there was nothing to stash.
func (r *Runner) StashPush(ctx context.Context, dir string, opts StashOptions) (created bool, err error) {
	before, err := r.stashTip(ctx, dir)
	if err != nil {
		return false, err
	}
	cmd := Cmd{Dir: dir, Args: []string{"stash", "push", "--quiet"}}
	if opts.IncludeUntracked {
		cmd.Args = append(cmd.Args, "--include-untracked")
	}
	if opts.Message != "" {
		cmd.Args = append(cmd.Args, "--message="+opts.Message)
	}
	if len(opts.Paths) > 0 {
		pathArgs, stdin, err := pathspecStdin(opts.Paths)
		if err != nil {
			return false, err
		}
		cmd.Args, cmd.Stdin, cmd.Env = append(cmd.Args, pathArgs...), stdin, []string{literalPathspecsEnv}
	}
	if _, err := r.Run(ctx, cmd); err != nil {
		return false, err
	}
	after, err := r.stashTip(ctx, dir)
	return after != before, err
}

// Stashes lists the stash, newest first.
func (r *Runner) Stashes(ctx context.Context, dir string) ([]Stash, error) {
	tip, err := r.stashTip(ctx, dir)
	if err != nil || tip == "" {
		return nil, err
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{
		"log", "--walk-reflogs", "--no-color", "--no-show-signature", "--encoding=UTF-8",
		"--format=%H%x00%gs%x00%ct", "-z", stashRef, "--",
	}})
	if err != nil {
		return nil, err
	}
	fields := splitNUL(res.Stdout)
	if len(fields)%3 != 0 {
		return nil, errors.New("git stash list: unexpected output")
	}
	stashes := make([]Stash, 0, len(fields)/3)
	for i := 0; i < len(fields); i += 3 {
		n := i / 3
		secs, err := strconv.ParseInt(strings.TrimSpace(fields[i+2]), 10, 64)
		if err != nil {
			return nil, errors.New("git stash list: unexpected date")
		}
		stashes = append(stashes, Stash{
			Index: n, Ref: StashRef(n), Hash: fields[i], Message: fields[i+1], Time: time.Unix(secs, 0),
		})
	}
	return stashes, nil
}

// StashRef names a stash entry.
func StashRef(index int) string { return "stash@{" + strconv.Itoa(index) + "}" }

// StashApply puts a stash entry's changes back, staged state included. With
// pop the entry is dropped afterwards — unless the changes conflicted, in
// which case it is kept (and the error has kind KindConflict).
func (r *Runner) StashApply(ctx context.Context, dir string, index int, pop bool) error {
	if index < 0 {
		return &ArgError{Code: ArgInvalidRev, Value: strconv.Itoa(index)}
	}
	command := "apply"
	if pop {
		command = "pop"
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"stash", command, "--quiet", "--index", StashRef(index)}})
	if err != nil {
		return r.conflictKind(ctx, dir, err)
	}
	return nil
}

// StashDrop deletes a stash entry. Its changes are then only reachable
// through a backup; taking one is the safety layer's job.
func (r *Runner) StashDrop(ctx context.Context, dir string, index int) error {
	if index < 0 {
		return &ArgError{Code: ArgInvalidRev, Value: strconv.Itoa(index)}
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"stash", "drop", "--quiet", StashRef(index)}})
	return err
}

// StashStore puts a stash commit back on the stash list (it becomes
// stash@{0}). It undoes a drop.
func (r *Runner) StashStore(ctx context.Context, dir, hash, message string) error {
	if err := checkName(hash, ArgInvalidRev); err != nil {
		return err
	}
	args := []string{"stash", "store", "--quiet"}
	if message != "" {
		args = append(args, "--message="+message)
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: append(args, hash)})
	return err
}
