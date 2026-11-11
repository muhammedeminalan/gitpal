// Package history records every operation gitpal performs in a repository, so
// the last one can be undone (spec §7.4).
//
// The record lives in the repository's git directory
// (.git/gitpal/history.jsonl), one JSON object per line. It holds commit
// hashes, branch names, backup refs and file paths — never file content,
// commit messages or credentials.
package history

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// FileName is the history file's path inside the git directory.
const FileName = "gitpal/history.jsonl"

// OpUndo is the operation recorded when another one is undone.
const OpUndo = "undo"

// Entry is one operation.
type Entry struct {
	// ID identifies the entry; Append fills it in.
	ID   string    `json:"id"`
	Time time.Time `json:"time"`
	// Op names the operation: "commit", "discard", "push", …
	Op string `json:"op"`
	// Branch is the branch that was checked out; empty on a detached HEAD.
	Branch string `json:"branch,omitempty"`
	// Before and After are HEAD before and after the operation.
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	// Backup is the ref that kept the commit at risk; WorktreeBackup the ref
	// that kept the uncommitted changes at risk.
	Backup         string `json:"backup,omitempty"`
	WorktreeBackup string `json:"worktree_backup,omitempty"`
	// Target is the other branch or revision involved: the branch deleted or
	// switched to, the revision reset to, the remote pushed to.
	Target string `json:"target,omitempty"`
	// Detail is the variant of the operation an undo needs to know: the mode
	// of a reset, the strategy of a pull.
	Detail string `json:"detail,omitempty"`
	// Paths are the files the operation was limited to.
	Paths []string `json:"paths,omitempty"`
	// Failed is set when the operation did not complete: the kind of the
	// error, never its text.
	Failed string `json:"failed,omitempty"`
	// Undoes is the ID of the entry this one undid (Op is OpUndo).
	Undoes string `json:"undoes,omitempty"`
}

// Store reads and appends to a history file.
type Store struct {
	path string
	// MaxBytes is the size past which the oldest entries are dropped, down
	// to about half of it. 0 means DefaultMaxBytes.
	MaxBytes int64
}

// DefaultMaxBytes keeps a few thousand entries: far more than undo or the
// history view ever look at.
const DefaultMaxBytes = 1 << 20

// Open returns the store for the file at path. Nothing is created until the
// first Append.
func Open(path string) *Store { return &Store{path: path} }

// sequence tells entries apart that are appended in the same instant.
var sequence atomic.Uint64

// Append adds an entry and returns it as written. Time and ID are filled in
// when empty.
func (s *Store) Append(e Entry) (Entry, error) {
	if e.Op == "" {
		return Entry{}, errors.New("history: entry without an operation")
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	// Round-trips through JSON unchanged: no monotonic clock reading, UTC.
	e.Time = e.Time.UTC().Round(0)
	if e.ID == "" {
		e.ID = fmt.Sprintf("%s-%04d", e.Time.Format("20060102T150405.000000000Z"), sequence.Add(1)%10000)
	}

	line, err := json.Marshal(e)
	if err != nil {
		return Entry{}, fmt.Errorf("history: encode entry: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return Entry{}, fmt.Errorf("history: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Entry{}, fmt.Errorf("history: %w", err)
	}
	// One write per entry: an append this small is not interleaved with
	// another process's.
	_, werr := f.Write(append(line, '\n'))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return Entry{}, fmt.Errorf("history: %w", werr)
	}
	s.trim()
	return e, nil
}

// trim drops the oldest entries once the file has grown past MaxBytes. The
// file is rewritten through a temporary file and a rename, so a reader sees
// either the old or the new file. An entry another gitpal appends between the
// read and the rename is lost; that needs two gitpal processes writing to the
// same repository at the same instant, and only costs history, never a
// backup. A failure leaves the file as it was.
func (s *Store) trim() {
	limit := s.MaxBytes
	if limit <= 0 {
		limit = DefaultMaxBytes
	}
	info, err := os.Stat(s.path)
	if err != nil || info.Size() <= limit {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	// Keep whole lines from the end, up to half the limit.
	keep := data
	for int64(len(keep)) > limit/2 {
		i := bytes.IndexByte(keep, '\n')
		if i < 0 || i == len(keep)-1 {
			break
		}
		keep = keep[i+1:]
	}
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, keep, 0o600) != nil { //nolint:gosec // the path is gitpal's own history file inside .git, not user input
		return
	}
	if os.Rename(tmp, s.path) != nil {
		_ = os.Remove(tmp)
	}
}

// Entries returns every readable entry, oldest first. Lines that cannot be
// read are skipped: a damaged line must not take the rest of the history
// with it.
func (s *Store) Entries() ([]Entry, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	var entries []Entry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(nil, len(data)+1)
	for scanner.Scan() {
		var e Entry
		if json.Unmarshal(scanner.Bytes(), &e) != nil || e.Op == "" || e.ID == "" {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// LastUndoable returns the newest operation that completed and has not been
// undone. Undo entries themselves are not candidates.
func (s *Store) LastUndoable() (Entry, bool, error) {
	entries, err := s.Entries()
	if err != nil {
		return Entry{}, false, err
	}
	undone := map[string]bool{}
	for _, e := range entries {
		if e.Op == OpUndo && e.Failed == "" {
			undone[e.Undoes] = true
		}
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Op == OpUndo || e.Failed != "" || undone[e.ID] {
			continue
		}
		return e, true, nil
	}
	return Entry{}, false, nil
}
