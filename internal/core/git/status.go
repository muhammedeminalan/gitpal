package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FileState is one column of the two-letter "XY" state git reports for a
// file: X describes the index, Y the working tree.
type FileState byte

// File states.
const (
	StateUnmodified  FileState = '.'
	StateModified    FileState = 'M'
	StateTypeChanged FileState = 'T'
	StateAdded       FileState = 'A'
	StateDeleted     FileState = 'D'
	StateRenamed     FileState = 'R'
	StateCopied      FileState = 'C'
	StateUnmerged    FileState = 'U'
)

// EntryKind says what kind of record a status entry is.
type EntryKind string

// Entry kinds.
const (
	// EntryChanged is a tracked file that differs in the index, the working
	// tree or both.
	EntryChanged EntryKind = "changed"
	// EntryRenamed is a renamed or copied file; OrigPath holds the source.
	EntryRenamed EntryKind = "renamed"
	// EntryUnmerged is a file with unresolved conflicts.
	EntryUnmerged EntryKind = "unmerged"
	// EntryUntracked is a file git does not know.
	EntryUntracked EntryKind = "untracked"
	// EntryIgnored is a file matched by an ignore rule.
	EntryIgnored EntryKind = "ignored"
)

// StatusEntry is one file in the status.
type StatusEntry struct {
	Kind EntryKind
	Path string
	// OrigPath is the source of a rename or copy.
	OrigPath string
	// Index and Worktree are the two state columns. For an unmerged entry they
	// describe each side of the conflict instead (e.g. "DU": deleted by us).
	// Both are zero for untracked and ignored entries.
	Index, Worktree FileState
	// Score is the similarity, in percent, of a rename or copy.
	Score int
	// Submodule is set when the entry is a submodule rather than a file.
	Submodule bool
}

// Staged reports whether the entry has a change in the index, ready to be
// committed.
func (e StatusEntry) Staged() bool {
	return e.tracked() && e.Index != StateUnmodified
}

// Unstaged reports whether the entry has a change in the working tree that is
// not in the index yet.
func (e StatusEntry) Unstaged() bool {
	return e.tracked() && e.Worktree != StateUnmodified
}

// Conflicted reports whether the entry has unresolved conflicts.
func (e StatusEntry) Conflicted() bool { return e.Kind == EntryUnmerged }

func (e StatusEntry) tracked() bool { return e.Kind == EntryChanged || e.Kind == EntryRenamed }

// BranchStatus describes HEAD and how it relates to its upstream.
type BranchStatus struct {
	// Head is the branch name; empty when HEAD is detached.
	Head     string
	Detached bool
	// Unborn is set when the branch has no commits yet.
	Unborn bool
	// OID is the commit HEAD points at; empty when unborn.
	OID string
	// Upstream is the upstream branch, e.g. "origin/main"; empty if none.
	Upstream string
	// UpstreamGone is set when an upstream is configured but no longer exists
	// (the remote branch was deleted).
	UpstreamGone  bool
	Ahead, Behind int
}

// Operation is a multi-step git operation that has stopped half way.
type Operation string

// Operations.
const (
	OpNone       Operation = ""
	OpMerge      Operation = "merge"
	OpRebase     Operation = "rebase"
	OpAm         Operation = "am"
	OpCherryPick Operation = "cherry-pick"
	OpRevert     Operation = "revert"
	OpBisect     Operation = "bisect"
)

// Status is the state of a working tree.
type Status struct {
	Branch    BranchStatus
	Entries   []StatusEntry
	Operation Operation
}

// Clean reports whether there is nothing to commit and nothing untracked.
// Ignored files do not make a working tree dirty.
func (s Status) Clean() bool {
	for _, e := range s.Entries {
		if e.Kind != EntryIgnored {
			return false
		}
	}
	return true
}

// Staged returns the entries with a change in the index.
func (s Status) Staged() []StatusEntry { return s.filter(StatusEntry.Staged) }

// Unstaged returns the tracked entries with a change in the working tree only.
func (s Status) Unstaged() []StatusEntry { return s.filter(StatusEntry.Unstaged) }

// Untracked returns the files git does not know.
func (s Status) Untracked() []StatusEntry {
	return s.filter(func(e StatusEntry) bool { return e.Kind == EntryUntracked })
}

// Conflicted returns the entries with unresolved conflicts.
func (s Status) Conflicted() []StatusEntry { return s.filter(StatusEntry.Conflicted) }

func (s Status) filter(keep func(StatusEntry) bool) []StatusEntry {
	var out []StatusEntry
	for _, e := range s.Entries {
		if keep(e) {
			out = append(out, e)
		}
	}
	return out
}

// UntrackedMode chooses how untracked files are listed.
type UntrackedMode string

// Untracked modes.
const (
	// UntrackedAll lists every untracked file individually. It is the default.
	UntrackedAll UntrackedMode = "all"
	// UntrackedNormal lists a wholly untracked directory as one entry ("dir/").
	UntrackedNormal UntrackedMode = "normal"
	// UntrackedNone lists no untracked files.
	UntrackedNone UntrackedMode = "no"
)

// StatusOptions adjusts Status.
type StatusOptions struct {
	// Untracked defaults to UntrackedAll.
	Untracked UntrackedMode
	// Ignored also lists ignored files.
	Ignored bool
}

// Status reads the state of the working tree rooted at dir, including an
// operation that stopped half way.
//
// The flags pin down everything the user's configuration could change:
// rename detection (status.renames, diff.renames), untracked files
// (status.showUntrackedFiles), ahead/behind counting (status.aheadBehind) and
// columns (column.ui). status.showStash adds a header the parser skips.
// GIT_OPTIONAL_LOCKS=0 keeps a refresh in the background from taking the
// index lock away from a command the user is running.
func (r *Runner) Status(ctx context.Context, dir string, opts StatusOptions) (Status, error) {
	untracked := opts.Untracked
	if untracked == "" {
		untracked = UntrackedAll
	}
	ignored := "no"
	if opts.Ignored {
		ignored = "traditional"
	}
	res, err := r.Run(ctx, Cmd{
		Dir: dir,
		Args: []string{
			"status", "--porcelain=v2", "-z", "--branch", "--ahead-behind", "--renames", "--no-column",
			"--untracked-files=" + string(untracked), "--ignored=" + ignored,
		},
		Env: []string{"GIT_OPTIONAL_LOCKS=0"},
	})
	if err != nil {
		return Status{}, err
	}
	st, err := ParseStatus(res.Stdout)
	if err != nil {
		return Status{}, err
	}
	if st.Operation, err = r.InProgress(ctx, dir); err != nil {
		return Status{}, err
	}
	return st, nil
}

// ParseStatus parses the output of `git status --porcelain=v2 -z --branch`.
//
// Errors describe the shape of the offending record, never its content: a
// path is repository data and does not belong in an error message.
func ParseStatus(out []byte) (Status, error) {
	var st Status
	hasAheadBehind := false
	records := splitNUL(out)
	for i := 0; i < len(records); i++ {
		rec := records[i]
		if rec == "" {
			continue
		}
		var entry StatusEntry
		var err error
		switch rec[0] {
		case '#':
			var ab bool
			if ab, err = statusParseHeader(rec, &st.Branch); err != nil {
				return Status{}, err
			}
			hasAheadBehind = hasAheadBehind || ab
			continue
		case '1':
			entry, err = statusParseTracked(rec, EntryChanged, 9)
		case '2':
			// With -z the original path follows as a record of its own.
			if i+1 >= len(records) {
				return Status{}, errors.New("git status: rename record without its original path")
			}
			if entry, err = statusParseTracked(rec, EntryRenamed, 10); err == nil {
				i++
				entry.OrigPath = records[i]
			}
		case 'u':
			entry, err = statusParseTracked(rec, EntryUnmerged, 11)
		case '?':
			entry, err = statusParsePathOnly(rec, EntryUntracked)
		case '!':
			entry, err = statusParsePathOnly(rec, EntryIgnored)
		default:
			err = fmt.Errorf("git status: unknown record type %q", rec[0])
		}
		if err != nil {
			return Status{}, err
		}
		st.Entries = append(st.Entries, entry)
	}
	st.Branch.UpstreamGone = st.Branch.Upstream != "" && !hasAheadBehind
	return st, nil
}

// statusParseHeader reads one "# key value" line into b. It reports whether
// the line was the ahead/behind header. Headers it does not know (such as
// "# stash 2") are skipped, so a newer git does not break the parser.
func statusParseHeader(rec string, b *BranchStatus) (aheadBehind bool, err error) {
	key, value, _ := strings.Cut(strings.TrimPrefix(rec, "# "), " ")
	switch key {
	case "branch.oid":
		if value == "(initial)" {
			b.Unborn = true
		} else {
			b.OID = value
		}
	case "branch.head":
		if value == "(detached)" {
			b.Detached = true
		} else {
			b.Head = value
		}
	case "branch.upstream":
		b.Upstream = value
	case "branch.ab":
		ahead, behind, ok := strings.Cut(value, " ")
		if !ok || !strings.HasPrefix(ahead, "+") || !strings.HasPrefix(behind, "-") {
			return false, errors.New("git status: malformed ahead/behind header")
		}
		if b.Ahead, err = strconv.Atoi(ahead[1:]); err != nil {
			return false, errors.New("git status: malformed ahead count")
		}
		if b.Behind, err = strconv.Atoi(behind[1:]); err != nil {
			return false, errors.New("git status: malformed behind count")
		}
		return true, nil
	}
	return false, nil
}

// statusParseTracked parses an ordinary ("1"), rename/copy ("2") or unmerged
// ("u") record. They share the first three fields (type, XY, submodule state)
// and end with the path, which may itself contain spaces; fields is the total
// number of space-separated fields of that record type.
func statusParseTracked(rec string, kind EntryKind, fields int) (StatusEntry, error) {
	parts := strings.SplitN(rec, " ", fields)
	if len(parts) != fields || len(parts[1]) != 2 || parts[fields-1] == "" {
		return StatusEntry{}, fmt.Errorf("git status: malformed %q record", rec[0])
	}
	entry := StatusEntry{
		Kind:      kind,
		Path:      parts[fields-1],
		Index:     FileState(parts[1][0]),
		Worktree:  FileState(parts[1][1]),
		Submodule: strings.HasPrefix(parts[2], "S"),
	}
	if kind == EntryRenamed {
		// "R87" or "C100": the letter repeats the state, the number is the score.
		score := parts[fields-2]
		if len(score) < 2 {
			return StatusEntry{}, errors.New("git status: malformed rename score")
		}
		n, err := strconv.Atoi(score[1:])
		if err != nil {
			return StatusEntry{}, errors.New("git status: malformed rename score")
		}
		entry.Score = n
	}
	return entry, nil
}

func statusParsePathOnly(rec string, kind EntryKind) (StatusEntry, error) {
	if len(rec) < 3 || rec[1] != ' ' {
		return StatusEntry{}, fmt.Errorf("git status: malformed %q record", rec[0])
	}
	return StatusEntry{Kind: kind, Path: rec[2:]}, nil
}

// statusOperationFiles are the files and directories git keeps in its
// directory while an operation is under way, in the order they are checked:
// a rebase comes first because one that stops on a conflict can leave
// cherry-pick state behind as well.
var statusOperationFiles = []string{
	"rebase-merge", "rebase-apply", "rebase-apply/applying",
	"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG",
}

// InProgress reports the operation that stopped half way in the working tree
// rooted at dir, if any.
//
// The state files are located with `rev-parse --git-path`, so the answer is
// right in a linked worktree (which keeps its own state under
// .git/worktrees/<name>) and when the git directory lives elsewhere.
func (r *Runner) InProgress(ctx context.Context, dir string) (Operation, error) {
	args := []string{"rev-parse"}
	for _, name := range statusOperationFiles {
		args = append(args, "--git-path", name)
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: args})
	if err != nil {
		return OpNone, err
	}
	lines := strings.Split(strings.TrimSuffix(string(res.Stdout), "\n"), "\n")
	if len(lines) != len(statusOperationFiles) {
		return OpNone, errors.New("git rev-parse: unexpected number of paths")
	}
	exists := make(map[string]bool, len(lines))
	for i, line := range lines {
		path := filepath.FromSlash(strings.TrimSuffix(line, "\r"))
		if !filepath.IsAbs(path) {
			// git prints these relative to the directory it ran in.
			path = filepath.Join(dir, path)
		}
		if _, err := os.Stat(path); err == nil {
			exists[statusOperationFiles[i]] = true
		}
	}
	switch {
	case exists["rebase-merge"]:
		return OpRebase, nil
	case exists["rebase-apply/applying"]:
		return OpAm, nil
	case exists["rebase-apply"]:
		return OpRebase, nil
	case exists["MERGE_HEAD"]:
		return OpMerge, nil
	case exists["CHERRY_PICK_HEAD"]:
		return OpCherryPick, nil
	case exists["REVERT_HEAD"]:
		return OpRevert, nil
	case exists["BISECT_LOG"]:
		return OpBisect, nil
	}
	return OpNone, nil
}
