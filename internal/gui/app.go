// Package gui is the bridge between the desktop app's web frontend and
// internal/core (spec §9). Wails binds the exported methods of App to the
// frontend; every one of them is plain Go and is tested without a window.
//
// Like the command line it never runs git itself, and every risky operation
// goes through the safety layer: the frontend first asks Assess, shows the
// safety dialog, then calls Do with the user's answer.
package gui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/muhammedeminalan/gitpal/internal/core/ai"
	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/graph"
	"github.com/muhammedeminalan/gitpal/internal/core/history"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
	"github.com/muhammedeminalan/gitpal/internal/version"
)

// Deps is what App works with; tests replace all of it.
type Deps struct {
	// NewRunner builds the git runner; App adds its own observer.
	NewRunner func(opts ...git.Option) *git.Runner
	ConfigEnv config.Env
	Secrets   secrets.Store
	// NewAI builds the AI provider.
	NewAI func(ai.Settings) (ai.Provider, error)
	// Now is the clock.
	Now func() time.Time
	// PickFolder shows the system's folder dialog; "" when the user cancels.
	// Nil where there is no window (tests).
	PickFolder func(ctx context.Context) (string, error)
}

// App is bound to the frontend.
type App struct {
	deps Deps

	mu       sync.Mutex
	ctx      context.Context
	runner   *git.Runner
	root     string
	guard    *safety.Guard
	cfg      config.Config
	commands []Command
}

// Command is a git command that ran, for the "show the command" panel.
type Command struct {
	Line     string `json:"line"`
	ExitCode int    `json:"exitCode"`
	Millis   int64  `json:"millis"`
}

// maxCommands is how many recent commands are kept.
const maxCommands = 50

// New returns an App.
func New(deps Deps) *App {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	a := &App{deps: deps, ctx: context.Background()}
	a.cfg = a.loadConfig("")
	a.runner = deps.NewRunner(git.WithObserver(a.observe))
	return a
}

// Startup is called by Wails with the app's context.
func (a *App) Startup(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ctx = ctx
}

func (a *App) observe(e git.Event) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.commands = append(a.commands, Command{Line: e.String(), ExitCode: e.ExitCode, Millis: e.Duration.Milliseconds()})
	if len(a.commands) > maxCommands {
		a.commands = a.commands[len(a.commands)-maxCommands:]
	}
}

// Commands returns the git commands that ran recently, newest last.
func (a *App) Commands() []Command {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.commands)
}

func (a *App) loadConfig(repoDir string) config.Config {
	path, err := config.GlobalPath(a.deps.ConfigEnv)
	if err != nil {
		return config.Default()
	}
	loaded, err := config.Load(config.LoadOptions{GlobalPath: path, RepoDir: repoDir})
	if err != nil {
		return config.Default()
	}
	return loaded.Config
}

func (a *App) lang() i18n.Lang { return a.cfg.Language }

// fail turns an error into one whose text is the localized explanation:
// that is what the frontend shows.
func (a *App) fail(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(i18n.Describe(a.lang(), err))
}

func (a *App) current() (context.Context, *git.Runner, string, *safety.Guard, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.guard == nil {
		return nil, nil, "", nil, errors.New(i18n.T(a.lang(), i18n.MsgGUINoRepo))
	}
	return a.ctx, a.runner, a.root, a.guard, nil
}

// Info describes the app for the frontend.
type Info struct {
	Version string            `json:"version"`
	Lang    string            `json:"lang"`
	Theme   string            `json:"theme"`
	Texts   map[string]string `json:"texts"`
	// Root is the repository already open, empty when none is.
	Root string `json:"root"`
}

// Info returns the version, the language and theme settings, and every text
// the frontend shows, in the configured language.
func (a *App) Info() Info {
	a.mu.Lock()
	root, cfg := a.root, a.cfg
	a.mu.Unlock()
	return Info{
		Version: version.String(), Lang: string(cfg.Language), Theme: string(cfg.UI.Theme),
		Texts: i18n.GUITexts(cfg.Language), Root: root,
	}
}

// policy turns the safety settings into the safety layer's policy.
func policy(c config.SafetyConfig) safety.Policy {
	return safety.Policy{
		ProtectedBranches: c.ProtectedBranches,
		LargeFileBytes:    int64(c.LargeFileMB) << 20,
		WarnOnSecrets:     c.WarnOnSecrets,
		BackupRetention:   time.Duration(c.BackupRetentionDays) * 24 * time.Hour,
	}
}

// OpenRepo opens the repository containing dir and remembers it.
func (a *App) OpenRepo(dir string) (string, error) {
	a.mu.Lock()
	ctx, runner := a.ctx, a.runner
	a.mu.Unlock()
	root, err := runner.TopLevel(ctx, dir)
	if err != nil {
		return "", a.fail(err)
	}
	cfg := a.loadConfig(root)
	guard, err := safety.NewGuard(ctx, runner, root, policy(cfg.Safety))
	if err != nil {
		return "", a.fail(err)
	}
	a.mu.Lock()
	a.root, a.guard, a.cfg = root, guard, cfg
	a.mu.Unlock()
	a.remember(root)
	return root, nil
}

// InitRepo creates a repository in dir and opens it.
func (a *App) InitRepo(dir string) (string, error) {
	a.mu.Lock()
	ctx, runner := a.ctx, a.runner
	a.mu.Unlock()
	if err := runner.Init(ctx, dir, "main"); err != nil {
		return "", a.fail(err)
	}
	return a.OpenRepo(dir)
}

// CloneRepo clones url into dir and opens it.
func (a *App) CloneRepo(url, dir string) (string, error) {
	a.mu.Lock()
	ctx, runner := a.ctx, a.runner
	a.mu.Unlock()
	if err := runner.Clone(ctx, url, dir, git.CloneOptions{}); err != nil {
		return "", a.fail(err)
	}
	return a.OpenRepo(dir)
}

// FileEntry is one changed file.
type FileEntry struct {
	Path       string `json:"path"`
	OrigPath   string `json:"origPath,omitempty"`
	Index      string `json:"index"`
	Worktree   string `json:"worktree"`
	Staged     bool   `json:"staged"`
	Unstaged   bool   `json:"unstaged"`
	Untracked  bool   `json:"untracked"`
	Conflicted bool   `json:"conflicted"`
}

// RepoState is what the main screen shows.
type RepoState struct {
	Root      string           `json:"root"`
	Branch    git.BranchStatus `json:"branch"`
	Operation string           `json:"operation"`
	Files     []FileEntry      `json:"files"`
	Protected bool             `json:"protected"`
}

// State returns the state of the open repository.
func (a *App) State() (RepoState, error) {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return RepoState{}, err
	}
	st, err := runner.Status(ctx, root, git.StatusOptions{})
	if err != nil {
		return RepoState{}, a.fail(err)
	}
	out := RepoState{Root: root, Branch: st.Branch, Operation: string(st.Operation), Files: []FileEntry{}}
	out.Protected = st.Branch.Head != "" && policy(a.cfg.Safety).Protected(st.Branch.Head)
	for _, e := range st.Entries {
		if e.Kind == git.EntryIgnored {
			continue
		}
		state := func(s git.FileState) string {
			if s == 0 {
				return ""
			}
			return string(s)
		}
		out.Files = append(out.Files, FileEntry{
			Path: e.Path, OrigPath: e.OrigPath, Index: state(e.Index), Worktree: state(e.Worktree),
			Staged: e.Staged(), Unstaged: e.Unstaged(), Untracked: e.Kind == git.EntryUntracked, Conflicted: e.Conflicted(),
		})
	}
	return out, nil
}

// Diff returns the parsed changes of one file, staged or not.
func (a *App) Diff(path string, staged bool) ([]git.FileDiff, error) {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return nil, err
	}
	files, err := runner.Diff(ctx, root, git.DiffOptions{Staged: staged, Paths: []string{path}})
	if err != nil {
		return nil, a.fail(err)
	}
	if files == nil {
		files = []git.FileDiff{}
	}
	return files, nil
}

// Stage adds files to the index (staging is a safe operation).
func (a *App) Stage(paths []string) error {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return err
	}
	return a.fail(runner.Stage(ctx, root, paths))
}

// StageAll adds every change.
func (a *App) StageAll() error {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return err
	}
	return a.fail(runner.StageAll(ctx, root))
}

// Unstage takes files out of the index, keeping their changes.
func (a *App) Unstage(paths []string) error {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return err
	}
	return a.fail(runner.Unstage(ctx, root, paths))
}

// Resolve settles conflicted files: side "ours" or "theirs" takes that
// version, "" marks the files resolved as they are now (the user edited them).
func (a *App) Resolve(paths []string, side string) error {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return err
	}
	if side == "" {
		return a.fail(runner.Stage(ctx, root, paths))
	}
	return a.fail(runner.ResolveConflict(ctx, root, paths, git.ConflictSide(side)))
}

// PickFolder asks the user for a folder with the system dialog.
func (a *App) PickFolder() (string, error) {
	if a.deps.PickFolder == nil {
		return "", nil
	}
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	dir, err := a.deps.PickFolder(ctx)
	return dir, a.fail(err)
}

// StageHunks stages (or with staged, unstages) chosen hunks of one file.
func (a *App) StageHunks(path string, staged bool, hunks []int) error {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return err
	}
	files, err := runner.Diff(ctx, root, git.DiffOptions{Staged: staged, Paths: []string{path}})
	if err != nil || len(files) != 1 {
		return a.fail(err)
	}
	if staged {
		return a.fail(runner.UnstageHunks(ctx, root, files[0], hunks))
	}
	return a.fail(runner.StageHunks(ctx, root, files[0], hunks))
}

// Branches lists local and remote-tracking branches.
func (a *App) Branches() ([]git.Branch, error) {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return nil, err
	}
	b, err := runner.Branches(ctx, root, git.BranchListOptions{Remotes: true})
	if b == nil {
		b = []git.Branch{}
	}
	return b, a.fail(err)
}

// Stashes lists the stash.
func (a *App) Stashes() ([]git.Stash, error) {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return nil, err
	}
	s, err := runner.Stashes(ctx, root)
	if s == nil {
		s = []git.Stash{}
	}
	return s, a.fail(err)
}

// Tags lists the tags.
func (a *App) Tags() ([]git.Tag, error) {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return nil, err
	}
	t, err := runner.Tags(ctx, root)
	if t == nil {
		t = []git.Tag{}
	}
	return t, a.fail(err)
}

// Remotes lists the remotes, credentials removed.
func (a *App) Remotes() ([]git.Remote, error) {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return nil, err
	}
	r, err := runner.Remotes(ctx, root)
	return r, a.fail(err)
}

// LogEntry is one row of the history graph.
type LogEntry struct {
	Commit git.Commit `json:"commit"`
	Graph  string     `json:"graph"`
}

// Log returns the history with its graph, newest first.
func (a *App) Log(maxCount int, all bool, search string) ([]LogEntry, error) {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return nil, err
	}
	commits, err := runner.Log(ctx, root, git.LogOptions{Max: maxCount, All: all, Grep: search, IgnoreCase: true})
	if err != nil {
		return nil, a.fail(err)
	}
	rows := graph.Rows(commits)
	out := make([]LogEntry, len(commits))
	for i, c := range commits {
		out[i] = LogEntry{Commit: c, Graph: rows[i]}
	}
	return out, nil
}

// CommitFiles returns what one commit changed.
func (a *App) CommitFiles(rev string) ([]git.FileDiff, error) {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return nil, err
	}
	files, err := runner.Diff(ctx, root, git.DiffOptions{Commit: rev})
	return files, a.fail(err)
}

// HistoryEntries returns what gitpal did here, newest first.
func (a *App) HistoryEntries() ([]history.Entry, error) {
	_, _, _, guard, err := a.current()
	if err != nil {
		return nil, err
	}
	entries, err := guard.History().Entries()
	slices.Reverse(entries)
	if entries == nil {
		entries = []history.Entry{}
	}
	return entries, a.fail(err)
}

// Backups lists the safety backups.
func (a *App) Backups() ([]safety.Backup, error) {
	ctx, _, _, guard, err := a.current()
	if err != nil {
		return nil, err
	}
	b, err := guard.Backups(ctx)
	if b == nil {
		b = []safety.Backup{}
	}
	return b, a.fail(err)
}

// UndoInfo says what Undo would take back.
type UndoInfo struct {
	Possible bool   `json:"possible"`
	Op       string `json:"op"`
	When     string `json:"when"`
	Reason   string `json:"reason"`
}

// PlanUndo returns what Undo would do, or why it cannot.
func (a *App) PlanUndo() (UndoInfo, error) {
	ctx, _, _, guard, err := a.current()
	if err != nil {
		return UndoInfo{}, err
	}
	plan, err := guard.PlanUndo(ctx)
	if err != nil {
		return UndoInfo{}, a.fail(err)
	}
	info := UndoInfo{Possible: plan.Err == nil, Op: plan.Entry.Op}
	if !plan.Entry.Time.IsZero() {
		info.When = plan.Entry.Time.Local().Format("2006-01-02 15:04")
	}
	if plan.Err != nil {
		info.Reason = i18n.Describe(a.lang(), plan.Err)
	}
	return info, nil
}

// Undo takes back the last operation; the frontend has asked the user.
func (a *App) Undo() error {
	ctx, _, _, guard, err := a.current()
	if err != nil {
		return err
	}
	_, err = guard.Undo(ctx, func(safety.Assessment) (safety.Answer, error) { return safety.Answer{Approved: true}, nil }, safety.DoOptions{})
	return a.fail(err)
}

// Settings is every setting as text, for the settings screen.
func (a *App) Settings() (map[string]string, error) {
	out := map[string]string{}
	for _, key := range config.Keys() {
		value, err := config.Get(a.cfg, key)
		if err != nil {
			return nil, a.fail(err)
		}
		out[key] = value
	}
	return out, nil
}

// SetSetting changes one setting in the global file.
func (a *App) SetSetting(key, value string) error {
	path, err := config.GlobalPath(a.deps.ConfigEnv)
	if err != nil {
		return a.fail(err)
	}
	if err := config.SetGlobal(path, key, value); err != nil {
		return a.fail(err)
	}
	a.mu.Lock()
	a.cfg = a.loadConfig(a.root)
	a.mu.Unlock()
	return nil
}

// Identity returns the commit identity.
func (a *App) Identity() (git.Identity, error) {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return git.Identity{}, err
	}
	id, err := runner.Identity(ctx, root)
	return id, a.fail(err)
}

// SetIdentity sets the commit identity, globally or for this repository.
func (a *App) SetIdentity(name, email string, global bool) error {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return err
	}
	return a.fail(runner.SetIdentity(ctx, root, global, git.Identity{Name: name, Email: email}))
}

// SetAIKey stores the AI provider's key in the keychain.
func (a *App) SetAIKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New(i18n.T(a.lang(), i18n.MsgAuthEmpty))
	}
	return a.fail(a.deps.Secrets.Set(secrets.AIKey(string(a.cfg.AI.Provider)), strings.TrimSpace(key)))
}

// HasAIKey reports whether a key is stored (the key itself never leaves Go).
func (a *App) HasAIKey() bool {
	key, err := a.deps.Secrets.Get(secrets.AIKey(string(a.cfg.AI.Provider)))
	return err == nil && key != ""
}
