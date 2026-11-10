package git

import (
	"bytes"
	"path"
	"strings"
	"unicode"
)

// Conventions shared by every command in this package:
//
//   - dir is the root of the working tree (see TopLevel). Paths given to and
//     returned by this package are relative to it and use forward slashes, the
//     way `git status --porcelain` prints them.
//   - User-supplied paths reach git through pathspecStdin when the command
//     supports --pathspec-from-file, otherwise after a "--" argument. Either
//     way literalPathspecsEnv is set, so "*.go" or ":(exclude)x" name files
//     literally instead of being expanded.
//   - User-supplied names and revisions are checked with checkName and placed
//     after --end-of-options, so a value such as "--force" can never be read
//     as an option.

// literalPathspecsEnv makes git treat every pathspec as a literal path.
const literalPathspecsEnv = "GIT_LITERAL_PATHSPECS=1"

// endOfOptions ends option parsing for the arguments after it (git 2.24+).
const endOfOptions = "--end-of-options"

// splitNUL splits NUL-separated output (`-z`) into fields. A trailing NUL
// terminates the last field rather than starting an empty one.
func splitNUL(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	b = bytes.TrimSuffix(b, []byte{0})
	return strings.Split(string(b), "\x00")
}

// pathspecStdin prepares paths to be read by git from standard input,
// NUL-separated. That avoids command-line length limits (Windows: 32 KiB) and
// any chance of a path being read as an option. It returns the arguments to
// add to the command and the data to feed it on stdin.
func pathspecStdin(paths []string) (args []string, stdin []byte, err error) {
	if err := checkPaths(paths); err != nil {
		return nil, nil, err
	}
	var buf bytes.Buffer
	for _, p := range paths {
		buf.WriteString(p)
		buf.WriteByte(0)
	}
	return []string{"--pathspec-from-file=-", "--pathspec-file-nul"}, buf.Bytes(), nil
}

// checkPaths refuses an empty list and any path that is empty, absolute,
// contains a NUL byte or points outside the working tree.
func checkPaths(paths []string) error {
	if len(paths) == 0 {
		return &ArgError{Code: ArgNoPaths}
	}
	for _, p := range paths {
		if !validRelPath(p) {
			return &ArgError{Code: ArgInvalidPath, Value: p}
		}
	}
	return nil
}

func validRelPath(p string) bool {
	if p == "" || strings.ContainsRune(p, 0) {
		return false
	}
	slashed := strings.ReplaceAll(p, `\`, "/")
	if strings.HasPrefix(slashed, "/") || hasDriveLetter(slashed) {
		return false
	}
	clean := path.Clean(slashed)
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

func hasDriveLetter(p string) bool {
	return len(p) >= 2 && p[1] == ':' && ((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z'))
}

// checkName refuses a branch, remote or revision name that could be mistaken
// for an option or that git would never accept: empty, starting with "-", or
// containing whitespace or control characters. git applies its own, stricter
// rules afterwards; this is the part that must hold before git sees the value.
func checkName(value string, code ArgCode) error {
	if value == "" || strings.HasPrefix(value, "-") ||
		strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return &ArgError{Code: code, Value: value}
	}
	return nil
}

// networkCmd is a command that talks to a remote, so it gets the longer
// timeout.
func networkCmd(dir string, args ...string) Cmd {
	return Cmd{Dir: dir, Args: args, Timeout: NetworkTimeout}
}
