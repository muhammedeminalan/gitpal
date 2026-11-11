package tui

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// RebaseOptions describes the interactive rebase screen.
type RebaseOptions struct {
	Lang  i18n.Lang
	Title string
	Steps []git.RebaseStep
	In    io.Reader
	Out   io.Writer
}

// EditRebase lets the user reorder the commits of a rebase and choose what
// happens to each one. It returns the edited plan.
func EditRebase(ctx context.Context, opts RebaseOptions) ([]git.RebaseStep, error) {
	m := newRebaseEditor(opts.Lang, opts.Title, opts.Steps)
	final, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(opts.In), tea.WithOutput(opts.Out)).Run()
	if err != nil {
		if errors.Is(err, tea.ErrProgramKilled) || errors.Is(err, context.Canceled) {
			return nil, ErrCanceled
		}
		return nil, err
	}
	e := final.(rebaseEditor)
	if e.canceled {
		return nil, ErrCanceled
	}
	return e.steps, nil
}

type rebaseEditor struct {
	loc      i18n.Localizer
	title    string
	steps    []git.RebaseStep
	cursor   int
	editing  bool
	input    textinput.Model
	done     bool
	canceled bool
}

func newRebaseEditor(lang i18n.Lang, title string, steps []git.RebaseStep) rebaseEditor {
	in := textinput.New()
	in.CharLimit = 500
	return rebaseEditor{loc: i18n.For(lang), title: title, steps: append([]git.RebaseStep(nil), steps...), input: in}
}

func (e rebaseEditor) Init() tea.Cmd { return nil }

func (e rebaseEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keys, ok := splitRunes(msg); ok && !e.editing {
		return applyEach(e, keys)
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok || len(e.steps) == 0 {
		if ok && (key.String() == "q" || key.String() == "esc") {
			e.canceled = true
			return e, tea.Quit
		}
		return e, nil
	}
	if e.editing {
		switch key.String() {
		case "enter":
			e.steps[e.cursor].Message = strings.TrimSpace(e.input.Value())
			e.editing = false
		case "esc":
			e.editing = false
		default:
			var cmd tea.Cmd
			e.input, cmd = e.input.Update(key)
			return e, cmd
		}
		return e, nil
	}
	set := func(action git.RebaseAction) {
		// The first commit has nothing to fold into.
		if e.cursor == 0 && (action == git.RebaseSquash || action == git.RebaseFixup) {
			return
		}
		e.steps[e.cursor].Action = action
	}
	switch key.String() {
	case "up", "k":
		if e.cursor > 0 {
			e.cursor--
		}
	case "down", "j":
		if e.cursor < len(e.steps)-1 {
			e.cursor++
		}
	case "K", "shift+up":
		if e.cursor > 0 {
			e.steps[e.cursor], e.steps[e.cursor-1] = e.steps[e.cursor-1], e.steps[e.cursor]
			e.cursor--
		}
	case "J", "shift+down":
		if e.cursor < len(e.steps)-1 {
			e.steps[e.cursor], e.steps[e.cursor+1] = e.steps[e.cursor+1], e.steps[e.cursor]
			e.cursor++
		}
	case "p":
		set(git.RebasePick)
	case "s":
		set(git.RebaseSquash)
		if e.steps[e.cursor].Action == git.RebaseSquash {
			return e.edit()
		}
	case "f":
		set(git.RebaseFixup)
	case "d":
		set(git.RebaseDrop)
	case "r":
		set(git.RebaseReword)
		return e.edit()
	case "enter":
		// A fold needs something to fold into: the first kept commit cannot
		// be a squash or fixup.
		for _, s := range e.steps {
			if s.Action == git.RebaseDrop {
				continue
			}
			if s.Action == git.RebaseSquash || s.Action == git.RebaseFixup {
				return e, nil
			}
			break
		}
		e.done = true
		return e, tea.Quit
	case "q", "esc", "ctrl+c":
		e.canceled = true
		return e, tea.Quit
	}
	return e, nil
}

func (e rebaseEditor) edit() (tea.Model, tea.Cmd) {
	e.editing = true
	msg := e.steps[e.cursor].Message
	if msg == "" {
		msg = e.steps[e.cursor].Subject
	}
	e.input.SetValue(msg)
	e.input.Focus()
	return e, textinput.Blink
}

func (e rebaseEditor) View() string {
	if e.done || e.canceled {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(e.title) + "\n\n")
	for i, s := range e.steps {
		action := string(s.Action)
		switch s.Action {
		case git.RebaseDrop:
			action = errorStyle.Render(action)
		case git.RebaseSquash, git.RebaseFixup:
			action = warnStyle.Render(action)
		case git.RebaseReword:
			action = selectedStyle.Render(action)
		case git.RebasePick:
		}
		subject := s.Subject
		if s.Message != "" && (s.Action == git.RebaseReword || s.Action == git.RebaseSquash) {
			subject = s.Message + helpStyle.Render("  ← "+s.Subject)
		}
		line := padRight(action, 7) + " " + shortHash(s.Hash) + " " + subject
		if i == e.cursor {
			b.WriteString(cursorStyle.Render("› ") + line + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}
	b.WriteString("\n")
	if e.editing {
		b.WriteString(e.loc.T(i18n.MsgTUIRebaseMsg) + "\n" + e.input.View() + "\n")
	} else {
		b.WriteString(helpStyle.Render(e.loc.T(i18n.MsgTUIHelpRebase)) + "\n")
	}
	return b.String()
}

// padRight pads text to a width of visible characters.
func padRight(s string, width int) string {
	visible := len([]rune(stripANSI(s)))
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			in = true
		case in && r == 'm':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
