package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ConflictSide picks one side of a conflict.
type ConflictSide string

// Conflict sides, in git's meaning: during a merge "ours" is the current
// branch; during a rebase it is the branch being rebased onto.
const (
	SideOurs   ConflictSide = "ours"
	SideTheirs ConflictSide = "theirs"
)

// ConflictVersions are the three versions of a conflicted file.
type ConflictVersions struct {
	Base, Ours, Theirs          []byte
	HasBase, HasOurs, HasTheirs bool
}

// Conflict returns the common ancestor's, our and their version of a
// conflicted file. A side that deleted the file has no version.
func (r *Runner) Conflict(ctx context.Context, dir, path string) (ConflictVersions, error) {
	if err := checkPaths([]string{path}); err != nil {
		return ConflictVersions{}, err
	}
	var v ConflictVersions
	for stage, dst := range map[int]struct {
		data *[]byte
		has  *bool
	}{1: {&v.Base, &v.HasBase}, 2: {&v.Ours, &v.HasOurs}, 3: {&v.Theirs, &v.HasTheirs}} {
		res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"cat-file", "blob", ":" + strconv.Itoa(stage) + ":" + path}})
		if err != nil {
			if _, exited := ExitCode(err); exited {
				continue // no such stage
			}
			return ConflictVersions{}, err
		}
		*dst.data, *dst.has = res.Stdout, true
	}
	if !v.HasOurs && !v.HasTheirs && !v.HasBase {
		return v, &Error{Kind: KindPathspecNoMatch, Args: []string{"cat-file", path}, ExitCode: -1}
	}
	return v, nil
}

// ResolveConflict settles conflicted files by taking one side whole, and
// marks them resolved. Where the chosen side deleted the file, it is removed.
func (r *Runner) ResolveConflict(ctx context.Context, dir string, paths []string, side ConflictSide) error {
	if side != SideOurs && side != SideTheirs {
		return &ArgError{Code: ArgInvalidRev, Value: string(side)}
	}
	if err := checkPaths(paths); err != nil {
		return err
	}
	var keep, remove []string
	for _, p := range paths {
		v, err := r.Conflict(ctx, dir, p)
		if err != nil {
			return err
		}
		if (side == SideOurs && v.HasOurs) || (side == SideTheirs && v.HasTheirs) {
			keep = append(keep, p)
		} else {
			remove = append(remove, p)
		}
	}
	if len(keep) > 0 {
		if err := r.runWithPaths(ctx, dir, keep, "checkout", "--"+string(side)); err != nil {
			return err
		}
		if err := r.Stage(ctx, dir, keep); err != nil {
			return err
		}
	}
	if len(remove) > 0 {
		return r.runWithPaths(ctx, dir, remove, "rm", "--quiet", "--force")
	}
	return nil
}

// ReflogEntry is one movement of a ref.
type ReflogEntry struct {
	// Selector names the entry, e.g. "HEAD@{2}".
	Selector string
	Hash     string
	// Action is git's description, e.g. "commit: add login" or
	// "reset: moving to HEAD~1".
	Action string
	Time   time.Time
}

// Reflog lists where a ref (HEAD when empty) has pointed, newest first: the
// place to find commits that no branch holds any more.
func (r *Runner) Reflog(ctx context.Context, dir, ref string, maxCount int) ([]ReflogEntry, error) {
	if ref == "" {
		ref = "HEAD"
	}
	if err := checkName(ref, ArgInvalidRev); err != nil {
		return nil, err
	}
	args := []string{
		"log", "--walk-reflogs", "--date=unix", "--no-color", "--no-show-signature", "--encoding=UTF-8",
		"--format=%H%x00%gd%x00%gs", "-z",
	}
	if maxCount > 0 {
		args = append(args, "--max-count="+strconv.Itoa(maxCount))
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: append(args, endOfOptions, ref, "--")})
	if err != nil {
		return nil, err
	}
	fields := splitNUL(res.Stdout)
	if len(fields)%3 != 0 {
		return nil, errors.New("git reflog: unexpected output")
	}
	entries := make([]ReflogEntry, 0, len(fields)/3)
	for i := 0; i < len(fields); i += 3 {
		e := ReflogEntry{Hash: fields[i], Action: fields[i+2], Selector: ref + "@{" + strconv.Itoa(i/3) + "}"}
		// %gd under --date=unix is "ref@{<seconds>}".
		if open := strings.LastIndex(fields[i+1], "@{"); open >= 0 {
			if secs, err := strconv.ParseInt(strings.TrimSuffix(fields[i+1][open+2:], "}"), 10, 64); err == nil {
				e.Time = time.Unix(secs, 0)
			}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// BlameLine is one line of a file with the commit that last changed it.
type BlameLine struct {
	Hash   string
	Author string
	Time   time.Time
	// Line is the line number in the file, from 1.
	Line    int
	Summary string
	Text    string
}

// Blame tells, line by line, which commit last changed a file at rev (the
// working tree when empty).
func (r *Runner) Blame(ctx context.Context, dir, path, rev string) ([]BlameLine, error) {
	if err := checkPaths([]string{path}); err != nil {
		return nil, err
	}
	args := []string{"blame", "--porcelain"}
	if rev != "" {
		if err := checkName(rev, ArgInvalidRev); err != nil {
			return nil, err
		}
		args = append(args, rev)
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: append(args, "--", path), Env: []string{literalPathspecsEnv}})
	if err != nil {
		return nil, err
	}
	return parseBlame(res.Stdout)
}

func parseBlame(out []byte) ([]BlameLine, error) {
	type meta struct {
		author, summary string
		at              time.Time
	}
	known := map[string]*meta{}
	var lines []BlameLine
	var current *BlameLine
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(nil, 16<<20)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "\t") {
			if current == nil {
				return nil, errors.New("git blame: line without a header")
			}
			m := known[current.Hash]
			current.Author, current.Summary, current.Time, current.Text = m.author, m.summary, m.at, text[1:]
			lines = append(lines, *current)
			current = nil
			continue
		}
		key, value, _ := strings.Cut(text, " ")
		if current == nil {
			// A header: "<hash> <orig-line> <final-line> [<count>]".
			fields := strings.Fields(text)
			if len(fields) < 3 || len(fields[0]) < 40 {
				return nil, errors.New("git blame: malformed header")
			}
			n, err := strconv.Atoi(fields[2])
			if err != nil {
				return nil, errors.New("git blame: malformed line number")
			}
			current = &BlameLine{Hash: fields[0], Line: n}
			if known[current.Hash] == nil {
				known[current.Hash] = &meta{}
			}
			continue
		}
		m := known[current.Hash]
		switch key {
		case "author":
			m.author = value
		case "author-time":
			if secs, err := strconv.ParseInt(value, 10, 64); err == nil {
				m.at = time.Unix(secs, 0)
			}
		case "summary":
			m.summary = value
		}
	}
	return lines, scanner.Err()
}
