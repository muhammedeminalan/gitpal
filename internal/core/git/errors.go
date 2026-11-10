package git

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorKind classifies a failed git command so callers can react to the cause
// instead of matching on text.
type ErrorKind string

// Error kinds.
const (
	// KindUnknown is a failure gitpal has no specific explanation for.
	KindUnknown ErrorKind = "unknown"
	// KindGitNotFound means the git executable could not be started.
	KindGitNotFound ErrorKind = "git_not_found"
	// KindDirNotFound means the directory to run in does not exist.
	KindDirNotFound ErrorKind = "dir_not_found"
	// KindTimeout means the command ran longer than allowed and was killed.
	KindTimeout ErrorKind = "timeout"
	// KindCanceled means the caller canceled the command.
	KindCanceled ErrorKind = "canceled"
	// KindNotARepo means the directory is not inside a git repository.
	KindNotARepo ErrorKind = "not_a_repo"
)

// maxErrorText bounds how much of git's output Error.Error prints. The full
// text stays in Error.Stderr.
const maxErrorText = 2000

// Error is a failed git command.
type Error struct {
	Kind ErrorKind
	// Args are the git arguments, with credentials redacted.
	Args []string
	// ExitCode is git's exit status, or -1 if it never exited normally.
	ExitCode int
	// Stderr is what git printed to standard error, trimmed, with
	// credentials redacted.
	Stderr string
	// Err is the underlying error, if any (context or process error).
	Err error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(commandLine(e.Args))
	if e.ExitCode >= 0 {
		fmt.Fprintf(&b, ": exit status %d", e.ExitCode)
	} else if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	if text := truncate(e.Stderr, maxErrorText); text != "" {
		b.WriteString(": ")
		b.WriteString(text)
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Err }

// KindOf returns the kind of a git failure: "" for nil, and KindUnknown for an
// error that did not come from running git.
func KindOf(err error) ErrorKind {
	if err == nil {
		return ""
	}
	var gerr *Error
	if errors.As(err, &gerr) {
		return gerr.Kind
	}
	return KindUnknown
}

// ExitCode returns git's exit status if err is a git command that ran and
// exited non-zero. Some commands use the status as an answer rather than a
// failure (`diff --quiet`, `config --get`).
func ExitCode(err error) (int, bool) {
	var gerr *Error
	if errors.As(err, &gerr) && gerr.ExitCode >= 0 {
		return gerr.ExitCode, true
	}
	return 0, false
}

// stderrKinds maps fragments of git's (English) error output to a kind. The
// first entry with a matching fragment wins, so more specific entries come
// first.
var stderrKinds = []struct {
	kind      ErrorKind
	fragments []string
}{
	{KindNotARepo, []string{"not a git repository"}},
}

func classify(stderr string) ErrorKind {
	text := strings.ToLower(stderr)
	for _, entry := range stderrKinds {
		for _, fragment := range entry.fragments {
			if strings.Contains(text, fragment) {
				return entry.kind
			}
		}
	}
	return KindUnknown
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !isRuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + " …"
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
