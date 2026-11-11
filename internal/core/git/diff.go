package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// DiffOptions chooses which two states are compared.
type DiffOptions struct {
	// Staged compares the index with HEAD (with nothing, before the first
	// commit). Otherwise the working tree is compared with the index.
	Staged bool
	// Commit, when set, shows what that commit changed relative to its first
	// parent, or to nothing for a root commit. Staged is then ignored.
	Commit string
	// Paths limits the diff to these files or directories.
	Paths []string
	// Context is the number of unchanged lines shown around a change. Zero
	// means git's default of 3.
	Context int
}

// ChangeType says what happened to a file.
type ChangeType string

// Change types.
const (
	ChangeAdded       ChangeType = "added"
	ChangeDeleted     ChangeType = "deleted"
	ChangeModified    ChangeType = "modified"
	ChangeRenamed     ChangeType = "renamed"
	ChangeCopied      ChangeType = "copied"
	ChangeTypeChanged ChangeType = "type-changed"
)

// LineKind is the marker in the first column of a diff line.
type LineKind byte

// Line kinds.
const (
	LineContext LineKind = ' '
	LineAdded   LineKind = '+'
	LineDeleted LineKind = '-'
	// LineNoNewline is git's "\ No newline at end of file" note about the
	// line before it. It is not a line of the file.
	LineNoNewline LineKind = '\\'
)

// DiffLine is one line of a hunk.
type DiffLine struct {
	Kind LineKind
	// Text is the line without its marker and without the trailing newline.
	// A carriage return at the end of a CRLF line is kept.
	Text string
	// OldLine and NewLine are 1-based line numbers in the old and new file;
	// zero where the line does not exist on that side.
	OldLine, NewLine int
}

// Hunk is one "@@ … @@" block of changes.
type Hunk struct {
	OldStart, OldLines int
	NewStart, NewLines int
	// Section is the text after the second "@@": usually the enclosing
	// function.
	Section string
	Lines   []DiffLine
}

// FileDiff is the change to one file.
type FileDiff struct {
	// Path is the file's path after the change; for a deletion, the path it
	// had.
	Path string
	// OldPath is set for renames and copies only.
	OldPath string
	Change  ChangeType
	// Binary files have no hunks.
	Binary bool
	// OldMode and NewMode are git file modes such as "100644". They are equal
	// when the mode did not change; one is empty for an added or deleted file.
	OldMode, NewMode string
	// Similarity is the percentage git reports for a rename or copy.
	Similarity int
	Hunks      []Hunk
}

// FileStat is the size of the change to one file.
type FileStat struct {
	Path string
	// OldPath is set for renames and copies only.
	OldPath        string
	Added, Deleted int
	// Binary files have no line counts.
	Binary bool
}

// diffFlags pin down everything the user's configuration could change about
// the output: prefixes (diff.noprefix, diff.mnemonicPrefix), external tools
// (diff.external, textconv filters), color, rename detection (diff.renames),
// hunk merging (diff.interHunkContext), submodule format (diff.submodule),
// path trimming (diff.relative; --no-relative is git 2.28+) and hash length
// (core.abbrev). The context size is always passed as well (diff.context).
//
// Left to the user on purpose: diff.orderFile (only the order of files),
// diff.algorithm (which lines are shown as changed, all of them correct) and
// diff.ignoreSubmodules.
var diffFlags = []string{
	"--no-color", "--no-ext-diff", "--no-textconv", "--src-prefix=a/", "--dst-prefix=b/",
	"--find-renames", "--inter-hunk-context=0", "--submodule=short", "--no-relative", "--full-index",
}

// Diff returns the parsed changes.
func (r *Runner) Diff(ctx context.Context, dir string, opts DiffOptions) ([]FileDiff, error) {
	raw, err := r.DiffRaw(ctx, dir, opts)
	if err != nil {
		return nil, err
	}
	return ParseDiff(raw)
}

// DiffRaw returns the unified diff exactly as git printed it.
func (r *Runner) DiffRaw(ctx context.Context, dir string, opts DiffOptions) ([]byte, error) {
	unified := opts.Context
	if unified <= 0 {
		unified = 3
	}
	res, err := r.diffRun(ctx, dir, opts, "--patch", "--unified="+strconv.Itoa(unified))
	return res.Stdout, err
}

// DiffStat returns the number of added and deleted lines per file.
//
// The context size is not passed here: `--unified` implies `--patch`, and the
// counts do not depend on it.
func (r *Runner) DiffStat(ctx context.Context, dir string, opts DiffOptions) ([]FileStat, error) {
	res, err := r.diffRun(ctx, dir, opts, "--numstat", "-z")
	if err != nil {
		return nil, err
	}
	return diffParseStat(res.Stdout)
}

func (r *Runner) diffRun(ctx context.Context, dir string, opts DiffOptions, format ...string) (Result, error) {
	args := append([]string{"diff"}, diffFlags...)
	args = append(args, format...)

	switch {
	case opts.Commit != "":
		from, to, err := r.diffCommitRange(ctx, dir, opts.Commit)
		if err != nil {
			return Result{}, err
		}
		args = append(args, from, to)
	default:
		// Outside a repository `git diff` turns into a file comparison tool
		// with error messages of its own; make sure there is one first.
		if _, _, err := r.Head(ctx, dir); err != nil {
			return Result{}, err
		}
		if opts.Staged {
			args = append(args, "--cached")
		}
	}

	// git diff cannot read paths from stdin, so they follow "--".
	args = append(args, "--")
	if len(opts.Paths) > 0 {
		if err := checkPaths(opts.Paths); err != nil {
			return Result{}, err
		}
		args = append(args, opts.Paths...)
	}
	return r.Run(ctx, Cmd{Dir: dir, Args: args, Env: []string{literalPathspecsEnv}})
}

// diffCommitRange returns the two trees to compare to see what rev changed:
// its first parent and itself. A root commit is compared with the empty tree,
// whose id is asked from git rather than written down, because it depends on
// the repository's hash algorithm.
func (r *Runner) diffCommitRange(ctx context.Context, dir, rev string) (from, to string, err error) {
	to, err = r.resolveCommit(ctx, dir, rev)
	if err != nil {
		return "", "", err
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"rev-parse", "--verify", "--quiet", to + "^1^{commit}"}})
	if err == nil {
		return strings.TrimSpace(string(res.Stdout)), to, nil
	}
	if code, exited := ExitCode(err); !exited || code != 1 {
		return "", "", err
	}
	res, err = r.Run(ctx, Cmd{Dir: dir, Args: []string{"hash-object", "-t", "tree", "--stdin"}, Stdin: []byte{}})
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(string(res.Stdout)), to, nil
}

// ParseDiff parses a unified diff as `git diff` prints it with the standard
// "a/" and "b/" prefixes.
//
// Errors describe the shape of the problem, never the text around it: a diff
// is file content and does not belong in an error message.
func ParseDiff(raw []byte) ([]FileDiff, error) {
	p := diffParser{lines: strings.Split(string(raw), "\n")}
	// The text ends with a newline, so the last element is empty.
	if n := len(p.lines); n > 0 && p.lines[n-1] == "" {
		p.lines = p.lines[:n-1]
	}

	var files []FileDiff
	for p.more() {
		if !strings.HasPrefix(p.peek(), "diff --git ") {
			return nil, p.errorf("expected a file header")
		}
		fd, err := p.file()
		if err != nil {
			return nil, err
		}
		files = append(files, fd)
	}
	return files, nil
}

type diffParser struct {
	lines []string
	pos   int
}

func (p *diffParser) more() bool   { return p.pos < len(p.lines) }
func (p *diffParser) peek() string { return p.lines[p.pos] }

func (p *diffParser) errorf(format string, args ...any) error {
	return fmt.Errorf("git diff: line %d: %s", p.pos+1, fmt.Sprintf(format, args...))
}

// diffHeader collects what the lines before the first hunk say about a file.
type diffHeader struct {
	gitLine            string // what follows "diff --git "
	oldName, newName   string // from "--- " and "+++ "; "" when absent or /dev/null
	fromName, toName   string // from "rename/copy from/to"
	added, deleted     bool
	renamed, copied    bool
	oldMode, newMode   string
	hasOldNew, hasFrom bool
}

// file parses one file: its header lines, then its hunks.
func (p *diffParser) file() (FileDiff, error) {
	h := diffHeader{gitLine: strings.TrimPrefix(p.peek(), "diff --git ")}
	p.pos++

	var fd FileDiff
	inHeader := true
	for inHeader && p.more() {
		line := p.peek()
		key, value, _ := strings.Cut(line, " ")
		var err error
		switch {
		case strings.HasPrefix(line, "diff --git "), strings.HasPrefix(line, "@@ "):
			inHeader = false
			continue
		case strings.HasPrefix(line, "old mode "):
			h.oldMode = line[len("old mode "):]
		case strings.HasPrefix(line, "new mode "):
			h.newMode = line[len("new mode "):]
		case strings.HasPrefix(line, "deleted file mode "):
			h.deleted, h.oldMode = true, line[len("deleted file mode "):]
		case strings.HasPrefix(line, "new file mode "):
			h.added, h.newMode = true, line[len("new file mode "):]
		case strings.HasPrefix(line, "rename from "):
			h.renamed, h.hasFrom = true, true
			h.fromName, err = diffUnquote(line[len("rename from "):])
		case strings.HasPrefix(line, "rename to "):
			h.toName, err = diffUnquote(line[len("rename to "):])
		case strings.HasPrefix(line, "copy from "):
			h.copied, h.hasFrom = true, true
			h.fromName, err = diffUnquote(line[len("copy from "):])
		case strings.HasPrefix(line, "copy to "):
			h.toName, err = diffUnquote(line[len("copy to "):])
		case strings.HasPrefix(line, "similarity index "):
			fd.Similarity, err = strconv.Atoi(strings.TrimSuffix(line[len("similarity index "):], "%"))
		case strings.HasPrefix(line, "dissimilarity index "):
		case key == "index":
			// "index <old>..<new> <mode>": the mode is there when unchanged.
			if _, mode, ok := strings.Cut(value, " "); ok && !h.added && !h.deleted {
				h.oldMode, h.newMode = mode, mode
			}
		case key == "---":
			h.hasOldNew = true
			h.oldName, err = diffSideName(value, "a/")
		case key == "+++":
			h.newName, err = diffSideName(value, "b/")
		case strings.HasPrefix(line, "Binary files "):
			fd.Binary = true
		case line == "GIT binary patch":
			fd.Binary = true
			p.skipBinaryPatch()
			inHeader = false
			continue
		default:
			return FileDiff{}, p.errorf("unexpected line in a file header")
		}
		if err != nil {
			return FileDiff{}, p.errorf("malformed file header")
		}
		p.pos++
	}

	if err := h.resolve(&fd); err != nil {
		return FileDiff{}, p.errorf("%s", err)
	}

	for p.more() && strings.HasPrefix(p.peek(), "@@ ") {
		hunk, err := p.hunk()
		if err != nil {
			return FileDiff{}, err
		}
		fd.Hunks = append(fd.Hunks, hunk)
	}
	if p.more() && !strings.HasPrefix(p.peek(), "diff --git ") {
		return FileDiff{}, p.errorf("unexpected line after a hunk")
	}
	return fd, nil
}

// skipBinaryPatch moves past the encoded data of a "GIT binary patch". None of
// its lines can start a new file.
func (p *diffParser) skipBinaryPatch() {
	for p.more() && !strings.HasPrefix(p.peek(), "diff --git ") {
		p.pos++
	}
}

// resolve works out the file's paths and change type from its header.
//
// The unambiguous lines are preferred: "rename from/to", "copy from/to", then
// "--- a/…" and "+++ b/…". A change without content (a mode change, an empty
// or binary file) has none of them, which leaves only the "diff --git" line.
func (h *diffHeader) resolve(fd *FileDiff) error {
	fd.OldMode, fd.NewMode = h.oldMode, h.newMode
	switch {
	case h.added:
		fd.Change = ChangeAdded
	case h.deleted:
		fd.Change = ChangeDeleted
	case h.renamed:
		fd.Change = ChangeRenamed
	case h.copied:
		fd.Change = ChangeCopied
	case diffFileType(h.oldMode) != diffFileType(h.newMode):
		fd.Change = ChangeTypeChanged
	default:
		fd.Change = ChangeModified
	}

	switch {
	case h.hasFrom:
		if h.fromName == "" || h.toName == "" {
			return errors.New("rename or copy without both names")
		}
		fd.OldPath, fd.Path = h.fromName, h.toName
	case h.hasOldNew && (h.oldName != "" || h.newName != ""):
		fd.Path = h.newName
		if fd.Path == "" {
			fd.Path = h.oldName
		}
	default:
		name, err := diffNameFromGitLine(h.gitLine)
		if err != nil {
			return err
		}
		fd.Path = name
	}
	return nil
}

// diffFileType returns the part of a mode that says what kind of object it is
// ("100" file, "120" link, "160" submodule).
func diffFileType(mode string) string {
	if len(mode) < 3 {
		return ""
	}
	return mode[:3]
}

// diffSideName reads the name from a "--- " or "+++ " line. It returns "" for
// /dev/null, the side that does not exist.
func diffSideName(value, prefix string) (string, error) {
	if value == "/dev/null" {
		return "", nil
	}
	if strings.HasPrefix(value, `"`) {
		name, err := diffUnquote(value)
		if err != nil || !strings.HasPrefix(name, prefix) {
			return "", errors.New("malformed name")
		}
		return name[len(prefix):], nil
	}
	if !strings.HasPrefix(value, prefix) {
		return "", errors.New("malformed name")
	}
	// git adds a tab after a name containing a space, so that tools reading
	// the line can tell where the name ends.
	return strings.TrimSuffix(value[len(prefix):], "\t"), nil
}

// diffNameFromGitLine reads the name from what follows "diff --git ", for a
// file whose two names are the same (anything else has rename or copy lines).
//
// Unquoted, the line is `a/NAME b/NAME`, which cannot be split at " b/" when
// NAME itself contains it. Both halves being equal, it is split in the
// middle, as `git apply` does.
func diffNameFromGitLine(rest string) (string, error) {
	malformed := errors.New("cannot read the file name")
	if strings.HasPrefix(rest, `"`) {
		end := diffQuoteEnd(rest)
		if end < 0 {
			return "", malformed
		}
		name, err := diffUnquote(rest[:end+1])
		if err != nil || !strings.HasPrefix(name, "a/") {
			return "", malformed
		}
		return name[2:], nil
	}
	n := (len(rest) - len("a/ b/")) / 2
	if n <= 0 || len(rest) != 2*n+5 {
		return "", malformed
	}
	name := rest[2 : 2+n]
	if rest != "a/"+name+" b/"+name {
		return "", malformed
	}
	return name, nil
}

// diffQuoteEnd returns the index of the quote that closes the C-style string
// starting at s[0], or -1.
func diffQuoteEnd(s string) int {
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return -1
}

// diffUnquote undoes git's C-style quoting of a path. git quotes a path that
// contains a double quote, a backslash or a control character; anything else
// is returned as it is.
func diffUnquote(s string) (string, error) {
	if !strings.HasPrefix(s, `"`) {
		return s, nil
	}
	if len(s) < 2 || diffQuoteEnd(s) != len(s)-1 {
		return "", errors.New("unterminated quoted path")
	}
	body := s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i++
		switch e := body[i]; e {
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte('\v')
		case '\\', '"':
			b.WriteByte(e)
		default:
			// Any other byte is written as three octal digits.
			if i+2 >= len(body) {
				return "", errors.New("malformed escape in a quoted path")
			}
			n, err := strconv.ParseUint(body[i:i+3], 8, 8)
			if err != nil {
				return "", errors.New("malformed escape in a quoted path")
			}
			b.WriteByte(byte(n))
			i += 2
		}
	}
	return b.String(), nil
}

// hunk parses one hunk. The header says how many lines of each side follow,
// and that count — not the look of a line — decides where the hunk ends:
// file content is free to look like diff syntax.
func (p *diffParser) hunk() (Hunk, error) {
	h, err := diffParseHunkHeader(p.peek())
	if err != nil {
		return Hunk{}, p.errorf("%s", err)
	}
	p.pos++

	oldLeft, newLeft := h.OldLines, h.NewLines
	oldNo, newNo := h.OldStart, h.NewStart
	for oldLeft > 0 || newLeft > 0 {
		if !p.more() {
			return Hunk{}, p.errorf("hunk ends before its announced length")
		}
		line := p.peek()
		kind := LineContext
		text := line
		if line != "" {
			// A completely empty line is a blank context line printed without
			// its marker (diff.suppressBlankEmpty).
			kind, text = LineKind(line[0]), line[1:]
		}
		dl := DiffLine{Kind: kind, Text: text}
		switch kind {
		case LineContext:
			dl.OldLine, dl.NewLine = oldNo, newNo
			oldNo, newNo, oldLeft, newLeft = oldNo+1, newNo+1, oldLeft-1, newLeft-1
		case LineDeleted:
			dl.OldLine = oldNo
			oldNo, oldLeft = oldNo+1, oldLeft-1
		case LineAdded:
			dl.NewLine = newNo
			newNo, newLeft = newNo+1, newLeft-1
		case LineNoNewline:
			dl.Text = strings.TrimPrefix(text, " ")
		default:
			return Hunk{}, p.errorf("unknown line marker in a hunk")
		}
		if oldLeft < 0 || newLeft < 0 {
			return Hunk{}, p.errorf("hunk has more lines than announced")
		}
		h.Lines = append(h.Lines, dl)
		p.pos++
	}
	// The note about a missing final newline can follow the very last line.
	if p.more() && strings.HasPrefix(p.peek(), `\ `) {
		h.Lines = append(h.Lines, DiffLine{Kind: LineNoNewline, Text: p.peek()[2:]})
		p.pos++
	}
	return h, nil
}

// diffParseHunkHeader parses "@@ -a[,b] +c[,d] @@[ section]". A missing count
// means one line.
func diffParseHunkHeader(line string) (Hunk, error) {
	malformed := errors.New("malformed hunk header")
	rest, ok := strings.CutPrefix(line, "@@ -")
	if !ok {
		return Hunk{}, malformed
	}
	ranges, section, ok := strings.Cut(rest, " @@")
	if !ok {
		return Hunk{}, malformed
	}
	oldRange, newRange, ok := strings.Cut(ranges, " +")
	if !ok {
		return Hunk{}, malformed
	}
	var h Hunk
	var err error
	if h.OldStart, h.OldLines, err = diffParseRange(oldRange); err != nil {
		return Hunk{}, malformed
	}
	if h.NewStart, h.NewLines, err = diffParseRange(newRange); err != nil {
		return Hunk{}, malformed
	}
	h.Section = strings.TrimPrefix(section, " ")
	return h, nil
}

func diffParseRange(s string) (start, count int, err error) {
	startText, countText, hasCount := strings.Cut(s, ",")
	start, err = diffParseCount(startText)
	if err != nil {
		return 0, 0, err
	}
	count = 1
	if hasCount {
		if count, err = diffParseCount(countText); err != nil {
			return 0, 0, err
		}
	}
	return start, count, nil
}

// diffParseCount parses a non-negative line number or count. The limit keeps
// a nonsense header from making the parser count for ever.
func diffParseCount(s string) (int, error) {
	n, err := strconv.ParseUint(s, 10, 31)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// diffParseStat parses `git diff --numstat -z`: "added<TAB>deleted<TAB>path"
// records ending in NUL. Binary files have "-" for both counts. A rename has
// an empty path, followed by the old and the new path as records of their
// own.
func diffParseStat(out []byte) ([]FileStat, error) {
	malformed := errors.New("git diff --numstat: malformed record")
	records := splitNUL(out)
	var stats []FileStat
	for i := 0; i < len(records); i++ {
		parts := strings.SplitN(records[i], "\t", 3)
		if len(parts) != 3 {
			return nil, malformed
		}
		st := FileStat{Path: parts[2]}
		if parts[0] == "-" && parts[1] == "-" {
			st.Binary = true
		} else {
			var err error
			if st.Added, err = diffParseCount(parts[0]); err != nil {
				return nil, malformed
			}
			if st.Deleted, err = diffParseCount(parts[1]); err != nil {
				return nil, malformed
			}
		}
		if st.Path == "" {
			if i+2 >= len(records) {
				return nil, malformed
			}
			st.OldPath, st.Path = records[i+1], records[i+2]
			i += 2
		}
		stats = append(stats, st)
	}
	return stats, nil
}
