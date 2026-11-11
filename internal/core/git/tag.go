package git

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Tag is a tag.
type Tag struct {
	Name string
	// Commit is the commit the tag points at.
	Commit    string
	Annotated bool
	// Subject is the first line of an annotated tag's message, or of the
	// commit's for a lightweight tag.
	Subject string
	Date    time.Time
}

const tagRefs = "refs/tags/"

// Tags lists the tags, newest first.
func (r *Runner) Tags(ctx context.Context, dir string) ([]Tag, error) {
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{
		"for-each-ref", "--sort=-creatordate", "--sort=-refname",
		"--format=%(refname)%00%(objecttype)%00%(objectname)%00%(*objectname)%00%(contents:subject)%00%(creatordate:iso-strict)",
		tagRefs,
	}})
	if err != nil {
		return nil, err
	}
	var tags []Tag
	for line := range strings.SplitSeq(strings.TrimSuffix(string(res.Stdout), "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\x00")
		if len(f) != 6 {
			return nil, errors.New("git for-each-ref: unexpected number of fields")
		}
		t := Tag{Name: strings.TrimPrefix(f[0], tagRefs), Commit: f[2], Annotated: f[1] == "tag", Subject: f[4]}
		if t.Annotated {
			t.Commit = f[3]
		}
		if f[5] != "" {
			if t.Date, err = time.Parse(time.RFC3339, f[5]); err != nil {
				return nil, errors.New("git for-each-ref: malformed date")
			}
		}
		tags = append(tags, t)
	}
	return tags, nil
}

// ValidateTagName checks a name for a new tag.
func (r *Runner) ValidateTagName(ctx context.Context, dir, name string) error {
	if err := checkName(name, ArgInvalidRev); err != nil {
		return err
	}
	if _, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"check-ref-format", tagRefs + name}}); err != nil {
		if _, exited := ExitCode(err); exited {
			return &ArgError{Code: ArgInvalidRev, Value: name}
		}
		return err
	}
	return nil
}

// CreateTag tags rev (HEAD when empty). A message makes it an annotated tag,
// signed if the user's tag.gpgSign says so (as commits follow commit.gpgSign);
// a lightweight tag is never turned into a signed one. It never moves an
// existing tag.
func (r *Runner) CreateTag(ctx context.Context, dir, name, rev, message string) error {
	if err := r.ValidateTagName(ctx, dir, name); err != nil {
		return err
	}
	if rev == "" {
		rev = "HEAD"
	}
	hash, err := r.resolveCommit(ctx, dir, rev)
	if err != nil {
		return err
	}
	cmd := Cmd{Dir: dir, Args: []string{"tag"}}
	if message != "" {
		cmd.Args = append(cmd.Args, "--annotate", "--file=-", "--cleanup=whitespace")
		cmd.Stdin = []byte(message)
	} else {
		// tag.gpgSign would otherwise turn a lightweight tag into a signed one.
		cmd.Args = append(cmd.Args, "--no-sign")
	}
	cmd.Args = append(cmd.Args, endOfOptions, name, hash)
	_, err = r.Run(ctx, cmd)
	if err != nil && strings.Contains(KindText(err), "already exists") {
		var gerr *Error
		if errorsAs(err, &gerr) {
			gerr.Kind = KindBranchExists
		}
	}
	return err
}

// KindText returns git's error output of a failed command, for the rare
// check that has nothing better to go on. Empty for other errors.
func KindText(err error) string {
	var gerr *Error
	if errorsAs(err, &gerr) {
		return strings.ToLower(gerr.Stderr)
	}
	return ""
}

// DeleteTag deletes a local tag.
func (r *Runner) DeleteTag(ctx context.Context, dir, name string) error {
	if err := checkName(name, ArgInvalidRev); err != nil {
		return err
	}
	if _, err := r.resolveCommit(ctx, dir, tagRefs+name); err != nil {
		return err
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"tag", "--delete", endOfOptions, name}})
	return err
}

// PushTag sends one tag to a remote.
func (r *Runner) PushTag(ctx context.Context, dir, remote, name string) error {
	if err := r.remoteRequire(ctx, dir, remote); err != nil {
		return err
	}
	if err := checkName(name, ArgInvalidRev); err != nil {
		return err
	}
	ref := tagRefs + name
	res, err := r.Run(ctx, networkCmd(dir, "push", "--porcelain", "--no-follow-tags", endOfOptions, remote, ref+":"+ref))
	if line, found := pushParse(res.Stdout, ref); found && line.flag == '!' {
		var gerr *Error
		if !errorsAs(err, &gerr) {
			gerr = &Error{Args: []string{"push", remote, ref}, ExitCode: res.ExitCode, Stderr: redact(string(res.Stderr))}
		}
		gerr.Kind = line.rejectionKind()
		return gerr
	}
	return err
}
