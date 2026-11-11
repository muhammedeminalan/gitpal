// Package tui holds gitpal's interactive terminal screens (spec §3, §8):
// choosing files and the dashboard that `gitpal` opens without a subcommand.
//
// Like the command line, it never runs git itself: it calls internal/core,
// and every risky operation goes through the safety layer.
package tui

import (
	"context"
	"errors"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrCanceled is returned when the user leaves a screen without confirming.
var ErrCanceled = errors.New("tui: canceled")

// Item is one choice in a picker.
type Item struct {
	// Label is what is shown; Value what is returned when chosen.
	Label, Value string
	Selected     bool
}

// PickOptions describes a picker screen.
type PickOptions struct {
	Title, Help string
	Items       []Item
	In          io.Reader
	Out         io.Writer
}

// Pick shows a list of items to choose from and returns the values of the
// chosen ones, in list order.
func Pick(ctx context.Context, opts PickOptions) ([]string, error) {
	m := newPicker(opts.Title, opts.Help, opts.Items)
	final, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(opts.In), tea.WithOutput(opts.Out)).Run()
	if err != nil {
		if errors.Is(err, tea.ErrProgramKilled) || errors.Is(err, context.Canceled) {
			return nil, ErrCanceled
		}
		return nil, err
	}
	p := final.(picker)
	if p.canceled {
		return nil, ErrCanceled
	}
	return p.chosen(), nil
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	helpStyle     = lipgloss.NewStyle().Faint(true)
	cursorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// picker is the model of the Pick screen.
type picker struct {
	title, help string
	items       []Item
	cursor      int
	done        bool
	canceled    bool
}

func newPicker(title, help string, items []Item) picker {
	return picker{title: title, help: help, items: append([]Item(nil), items...)}
}

func (p picker) Init() tea.Cmd { return nil }

func (p picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch key.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
	case " ", "x":
		if len(p.items) > 0 {
			p.items[p.cursor].Selected = !p.items[p.cursor].Selected
		}
	case "a":
		p.setAll(true)
	case "n":
		p.setAll(false)
	case "enter":
		p.done = true
		return p, tea.Quit
	case "q", "esc", "ctrl+c":
		p.canceled = true
		return p, tea.Quit
	}
	return p, nil
}

func (p *picker) setAll(selected bool) {
	for i := range p.items {
		p.items[i].Selected = selected
	}
}

func (p picker) chosen() []string {
	var out []string
	for _, item := range p.items {
		if item.Selected {
			out = append(out, item.Value)
		}
	}
	return out
}

func (p picker) View() string {
	if p.done || p.canceled {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(p.title) + "\n\n")
	for i, item := range p.items {
		box := "[ ]"
		if item.Selected {
			box = selectedStyle.Render("[x]")
		}
		line := box + " " + item.Label
		if i == p.cursor {
			line = cursorStyle.Render("›") + " " + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + helpStyle.Render(p.help) + "\n")
	return b.String()
}
