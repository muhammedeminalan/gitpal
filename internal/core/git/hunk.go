package git

import (
	"context"
	"fmt"
	"strings"
)

// BuildPatch writes a patch holding only the chosen hunks (by index) of a
// file's diff, ready for `git apply`.
//
// Leaving hunks out shifts the line numbers of the ones after them on the
// side the patch is applied to; they are recomputed. reverse builds the
// patch for `apply --reverse` (unstaging), where the new side is the base.
func BuildPatch(f FileDiff, hunks []int, reverse bool) ([]byte, error) {
	if f.Binary || f.Change != ChangeModified || f.OldPath != "" {
		return nil, &ArgError{Code: ArgUnsupportedFile, Value: f.Path}
	}
	chosen := map[int]bool{}
	for _, i := range hunks {
		if i < 0 || i >= len(f.Hunks) {
			return nil, &ArgError{Code: ArgUnsupportedFile, Value: f.Path}
		}
		chosen[i] = true
	}
	if len(chosen) == 0 {
		return nil, &ArgError{Code: ArgNoPaths}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git %s %s\n--- %s\n+++ %s\n",
		quotePatchPath("a/"+f.Path), quotePatchPath("b/"+f.Path), quotePatchPath("a/"+f.Path), quotePatchPath("b/"+f.Path))
	shift := 0 // lines the chosen hunks so far add (new - old)
	for i, h := range f.Hunks {
		if !chosen[i] {
			continue
		}
		oldStart, newStart := h.OldStart, h.NewStart
		if reverse {
			oldStart = h.NewStart - shift
		} else {
			newStart = h.OldStart + shift
		}
		// An empty side starts one line earlier: "@@ -0,0 +1,2 @@".
		if h.OldLines == 0 && !reverse {
			newStart++
		}
		if h.NewLines == 0 && reverse {
			oldStart++
		}
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldStart, h.OldLines, newStart, h.NewLines)
		for _, l := range h.Lines {
			if l.Kind == LineNoNewline {
				b.WriteString("\\ " + l.Text + "\n")
				continue
			}
			b.WriteString(string(l.Kind) + l.Text + "\n")
		}
		shift += h.NewLines - h.OldLines
	}
	return []byte(b.String()), nil
}

// quotePatchPath quotes a path the way git does when it contains anything a
// patch line could misread.
func quotePatchPath(p string) string {
	if !strings.ContainsFunc(p, func(c rune) bool { return c < ' ' || c == '"' || c == '\\' || c == 0x7f }) {
		return p
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(p); i++ {
		switch c := p[i]; c {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		default:
			if c < ' ' || c == 0x7f {
				fmt.Fprintf(&b, `\%03o`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// StageHunks stages some hunks of a file's unstaged changes. f comes from
// Diff with Staged false.
func (r *Runner) StageHunks(ctx context.Context, dir string, f FileDiff, hunks []int) error {
	return r.applyHunks(ctx, dir, f, hunks, false)
}

// UnstageHunks takes some hunks of a file's staged changes out of the index,
// keeping them in the working tree. f comes from Diff with Staged true.
func (r *Runner) UnstageHunks(ctx context.Context, dir string, f FileDiff, hunks []int) error {
	return r.applyHunks(ctx, dir, f, hunks, true)
}

func (r *Runner) applyHunks(ctx context.Context, dir string, f FileDiff, hunks []int, reverse bool) error {
	patch, err := BuildPatch(f, hunks, reverse)
	if err != nil {
		return err
	}
	// apply.whitespace=error would refuse ordinary changes.
	args := []string{"apply", "--cached", "--whitespace=nowarn"}
	if reverse {
		args = append(args, "--reverse")
	}
	_, err = r.Run(ctx, Cmd{Dir: dir, Args: append(args, "-"), Stdin: patch})
	return err
}
