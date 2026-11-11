package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestPickerKeys(t *testing.T) {
	items := []Item{{Label: "a.txt", Value: "a.txt", Selected: true}, {Label: "b.txt", Value: "b.txt"}, {Label: "c.txt", Value: "c.txt", Selected: true}}
	var m tea.Model = newPicker("title", "help", items)

	for _, k := range []string{" ", "down", " ", "down", "down", "up", "k", "j"} {
		m, _ = m.Update(key(k))
	}
	p := m.(picker)
	assert.Equal(t, []string{"b.txt", "c.txt"}, p.chosen())
	assert.Contains(t, p.View(), "title")
	assert.Contains(t, p.View(), "[x] b.txt")

	m, _ = m.Update(key("n"))
	assert.Empty(t, m.(picker).chosen())
	m, _ = m.Update(key("a"))
	assert.Len(t, m.(picker).chosen(), 3)

	m, cmd := m.Update(key("enter"))
	require.NotNil(t, cmd)
	assert.True(t, m.(picker).done)
	assert.Empty(t, m.View())

	canceled, _ := newPicker("t", "h", items).Update(key("q"))
	assert.True(t, canceled.(picker).canceled)

	// Items given by the caller are not changed behind its back.
	assert.True(t, items[0].Selected)
}

// Pick runs a real program on a scripted input.
func TestPickRunsAProgram(t *testing.T) {
	var out strings.Builder
	got, err := Pick(context.Background(), PickOptions{
		Title: "t", Help: "h",
		Items: []Item{{Label: "a", Value: "a"}, {Label: "b", Value: "b"}},
		In:    strings.NewReader("j \r"), Out: &out,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"b"}, got)

	_, err = Pick(context.Background(), PickOptions{Items: []Item{{Label: "a", Value: "a"}}, In: strings.NewReader("q"), Out: &out})
	assert.ErrorIs(t, err, ErrCanceled)
}

type dashFixture struct {
	t    *testing.T
	repo *gittest.Repo
	m    tea.Model
}

func newDash(t *testing.T) *dashFixture {
	t.Helper()
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "one\n")
	repo.Commit("first")
	repo.Git("switch", "--quiet", "-c", "work")
	guard, err := safety.NewGuard(context.Background(), repo.Runner, repo.Dir, safety.DefaultPolicy())
	require.NoError(t, err)
	d := newDashboard(context.Background(), DashboardOptions{
		Lang: i18n.EN, Git: repo.Runner, Dir: repo.Dir, Guard: guard, Pull: git.PullFFOnly,
	})
	f := &dashFixture{t: t, repo: repo, m: d}
	f.settle(d.Init())
	return f
}

// settle runs the commands a step produced, feeding their messages back,
// until nothing is left to do.
func (f *dashFixture) settle(cmd tea.Cmd) {
	f.t.Helper()
	for i := 0; cmd != nil && i < 20; i++ {
		msg := run(cmd)
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				f.settle(c)
			}
			return
		}
		if msg == nil {
			return
		}
		switch msg.(type) {
		case statusMsg, assessedMsg, doneMsg:
		default:
			return // a cursor blink or the like
		}
		f.m, cmd = f.m.Update(msg)
	}
}

// run executes a command, giving up on one that only waits for a timer (the
// cursor blink): the work the dashboard does answers at once.
func run(cmd tea.Cmd) tea.Msg {
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		return msg
	case <-time.After(300 * time.Millisecond):
		return nil
	}
}

func (f *dashFixture) press(keys ...string) {
	f.t.Helper()
	for _, k := range keys {
		var cmd tea.Cmd
		f.m, cmd = f.m.Update(key(k))
		f.settle(cmd)
	}
}

func (f *dashFixture) typeText(s string) {
	for _, r := range s {
		f.m, _ = f.m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func (f *dashFixture) view() string { return f.m.View() }

func TestDashboardStageAndCommit(t *testing.T) {
	f := newDash(t)
	f.repo.WriteFile("a.txt", "two\n")
	f.repo.WriteFile("new.txt", "new\n")
	f.press("r")
	assert.Contains(t, f.view(), "Branch: work")
	assert.Contains(t, f.view(), "M  a.txt")
	assert.Contains(t, f.view(), "?  new.txt")

	f.press(" ")
	assert.Contains(t, f.view(), "Staged")
	assert.Equal(t, "M  a.txt\n?? new.txt", f.repo.Git("status", "--porcelain=v1"))

	f.press("a")
	assert.Equal(t, "M  a.txt\nA  new.txt", f.repo.Git("status", "--porcelain=v1"))

	f.press("c")
	f.typeText("feat: from the dashboard")
	f.press("enter")

	assert.Equal(t, "feat: from the dashboard", f.repo.Git("log", "-1", "--format=%s"))
	assert.Contains(t, f.view(), "Committed:")
	assert.Contains(t, f.view(), "No changes.")

	f.press("q")
	assert.Empty(t, f.view())
}

func TestDashboardUnstage(t *testing.T) {
	f := newDash(t)
	f.repo.WriteFile("a.txt", "two\n")
	f.repo.Git("add", "a.txt")
	f.press("r", " ")
	assert.Equal(t, " M a.txt", f.repo.Git("status", "--porcelain=v1"))
}

func TestDashboardAsksBeforeARiskyCommit(t *testing.T) {
	f := newDash(t)
	f.repo.Git("switch", "--quiet", "main")
	f.repo.WriteFile("a.txt", "two\n")
	f.repo.Git("add", "a.txt")
	before := f.repo.Git("rev-parse", "HEAD")
	f.press("r", "c")
	f.typeText("on main")
	f.press("enter")

	assert.Contains(t, f.view(), "protected branch", "the warning is shown")
	assert.Contains(t, f.view(), "y confirm")

	f.press("n")
	assert.Equal(t, before, f.repo.Git("rev-parse", "HEAD"))
	assert.Contains(t, f.view(), "not confirmed")

	f.press("c")
	f.typeText("on main")
	f.press("enter", "y")
	assert.NotEqual(t, before, f.repo.Git("rev-parse", "HEAD"))
}

func TestDashboardShowsErrors(t *testing.T) {
	f := newDash(t)
	f.press("p") // no remote
	assert.Contains(t, f.view(), "no such remote")

	f.press("c", "esc")
	assert.Contains(t, f.view(), "c commit", "back to the list")
}

func TestDashboardTypedConfirmation(t *testing.T) {
	origin := gittest.NewBareRepo(t)
	repo := gittest.Clone(t, origin)
	repo.WriteFile("a.txt", "one\n")
	repo.Commit("first")
	repo.Git("switch", "--quiet", "-c", "work")
	repo.Git("push", "--quiet", "--set-upstream", "origin", "work")
	repo.Git("commit", "--quiet", "--amend", "-m", "rewritten")
	guard, err := safety.NewGuard(context.Background(), repo.Runner, repo.Dir, safety.DefaultPolicy())
	require.NoError(t, err)
	f := &dashFixture{t: t, repo: repo, m: newDashboard(context.Background(), DashboardOptions{Lang: i18n.EN, Git: repo.Runner, Dir: repo.Dir, Guard: guard})}

	var cmd tea.Cmd
	f.m, cmd = f.m.(dashboard).assess(safety.Push(git.PushOptions{ForceWithLease: true}))
	f.settle(cmd)
	assert.Contains(t, f.view(), "Type the branch name to confirm: work")

	f.typeText("wrong")
	f.press("enter")
	assert.Contains(t, f.view(), "not confirmed")

	f.m, cmd = f.m.(dashboard).assess(safety.Push(git.PushOptions{ForceWithLease: true}))
	f.settle(cmd)
	f.typeText("work")
	f.press("enter")
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), origin.Git("rev-parse", "refs/heads/work"))
}

func TestRunDashboardQuitsOnQ(t *testing.T) {
	repo := gittest.NewRepo(t)
	guard, err := safety.NewGuard(context.Background(), repo.Runner, repo.Dir, safety.DefaultPolicy())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var out strings.Builder

	err = RunDashboard(ctx, DashboardOptions{Lang: i18n.TR, Git: repo.Runner, Dir: repo.Dir, Guard: guard, In: strings.NewReader("q"), Out: &out})

	require.NoError(t, err)
}
