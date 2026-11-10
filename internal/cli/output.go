package cli

import (
	"encoding/json"
	"io"
)

// style colors text with ANSI escape codes, or leaves it alone when disabled.
type style struct{ enabled bool }

func (s style) wrap(code, text string) string {
	if !s.enabled {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (s style) bold(text string) string   { return s.wrap("1", text) }
func (s style) dim(text string) string    { return s.wrap("2", text) }
func (s style) red(text string) string    { return s.wrap("31", text) }
func (s style) green(text string) string  { return s.wrap("32", text) }
func (s style) yellow(text string) string { return s.wrap("33", text) }

// styleFor decides whether output to w may be colored: only on an interactive
// terminal that understands escape codes, and never when the user or a script
// asked for plain output (--no-color, --json, NO_COLOR, TERM=dumb).
func (a *app) styleFor(w io.Writer) style {
	getenv := a.env.Getenv
	switch {
	case a.flags.noColor, a.flags.json:
		return style{}
	case getenv("NO_COLOR") != "", getenv("TERM") == "dumb":
		return style{}
	case !a.env.IsTerminal(w):
		return style{}
	case a.env.GOOS == "windows" && getenv("WT_SESSION") == "" && getenv("TERM") == "":
		// The classic Windows console prints escape codes as garbage.
		return style{}
	}
	return style{enabled: true}
}

// writeJSON prints v to stdout as one indented JSON document.
func (a *app) writeJSON(v any) error {
	enc := json.NewEncoder(a.env.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
