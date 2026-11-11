package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Stage adds the given paths to the index: modifications, new files and
// deletions alike. A directory stages everything under it.
//
// The paths are names, never patterns: "*.txt" stages the file called
// "*.txt" and nothing else.
func (r *Runner) Stage(ctx context.Context, dir string, paths []string) error {
	return r.runWithPaths(ctx, dir, paths, "add", "--all", "--no-ignore-errors")
}

// StageAll adds every change in the working tree to the index.
func (r *Runner) StageAll(ctx context.Context, dir string) error {
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"add", "--all", "--no-ignore-errors"}})
	return err
}

// Unstage takes the given paths out of the index without touching the working
// tree: the changes are kept, they are just no longer staged.
func (r *Runner) Unstage(ctx context.Context, dir string, paths []string) error {
	_, born, err := r.Head(ctx, dir)
	if err != nil {
		return err
	}
	if born {
		return r.runWithPaths(ctx, dir, paths, "restore", "--staged", "--no-progress")
	}
	// Without a commit there is nothing for `restore --staged` to restore
	// from; unstaging means dropping the entry. --force is needed for a file
	// edited again after it was staged, and is safe: with --cached git never
	// touches the working tree.
	return r.runWithPaths(ctx, dir, paths, "rm", "--cached", "--force", "-r", "--quiet")
}

// UnstageAll empties the index of staged changes, keeping the working tree.
//
// It resets by path rather than with a bare `git reset`, which would also
// abandon a merge in progress.
func (r *Runner) UnstageAll(ctx context.Context, dir string) error {
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"reset", "--quiet", "--", "."}})
	return err
}

// RestoreOptions says what Restore puts back, and from where.
type RestoreOptions struct {
	Paths []string
	// Source is the revision to take the content from. Empty means git's
	// default: HEAD when the index is restored, otherwise the index.
	Source string
	// Staged restores the index; Worktree restores the working tree. When
	// both are false the working tree is restored, as git does.
	Staged   bool
	Worktree bool
}

// Restore puts the given paths back to the state of the source.
//
// Restoring the working tree throws uncommitted changes away for good. This
// function does exactly what it is asked; asking the user and keeping a
// backup is the safety layer's job. Untracked files are never deleted.
func (r *Runner) Restore(ctx context.Context, dir string, opts RestoreOptions) error {
	args := []string{"restore", "--no-progress"}
	if opts.Source != "" {
		if err := checkName(opts.Source, ArgInvalidRev); err != nil {
			return err
		}
		args = append(args, "--source="+opts.Source)
	}
	if opts.Staged {
		args = append(args, "--staged")
	}
	if opts.Worktree {
		args = append(args, "--worktree")
	}
	return r.runWithPaths(ctx, dir, opts.Paths, args...)
}

// runWithPaths runs a command that takes its paths from standard input, as
// literal names.
func (r *Runner) runWithPaths(ctx context.Context, dir string, paths []string, args ...string) error {
	pathArgs, stdin, err := pathspecStdin(paths)
	if err != nil {
		return err
	}
	_, err = r.Run(ctx, Cmd{
		Dir:   dir,
		Args:  append(args, pathArgs...),
		Stdin: stdin,
		Env:   []string{literalPathspecsEnv},
	})
	return err
}

// IgnorePaths appends the given paths to the .gitignore at the root of the
// working tree dir, creating the file if needed.
//
// Each path becomes a pattern that matches that path and nothing else: it is
// anchored at the root ("/build/out.bin" does not match "sub/build/out.bin")
// and pattern characters are escaped (a file called "a*.txt" does not make
// "ab.txt" ignored). A path ending in "/" matches a directory only.
//
// Existing content is kept byte for byte. Patterns already present are
// skipped, a missing final newline is added first, and a file with CRLF line
// endings gets CRLF. A path containing a line break cannot be written to a
// .gitignore and is refused.
func IgnorePaths(dir string, paths []string) error {
	if err := checkPaths(paths); err != nil {
		return err
	}
	patterns := make([]string, 0, len(paths))
	for _, p := range paths {
		pattern, ok := stageIgnorePattern(p)
		if !ok {
			return &ArgError{Code: ArgInvalidPath, Value: p}
		}
		patterns = append(patterns, pattern)
	}

	file := filepath.Join(dir, ".gitignore")
	existing, err := os.ReadFile(file) //nolint:gosec // G304: the repository's own .gitignore
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read .gitignore: %w", err)
	}

	eol := "\n"
	if bytes.Contains(existing, []byte("\r\n")) {
		eol = "\r\n"
	}
	present := map[string]bool{}
	for line := range strings.SplitSeq(string(existing), "\n") {
		present[strings.TrimSuffix(line, "\r")] = true
	}

	var add strings.Builder
	for _, pattern := range patterns {
		if present[pattern] {
			continue
		}
		present[pattern] = true
		add.WriteString(pattern)
		add.WriteString(eol)
	}
	if add.Len() == 0 {
		return nil
	}

	content := slices.Clone(existing)
	if len(content) > 0 && content[len(content)-1] != '\n' {
		content = append(content, eol...)
	}
	content = append(content, add.String()...)
	// A .gitignore is an ordinary tracked file; it gets the ordinary mode.
	if err := os.WriteFile(file, content, 0o644); err != nil { //nolint:gosec // G306: see above
		return fmt.Errorf("write .gitignore: %w", err)
	}
	return nil
}

// stageIgnorePattern turns a path (already checked by checkPaths) into a
// .gitignore pattern matching exactly that path. ok is false when the path
// cannot be expressed: the root itself, or a name with a line break.
func stageIgnorePattern(p string) (pattern string, ok bool) {
	if strings.ContainsAny(p, "\n\r") {
		return "", false
	}
	clean := path.Clean(p)
	if clean == "." {
		return "", false
	}

	var b strings.Builder
	// The leading slash anchors the pattern at the root. It also means a name
	// starting with "#" or "!" is not read as a comment or a negation.
	b.WriteByte('/')
	for i := 0; i < len(clean); i++ {
		if strings.IndexByte(`\*?[`, clean[i]) >= 0 {
			b.WriteByte('\\')
		}
		b.WriteByte(clean[i])
	}
	if strings.HasSuffix(p, "/") {
		b.WriteByte('/')
	}

	// git drops trailing spaces from a pattern unless each is escaped.
	pattern = b.String()
	trimmed := strings.TrimRight(pattern, " ")
	return trimmed + strings.Repeat(`\ `, len(pattern)-len(trimmed)), true
}
