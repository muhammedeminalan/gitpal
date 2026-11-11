// Package graph draws a commit history as text lanes, for the CLI and the GUI.
package graph

import (
	"slices"
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
)

// Rows draws the history as lanes, one row per commit, newest first:
// "*" marks the commit, "|" a line of history passing by. It is the plain
// cousin of `git log --graph`: lanes appear and disappear without diagonal
// connectors, which keeps every row exactly as wide as the lanes it has.
func Rows(commits []git.Commit) []string {
	var lanes []string // the commit each lane is waiting for
	rows := make([]string, 0, len(commits))
	for _, c := range commits {
		col := slices.Index(lanes, c.Hash)
		if col < 0 {
			lanes = append(lanes, c.Hash)
			col = len(lanes) - 1
		}
		var b strings.Builder
		for i := range lanes {
			if i > 0 {
				b.WriteByte(' ')
			}
			switch {
			case i == col:
				b.WriteByte('*')
			case lanes[i] == c.Hash:
				// Another lane reached this commit too: it ends here.
				b.WriteByte('/')
			default:
				b.WriteByte('|')
			}
		}
		rows = append(rows, b.String())

		// Lanes that waited for this commit now wait for its parents.
		next := make([]string, 0, len(lanes)+len(c.Parents))
		for i, h := range lanes {
			switch {
			case i == col && len(c.Parents) > 0:
				next = append(next, c.Parents[0])
			case h == c.Hash:
			default:
				next = append(next, h)
			}
		}
		for _, p := range c.Parents[min(1, len(c.Parents)):] {
			if !slices.Contains(next, p) {
				next = append(next, p)
			}
		}
		lanes = next
	}
	return rows
}
