package ai

import (
	"fmt"
	"path"
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
)

// DefaultBudget is roughly 12,000 tokens of diff, at about four characters a
// token (spec §6.3).
const DefaultBudget = 48_000

// minifiedLine is the line length from which a file is treated as generated.
const minifiedLine = 1000

// lockFiles are generated dependency lists: their content says nothing a
// commit message needs (spec §6.3).
var lockFiles = map[string]bool{
	"package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true, "go.sum": true, "pubspec.lock": true,
	"Cargo.lock": true, "Gemfile.lock": true, "poetry.lock": true, "composer.lock": true, "Podfile.lock": true,
	"Package.resolved": true, "packages.lock.json": true,
}

// Payload is the text that is sent, and an account of what was held back.
type Payload struct {
	// Text is the summary of files and the diff, ready to send.
	Text string
	// Files lists every changed file with its line counts.
	Files []string
	// Withheld are files whose content was not sent because they hold
	// credentials; only their names are mentioned.
	Withheld []string
	// Summarized are files reduced to a "changed" note: lock files,
	// minified, binary.
	Summarized []string
	// Truncated is set when the diff was cut to fit the budget.
	Truncated bool
}

// Chars is the size of what is sent.
func (p Payload) Chars() int { return len(p.Text) }

// ApproxTokens estimates the tokens sent, for showing the user (spec §6.3).
func (p Payload) ApproxTokens() int { return (len(p.Text) + 3) / 4 }

// Prepare turns a diff into the text a provider may see. budget caps the
// diff's size in characters; zero means DefaultBudget.
//
// Over budget, every file keeps its header and an equal share of the budget
// for its lines, so a big change in one file cannot crowd the others out.
func Prepare(files []git.FileDiff, budget int) Payload {
	if budget <= 0 {
		budget = DefaultBudget
	}
	var p Payload
	type section struct {
		header string
		lines  []string
	}
	var sections []section
	for _, f := range files {
		added, deleted := 0, 0
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				switch l.Kind {
				case git.LineAdded:
					added++
				case git.LineDeleted:
					deleted++
				case git.LineContext, git.LineNoNewline:
				}
			}
		}
		name := f.Path
		if f.OldPath != "" {
			name = f.OldPath + " -> " + f.Path
		}
		p.Files = append(p.Files, fmt.Sprintf("%s %s (+%d -%d)", f.Change, name, added, deleted))

		header := fmt.Sprintf("### %s (%s)", name, f.Change)
		switch {
		case safety.IsSecretFile(f.Path) || (f.OldPath != "" && safety.IsSecretFile(f.OldPath)) || holdsPrivateKey(f):
			p.Withheld = append(p.Withheld, f.Path)
			sections = append(sections, section{header: header + " — content withheld: may contain secrets"})
		case f.Binary || lockFiles[path.Base(f.Path)] || isMinified(f):
			p.Summarized = append(p.Summarized, f.Path)
			sections = append(sections, section{header: header + " — generated or binary file, content not included"})
		default:
			var lines []string
			for _, h := range f.Hunks {
				lines = append(lines, fmt.Sprintf("@@ -%d +%d @@ %s", h.OldStart, h.NewStart, h.Section))
				for _, l := range h.Lines {
					if l.Kind == git.LineNoNewline {
						continue
					}
					lines = append(lines, string(l.Kind)+Redact(strings.TrimSuffix(l.Text, "\r")))
				}
			}
			sections = append(sections, section{header: header, lines: lines})
		}
	}

	total := 0
	for _, s := range sections {
		total += len(s.header) + 1
		for _, l := range s.lines {
			total += len(l) + 1
		}
	}
	share := budget
	if total > budget && len(sections) > 0 {
		p.Truncated = true
		share = budget / len(sections)
	}

	var b strings.Builder
	b.WriteString("Changed files:\n")
	for _, f := range p.Files {
		b.WriteString("- " + f + "\n")
	}
	b.WriteString("\nDiff:\n")
	for _, s := range sections {
		b.WriteString(s.header + "\n")
		used := 0
		for i, l := range s.lines {
			if p.Truncated && used+len(l)+1 > share {
				fmt.Fprintf(&b, "… (%d more lines not shown)\n", len(s.lines)-i)
				break
			}
			b.WriteString(l + "\n")
			used += len(l) + 1
		}
	}
	p.Text = b.String()
	return p
}

func holdsPrivateKey(f git.FileDiff) bool {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if strings.Contains(l.Text, "PRIVATE KEY") && strings.Contains(l.Text, "-----BEGIN") {
				return true
			}
		}
	}
	return false
}

func isMinified(f git.FileDiff) bool {
	if strings.Contains(path.Base(f.Path), ".min.") {
		return true
	}
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if len(l.Text) > minifiedLine {
				return true
			}
		}
	}
	return false
}
