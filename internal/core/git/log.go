package git

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Commit is one commit of the history.
type Commit struct {
	Hash      string
	ShortHash string
	// Parents are the full hashes of the parent commits: none for a root
	// commit, two or more for a merge. They are what a history graph is
	// drawn from.
	Parents []string

	// AuthorName and AuthorEmail are as recorded in the commit, without
	// applying a .mailmap.
	AuthorName, AuthorEmail string
	// AuthorDate keeps the UTC offset the commit was made with.
	AuthorDate time.Time

	CommitterName, CommitterEmail string
	CommitterDate                 time.Time

	// Subject is the first paragraph of the message, on one line.
	Subject string
	// Body is the rest of the message, without trailing newlines.
	Body string
	// Refs are the names pointing at the commit, as git decorates them:
	// "HEAD -> main", "origin/main", "tag: v1.0".
	Refs []string
}

// IsMerge reports whether the commit has more than one parent.
func (c Commit) IsMerge() bool { return len(c.Parents) > 1 }

// LogOptions chooses which commits Log returns.
type LogOptions struct {
	// Revs are revisions or ranges such as "main..feat" or "^main". Empty
	// means HEAD.
	Revs []string
	// All starts from every ref instead.
	All bool
	// Max limits the number of commits; zero means no limit.
	Max int
	// Skip leaves out that many commits from the start, for paging.
	Skip int
	// Author and Grep keep commits whose author ("Name <email>") or message
	// contains the text. The text is matched literally, not as a regular
	// expression: someone searching for "fix(ui)" or "[wip]" means exactly
	// that. With both set, a commit must match both.
	Author, Grep string
	// IgnoreCase applies to Author and Grep.
	IgnoreCase bool
	// Since and Until bound the committer date; the zero value is no bound.
	Since, Until time.Time
	// Paths keeps commits that touched these files or directories.
	Paths []string
	// Follow continues the history of a file across renames. git supports it
	// for a single file only; it is ignored unless Paths has exactly one
	// entry.
	Follow bool
	// FirstParent follows only the first parent of merges.
	FirstParent bool
	// NoMerges leaves merge commits out.
	NoMerges bool
}

// logFields is the layout Log asks git for: these fields separated by NUL,
// and, because of -z, a NUL after each commit. No field can contain a NUL, so
// a commit is exactly len(logFields) fields whatever its message says.
var logFields = []string{
	"%H", "%h", "%P",
	"%an", "%ae", "%aI",
	"%cn", "%ce", "%cI",
	"%D", "%s", "%b",
}

// logFlags pin down everything the user's configuration could change about
// the output: the format (format.pretty), signature checks printed into it
// (log.showSignature), decoration style (log.decorate), notes, the mailmap
// (log.mailmap, which would also change what --author matches) and the
// encoding (i18n.logOutputEncoding). Dates use %aI/%cI, which log.date does
// not affect.
var logFlags = []string{
	"-z", "--no-color", "--no-show-signature", "--no-notes", "--no-use-mailmap",
	"--decorate=short", "--encoding=UTF-8",
}

// Log returns commits, newest first. On a branch without commits it returns
// nothing, unless specific revisions were asked for.
func (r *Runner) Log(ctx context.Context, dir string, opts LogOptions) ([]Commit, error) {
	if len(opts.Revs) == 0 && !opts.All {
		_, born, err := r.Head(ctx, dir)
		if err != nil || !born {
			return nil, err
		}
	}

	args := append([]string{"log"}, logFlags...)
	args = append(args, "--format="+strings.Join(logFields, "%x00"))
	if opts.Max > 0 {
		args = append(args, "--max-count="+strconv.Itoa(opts.Max))
	}
	if opts.Skip > 0 {
		args = append(args, "--skip="+strconv.Itoa(opts.Skip))
	}
	if opts.Author != "" || opts.Grep != "" {
		args = append(args, "--fixed-strings")
		if opts.IgnoreCase {
			args = append(args, "--regexp-ignore-case")
		}
	}
	// User text only ever appears as the value of an "--option=value" argument.
	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}
	if opts.Grep != "" {
		args = append(args, "--grep="+opts.Grep)
	}
	if !opts.Since.IsZero() {
		args = append(args, "--since="+opts.Since.UTC().Format(time.RFC3339))
	}
	if !opts.Until.IsZero() {
		args = append(args, "--until="+opts.Until.UTC().Format(time.RFC3339))
	}
	if opts.FirstParent {
		args = append(args, "--first-parent")
	}
	if opts.NoMerges {
		args = append(args, "--no-merges")
	}
	// log.follow would otherwise turn following on by itself.
	if opts.Follow && len(opts.Paths) == 1 {
		args = append(args, "--follow")
	} else {
		args = append(args, "--no-follow")
	}
	if opts.All {
		args = append(args, "--all")
	}

	args = append(args, endOfOptions)
	for _, rev := range opts.Revs {
		if err := checkName(rev, ArgInvalidRev); err != nil {
			return nil, err
		}
		args = append(args, rev)
	}
	args = append(args, "--")
	if len(opts.Paths) > 0 {
		if err := checkPaths(opts.Paths); err != nil {
			return nil, err
		}
		args = append(args, opts.Paths...)
	}

	res, err := r.Run(ctx, Cmd{Dir: dir, Args: args, Env: []string{literalPathspecsEnv}})
	if err != nil {
		return nil, err
	}
	return ParseLog(res.Stdout)
}

// ParseLog parses the output of the `git log` command Log runs.
//
// Errors never quote the output: commit messages are repository content.
func ParseLog(out []byte) ([]Commit, error) {
	fields := splitNUL(out)
	n := len(logFields)
	if len(fields)%n != 0 {
		return nil, errors.New("git log: output does not divide into whole commits")
	}
	commits := make([]Commit, 0, len(fields)/n)
	for i := 0; i < len(fields); i += n {
		f := fields[i : i+n]
		if f[0] == "" {
			return nil, errors.New("git log: commit without a hash")
		}
		c := Commit{
			Hash: f[0], ShortHash: f[1], Parents: strings.Fields(f[2]),
			AuthorName: f[3], AuthorEmail: f[4],
			CommitterName: f[6], CommitterEmail: f[7],
			Subject: f[10], Body: strings.TrimRight(f[11], "\n"),
		}
		var err error
		if c.AuthorDate, err = time.Parse(time.RFC3339, f[5]); err != nil {
			return nil, errors.New("git log: malformed author date")
		}
		if c.CommitterDate, err = time.Parse(time.RFC3339, f[8]); err != nil {
			return nil, errors.New("git log: malformed committer date")
		}
		if f[9] != "" {
			// Ref names cannot contain a space, so ", " only separates them.
			c.Refs = strings.Split(f[9], ", ")
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// CommitInfo returns one commit. The error has kind KindRefNotFound when rev
// does not name a commit.
func (r *Runner) CommitInfo(ctx context.Context, dir, rev string) (Commit, error) {
	hash, err := r.resolveCommit(ctx, dir, rev)
	if err != nil {
		return Commit{}, err
	}
	commits, err := r.Log(ctx, dir, LogOptions{Revs: []string{hash}, Max: 1})
	if err != nil {
		return Commit{}, err
	}
	if len(commits) != 1 {
		return Commit{}, errors.New("git log: expected exactly one commit")
	}
	return commits[0], nil
}

// RecentMessages returns the full messages of the last n commits on HEAD,
// newest first, without trailing newlines. It returns nil on a branch without
// commits.
func (r *Runner) RecentMessages(ctx context.Context, dir string, n int) ([]string, error) {
	_, born, err := r.Head(ctx, dir)
	if err != nil || !born || n <= 0 {
		return nil, err
	}
	args := append([]string{"log"}, logFlags...)
	args = append(args, "--format=%B", "--max-count="+strconv.Itoa(n), "--no-follow", "HEAD", "--")
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: args})
	if err != nil {
		return nil, err
	}
	messages := splitNUL(res.Stdout)
	for i, m := range messages {
		messages[i] = strings.TrimRight(m, "\n")
	}
	return messages, nil
}
