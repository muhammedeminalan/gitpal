// Package gitignore holds ready-made .gitignore templates (spec §4.12).
package gitignore

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed templates/*.gitignore
var templates embed.FS

// ErrUnknownTemplate is returned for a template name that does not exist.
var ErrUnknownTemplate = errors.New("unknown .gitignore template")

// Names lists the templates, sorted.
func Names() []string {
	entries, _ := fs.ReadDir(templates, "templates")
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".gitignore"))
	}
	slices.Sort(names)
	return names
}

// Template returns a template's content.
func Template(name string) (string, error) {
	data, err := templates.ReadFile("templates/" + name + ".gitignore")
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnknownTemplate, name)
	}
	return string(data), nil
}

// Apply adds the templates' lines to the .gitignore at the root of dir,
// skipping lines the file already has. Existing content is kept as it is.
func Apply(dir string, names []string) error {
	var add []string
	for _, name := range names {
		t, err := Template(strings.TrimSpace(name))
		if err != nil {
			return err
		}
		add = append(add, strings.Split(strings.TrimRight(t, "\n"), "\n")...)
	}
	file := filepath.Join(dir, ".gitignore")
	existing, err := os.ReadFile(file) //nolint:gosec // G304: the repository's own .gitignore
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	eol := "\n"
	if strings.Contains(string(existing), "\r\n") {
		eol = "\r\n"
	}
	present := map[string]bool{}
	for line := range strings.SplitSeq(string(existing), "\n") {
		present[strings.TrimRight(line, "\r")] = true
	}
	var b strings.Builder
	b.Write(existing)
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		b.WriteString(eol)
	}
	wrote := false
	for _, line := range add {
		// Comments and blank lines are kept for readability, rules once.
		if line != "" && !strings.HasPrefix(line, "#") && present[line] {
			continue
		}
		present[line] = true
		if !wrote && len(existing) > 0 {
			b.WriteString(eol)
		}
		wrote = true
		b.WriteString(line + eol)
	}
	if !wrote {
		return nil
	}
	return os.WriteFile(file, []byte(b.String()), 0o644) //nolint:gosec // G306: an ordinary tracked file
}
