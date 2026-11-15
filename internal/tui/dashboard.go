package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
)

// DashboardOptions is what the dashboard works with.
type DashboardOptions struct {
	Lang  i18n.Lang
	Git   *git.Runner
	Dir   string
	Guard *safety.Guard
	// Pull is the strategy `l` pulls with.
	Pull git.PullStrategy
	In   io.Reader
	Out  io.Writer
}

// RunDashboard shows the state of the repository and lets the user stage,
// commit, push, pull and fetch from one screen.
func RunDashboard(ctx context.Context, opts DashboardOptions) error {
	m := newDashboard(ctx, opts)
	_, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(opts.In), tea.WithOutput(opts.Out), tea.WithAltScreen()).Run()
	if errors.Is(err, tea.ErrProgramKilled) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

type mode int

const (
	modeList mode = iota
	modeCommit
	modeConfirm
	modeBusy
	// modeBranches: b ile açılan branch listesi.
	modeBranches
	// modeNewBranch: listede n ile yeni branch adı sorulur.
	modeNewBranch
)

// row is one line of the file list.
type row struct {
	entry  git.StatusEntry
	staged bool
}

type dashboard struct {
	ctx  context.Context
	opts DashboardOptions
	loc  i18n.Localizer

	status git.Status
	rows   []row
	cursor int
	mode   mode

	message textinput.Model
	typed   textinput.Model
	// branches ve bcursor, branch listesi (yalnızca yereldekiler) ve seçili satır.
	branches   []git.Branch
	bcursor    int
	branchName textinput.Model
	pending    *safety.Assessment
	info       string
	err        string
	quitting   bool
}

// Messages the dashboard's commands send back.
type (
	statusMsg struct {
		status git.Status
		err    error
	}
	assessedMsg struct {
		assessment safety.Assessment
		err        error
	}
	doneMsg struct {
		info string
		err  error
	}
	branchesMsg struct {
		branches []git.Branch
		err      error
	}
)

func newDashboard(ctx context.Context, opts DashboardOptions) dashboard {
	msg := textinput.New()
	msg.CharLimit = 500
	typed := textinput.New()
	name := textinput.New()
	name.CharLimit = 200
	return dashboard{ctx: ctx, opts: opts, loc: i18n.For(opts.Lang), message: msg, typed: typed, branchName: name}
}

func (d dashboard) Init() tea.Cmd { return d.refresh() }

func (d dashboard) refresh() tea.Cmd {
	return func() tea.Msg {
		st, err := d.opts.Git.Status(d.ctx, d.opts.Dir, git.StatusOptions{})
		return statusMsg{status: st, err: err}
	}
}

// rowsOf lists staged entries first, then the rest.
func rowsOf(st git.Status) []row {
	var staged, other []row
	for _, e := range st.Entries {
		switch {
		case e.Kind == git.EntryIgnored:
		case e.Staged() && !e.Unstaged():
			staged = append(staged, row{entry: e, staged: true})
		case e.Staged():
			staged = append(staged, row{entry: e, staged: true})
			other = append(other, row{entry: e})
		default:
			other = append(other, row{entry: e})
		}
	}
	return append(staged, other...)
}

func (d dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		if msg.err != nil {
			d.err = i18n.Describe(d.opts.Lang, msg.err)
			return d, nil
		}
		d.status, d.rows = msg.status, rowsOf(msg.status)
		if d.cursor >= len(d.rows) {
			d.cursor = max(0, len(d.rows)-1)
		}
		return d, nil
	case assessedMsg:
		return d.onAssessed(msg)
	case branchesMsg:
		if msg.err != nil {
			return d.Update(doneMsg{err: msg.err})
		}
		d.branches, d.bcursor, d.mode = msg.branches, 0, modeBranches
		for i, b := range msg.branches {
			if b.Current {
				d.bcursor = i
			}
		}
		return d, nil
	case doneMsg:
		d.mode = modeList
		d.pending = nil
		if msg.err != nil {
			d.err, d.info = i18n.Describe(d.opts.Lang, msg.err), ""
		} else {
			d.err, d.info = "", msg.info
		}
		return d, d.refresh()
	case tea.KeyMsg:
		switch d.mode {
		case modeCommit:
			return d.updateCommit(msg)
		case modeConfirm:
			return d.updateConfirm(msg)
		case modeBusy:
			return d, nil
		case modeList:
			return d.updateList(msg)
		case modeBranches:
			return d.updateBranches(msg)
		case modeNewBranch:
			return d.updateNewBranch(msg)
		}
	}
	return d, nil
}

func (d dashboard) updateList(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "esc", "ctrl+c":
		d.quitting = true
		return d, tea.Quit
	case "up", "k":
		if d.cursor > 0 {
			d.cursor--
		}
	case "down", "j":
		if d.cursor < len(d.rows)-1 {
			d.cursor++
		}
	case "r":
		return d, d.refresh()
	case " ":
		if len(d.rows) == 0 {
			return d, nil
		}
		r := d.rows[d.cursor]
		return d.busy(func() doneMsg {
			var err error
			if r.staged {
				err = d.opts.Git.Unstage(d.ctx, d.opts.Dir, []string{r.entry.Path})
			} else {
				paths := []string{r.entry.Path}
				if r.entry.OrigPath != "" {
					paths = append(paths, r.entry.OrigPath)
				}
				err = d.opts.Git.Stage(d.ctx, d.opts.Dir, paths)
			}
			return doneMsg{err: err}
		})
	case "a":
		return d.busy(func() doneMsg { return doneMsg{err: d.opts.Git.StageAll(d.ctx, d.opts.Dir)} })
	case "c":
		d.mode = modeCommit
		d.message.Reset()
		d.message.Focus()
		return d, textinput.Blink
	case "p":
		return d.assess(safety.Push(git.PushOptions{SetUpstream: d.status.Branch.Upstream == ""}))
	case "l":
		return d.assess(safety.Pull(git.PullOptions{Strategy: d.opts.Pull}))
	case "f":
		return d.busy(func() doneMsg {
			return doneMsg{err: d.opts.Git.Fetch(d.ctx, d.opts.Dir, git.FetchOptions{}), info: d.loc.T(i18n.MsgFetchDone)}
		})
	case "b":
		d.mode, d.err, d.info = modeBusy, "", ""
		return d, func() tea.Msg {
			all, err := d.opts.Git.Branches(d.ctx, d.opts.Dir, git.BranchListOptions{})
			return branchesMsg{branches: all, err: err}
		}
	}
	return d, nil
}

// updateBranches: listede gezinme, enter ile geçme, n ile yeni branch.
func (d dashboard) updateBranches(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q", "ctrl+c":
		d.mode = modeList
	case "up", "k":
		if d.bcursor > 0 {
			d.bcursor--
		}
	case "down", "j":
		if d.bcursor < len(d.branches)-1 {
			d.bcursor++
		}
	case "n":
		d.mode = modeNewBranch
		d.branchName.Reset()
		d.branchName.Focus()
		return d, textinput.Blink
	case "enter":
		if len(d.branches) == 0 {
			return d, nil
		}
		b := d.branches[d.bcursor]
		if b.Current {
			d.mode = modeList
			return d, nil
		}
		// Geçiş de güvenlik katmanından: commit'lenmemiş değişiklik varsa yedeklenir.
		return d.assess(safety.Switch(b.Name))
	}
	return d, nil
}

// updateNewBranch: ad yazılır; enter yeni branch'i açıp ona geçer. Yıkıcı bir
// işlem değil (hiçbir şey kaybolmaz), o yüzden onay sorulmaz.
func (d dashboard) updateNewBranch(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+c":
		d.mode = modeBranches
		return d, nil
	case "enter":
		name := strings.TrimSpace(d.branchName.Value())
		if name == "" {
			return d, nil
		}
		return d.busy(func() doneMsg {
			if err := d.opts.Git.ValidateBranchName(d.ctx, d.opts.Dir, name); err != nil {
				text := i18n.Describe(d.opts.Lang, err)
				if s := git.SuggestBranchName(name); s != "" && s != name {
					text += "\n" + d.loc.T(i18n.MsgBranchSuggest, s)
				}
				return doneMsg{err: errors.New(text)}
			}
			if err := d.opts.Git.CreateAndSwitch(d.ctx, d.opts.Dir, name, ""); err != nil {
				return doneMsg{err: err}
			}
			return doneMsg{info: d.loc.T(i18n.MsgBranchCreated, name)}
		})
	}
	var cmd tea.Cmd
	d.branchName, cmd = d.branchName.Update(key)
	return d, cmd
}

func (d dashboard) updateCommit(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+c":
		d.mode = modeList
		return d, nil
	case "enter":
		text := strings.TrimSpace(d.message.Value())
		if text == "" {
			return d, nil
		}
		return d.assess(safety.Commit(git.CommitOptions{Message: text}))
	}
	var cmd tea.Cmd
	d.message, cmd = d.message.Update(key)
	return d, cmd
}

func (d dashboard) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := d.pending
	if a.Confirm == safety.ConfirmTypeBranch {
		switch key.String() {
		case "esc", "ctrl+c":
			d.mode, d.pending, d.info = modeList, nil, d.loc.T(i18n.MsgSafetyDeclined)
			return d, nil
		case "enter":
			return d.run(*a, safety.Answer{Approved: true, Typed: strings.TrimSpace(d.typed.Value())})
		}
		var cmd tea.Cmd
		d.typed, cmd = d.typed.Update(key)
		return d, cmd
	}
	switch key.String() {
	case "y", "e":
		return d.run(*a, safety.Answer{Approved: true})
	case "n", "h", "esc", "q", "ctrl+c":
		d.mode, d.pending, d.info = modeList, nil, d.loc.T(i18n.MsgSafetyDeclined)
	}
	return d, nil
}

// busy runs work in the background and shows the result when it is done.
func (d dashboard) busy(work func() doneMsg) (tea.Model, tea.Cmd) {
	d.mode, d.err, d.info = modeBusy, "", ""
	return d, func() tea.Msg { return work() }
}

// assess judges an operation first, so the user sees what it will do before
// anything happens.
func (d dashboard) assess(op safety.Operation) (tea.Model, tea.Cmd) {
	d.mode, d.err, d.info = modeBusy, "", ""
	return d, func() tea.Msg {
		a, err := d.opts.Guard.Assess(d.ctx, op)
		return assessedMsg{assessment: a, err: err}
	}
}

func (d dashboard) onAssessed(msg assessedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return d.Update(doneMsg{err: msg.err})
	}
	a := msg.assessment
	if a.Blocked() {
		return d.Update(doneMsg{err: &safety.BlockedError{Assessment: a}})
	}
	if a.Confirm == safety.ConfirmNone {
		return d.run(a, safety.Answer{})
	}
	d.mode, d.pending = modeConfirm, &a
	d.typed.Reset()
	d.typed.Focus()
	return d, nil
}

// run performs an assessed operation with the user's answer.
func (d dashboard) run(a safety.Assessment, answer safety.Answer) (tea.Model, tea.Cmd) {
	return d.busy(func() doneMsg {
		res, err := d.opts.Guard.Do(d.ctx, a.Op, func(safety.Assessment) (safety.Answer, error) { return answer, nil }, safety.DoOptions{})
		if err != nil {
			return doneMsg{err: err}
		}
		return doneMsg{info: d.describe(a.Op, res)}
	})
}

func (d dashboard) describe(op safety.Operation, res safety.Result) string {
	switch op.Kind {
	case safety.OpCommit, safety.OpAmend:
		return d.loc.T(i18n.MsgCommitDone, shortHash(res.Commit.Hash), firstLine(op.CommitOptions.Message))
	case safety.OpPush, safety.OpForcePush:
		if res.Push.Status == git.PushUpToDate {
			return d.loc.T(i18n.MsgPushUpToDate)
		}
		return d.loc.T(i18n.MsgPushDone, res.Push.Branch, res.Push.Remote, res.Push.RemoteBranch)
	case safety.OpSwitch:
		return d.loc.T(i18n.MsgBranchSwitched, op.Branch)
	case safety.OpPull:
		if !res.Pull.Updated {
			return d.loc.T(i18n.MsgPullUpToDate)
		}
		return d.loc.T(i18n.MsgPullDone, shortHash(res.Pull.Before), shortHash(res.Pull.After))
	}
	return d.loc.T(i18n.MsgDone)
}

func (d dashboard) View() string {
	if d.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(branchLine(d.loc, d.status)) + "\n")
	if d.status.Operation != git.OpNone {
		b.WriteString(warnStyle.Render(d.loc.T(i18n.MsgStatusOperation, d.status.Operation)) + "\n")
	}
	b.WriteString("\n")
	if len(d.rows) == 0 {
		b.WriteString(d.loc.T(i18n.MsgTUINoChanges) + "\n")
	}
	header := ""
	for i, r := range d.rows {
		section := d.loc.T(i18n.MsgTUIChanges)
		if r.staged {
			section = d.loc.T(i18n.MsgTUIStaged)
		}
		if section != header {
			b.WriteString(titleStyle.Render(section) + "\n")
			header = section
		}
		line := entryLine(r)
		if i == d.cursor {
			b.WriteString(cursorStyle.Render("› ") + line + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}
	b.WriteString("\n")

	switch d.mode {
	case modeCommit:
		b.WriteString(d.loc.T(i18n.MsgTUICommitPrompt) + "\n" + d.message.View() + "\n")
	case modeConfirm:
		b.WriteString(confirmView(d.loc, *d.pending, d.typed))
	case modeBusy:
		b.WriteString(helpStyle.Render(d.loc.T(i18n.MsgTUIWorking)) + "\n")
	case modeBranches, modeNewBranch:
		b.WriteString(d.branchesView())
	case modeList:
		if d.err != "" {
			b.WriteString(errorStyle.Render(d.err) + "\n")
		}
		if d.info != "" {
			b.WriteString(selectedStyle.Render(d.info) + "\n")
		}
		b.WriteString(helpStyle.Render(d.loc.T(i18n.MsgTUIHelpDashboard)) + "\n")
	}
	return b.String()
}

func confirmView(loc i18n.Localizer, a safety.Assessment, typed textinput.Model) string {
	s := a.Summary(loc.Lang())
	var b strings.Builder
	b.WriteString(warnStyle.Render(loc.T(i18n.MsgRiskLabel)+": "+s.Risk) + "\n")
	for _, line := range s.Effects {
		b.WriteString("  - " + line + "\n")
	}
	for _, line := range s.Findings {
		b.WriteString(warnStyle.Render("  ! "+line) + "\n")
	}
	for _, line := range s.Backups {
		b.WriteString(helpStyle.Render("  "+line) + "\n")
	}
	if a.Confirm == safety.ConfirmTypeBranch {
		b.WriteString(loc.T(i18n.MsgTUITypeBranch, a.ConfirmText) + "\n" + typed.View() + "\n")
	} else {
		b.WriteString(helpStyle.Render(loc.T(i18n.MsgTUIConfirmHelp)) + "\n")
	}
	return b.String()
}

func branchLine(loc i18n.Localizer, st git.Status) string {
	b := st.Branch
	switch {
	case b.Unborn:
		return loc.T(i18n.MsgStatusUnborn, b.Head)
	case b.Detached:
		return loc.T(i18n.MsgStatusDetached, shortHash(b.OID))
	case b.Upstream != "" && !b.UpstreamGone:
		return loc.T(i18n.MsgStatusOnBranch, b.Head) + " · " + loc.T(i18n.MsgStatusUpstream, b.Upstream, b.Ahead, b.Behind)
	}
	return loc.T(i18n.MsgStatusOnBranch, b.Head)
}

// entryLine renders a file the way `git status --short` would.
func entryLine(r row) string {
	e := r.entry
	mark := "?"
	switch {
	case e.Kind == git.EntryUnmerged:
		mark = "U"
	case r.staged:
		mark = string(e.Index)
	case e.Kind != git.EntryUntracked:
		mark = string(e.Worktree)
	}
	name := e.Path
	if e.OrigPath != "" && r.staged {
		name = e.OrigPath + " → " + e.Path
	}
	return fmt.Sprintf("%s  %s", mark, name)
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func (d dashboard) branchesView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(d.loc.T(i18n.MsgTUIBranches)) + "\n")
	for i, br := range d.branches {
		mark := "  "
		if br.Current {
			mark = "* "
		}
		line := mark + br.Name
		if i == d.bcursor {
			b.WriteString(cursorStyle.Render("› ") + selectedStyle.Render(line) + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}
	b.WriteString("\n")
	if d.mode == modeNewBranch {
		b.WriteString(d.loc.T(i18n.MsgTUINewBranch) + "\n" + d.branchName.View() + "\n")
		return b.String()
	}
	b.WriteString(helpStyle.Render(d.loc.T(i18n.MsgTUIHelpBranches)) + "\n")
	return b.String()
}
