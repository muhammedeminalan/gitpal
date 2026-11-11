package gui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/ai"
	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
)

// OpRequest is an operation the frontend asks for. Kind is a safety.OpKind
// ("commit", "push", …); the other fields are used by the kinds that need them.
type OpRequest struct {
	Kind       string   `json:"kind"`
	Message    string   `json:"message"`
	Amend      bool     `json:"amend"`
	Paths      []string `json:"paths"`
	Branch     string   `json:"branch"`
	Rev        string   `json:"rev"`
	Mode       string   `json:"mode"`
	Force      bool     `json:"force"`
	Strategy   string   `json:"strategy"`
	StashIndex int      `json:"stashIndex"`
}

// operation turns a request into a safety operation.
func (a *App) operation(r OpRequest) (safety.Operation, error) {
	switch safety.OpKind(r.Kind) {
	case safety.OpCommit, safety.OpAmend:
		return safety.Commit(git.CommitOptions{Message: r.Message, Amend: r.Amend || r.Kind == string(safety.OpAmend)}), nil
	case safety.OpDiscard:
		return safety.Discard(r.Paths...), nil
	case safety.OpReset:
		return safety.Reset(git.ResetMode(r.Mode), r.Rev), nil
	case safety.OpDeleteBranch:
		return safety.DeleteBranch(r.Branch, r.Force), nil
	case safety.OpPush, safety.OpForcePush:
		opts := git.PushOptions{ForceWithLease: r.Force || r.Kind == string(safety.OpForcePush)}
		ctx, runner, root, _, err := a.current()
		if err != nil {
			return safety.Operation{}, err
		}
		if _, ok, err := runner.Upstream(ctx, root, ""); err == nil && !ok {
			opts.SetUpstream = true
		}
		return safety.Push(opts), nil
	case safety.OpPull:
		strategy := git.PullStrategy(r.Strategy)
		if strategy == "" {
			strategy = git.PullStrategy(a.cfg.Pull.Strategy)
		}
		return safety.Pull(git.PullOptions{Strategy: strategy}), nil
	case safety.OpSwitch:
		return safety.Switch(r.Branch), nil
	case safety.OpMerge:
		return safety.Merge(git.MergeOptions{Rev: r.Rev, Mode: git.MergeMode(r.Mode)}), nil
	case safety.OpRebase:
		return safety.Rebase(git.RebaseOptions{Upstream: r.Rev}), nil
	case safety.OpCherryPick:
		return safety.CherryPick(r.Rev), nil
	case safety.OpRevert:
		return safety.Revert(0, r.Rev), nil
	case safety.OpStashDrop:
		return safety.StashDrop(r.StashIndex), nil
	case safety.OpAbort:
		return safety.Abort(), nil
	}
	return safety.Operation{}, errors.New(i18n.T(a.lang(), i18n.MsgGUIUnknownOp, r.Kind))
}

// Assessment is the safety dialog's content.
type Assessment struct {
	Risk        string   `json:"risk"`
	RiskLabel   string   `json:"riskLabel"`
	Confirm     string   `json:"confirm"`
	ConfirmText string   `json:"confirmText"`
	Blocked     bool     `json:"blocked"`
	Effects     []string `json:"effects"`
	Findings    []string `json:"findings"`
	Backups     []string `json:"backups"`
}

var confirmNames = map[safety.Confirmation]string{
	safety.ConfirmNone: "none", safety.ConfirmSimple: "simple",
	safety.ConfirmExplicit: "explicit", safety.ConfirmTypeBranch: "type-branch",
}

// Assess judges an operation without running it: what the safety dialog
// shows before the user decides.
func (a *App) Assess(r OpRequest) (Assessment, error) {
	ctx, _, _, guard, err := a.current()
	if err != nil {
		return Assessment{}, err
	}
	if isStep(r.Kind) {
		// Carrying on or skipping a step needs no confirmation; the guard
		// records it so it can be undone.
		s := safety.Assessment{Risk: safety.RiskSafe}.Summary(a.lang())
		return Assessment{
			Risk: safety.RiskSafe.String(), RiskLabel: s.Risk, Confirm: confirmNames[safety.ConfirmNone],
			Effects: []string{}, Findings: []string{}, Backups: []string{},
		}, nil
	}
	op, err := a.operation(r)
	if err != nil {
		return Assessment{}, err
	}
	as, err := guard.Assess(ctx, op)
	if err != nil {
		return Assessment{}, a.fail(err)
	}
	s := as.Summary(a.lang())
	nonNil := func(v []string) []string {
		if v == nil {
			return []string{}
		}
		return v
	}
	return Assessment{
		Risk: as.Risk.String(), RiskLabel: s.Risk, Confirm: confirmNames[as.Confirm], ConfirmText: as.ConfirmText,
		Blocked: as.Blocked(), Effects: nonNil(s.Effects), Findings: nonNil(s.Findings), Backups: nonNil(s.Backups),
	}, nil
}

// Answer is the user's answer in the safety dialog.
type Answer struct {
	Approved bool   `json:"approved"`
	Typed    string `json:"typed"`
}

// Outcome is what an operation did.
type Outcome struct {
	Message string `json:"message"`
	Backup  string `json:"backup"`
}

// Do runs an operation through the safety layer with the user's answer. The
// guard checks the answer against the risk again: the frontend cannot skip
// a confirmation.
func (a *App) Do(r OpRequest, answer Answer) (Outcome, error) {
	ctx, _, _, guard, err := a.current()
	if err != nil {
		return Outcome{}, err
	}
	if isStep(r.Kind) {
		step := guard.Continue
		if r.Kind == string(safety.OpSkip) {
			step = guard.Skip
		}
		if _, _, err := step(ctx); err != nil {
			return Outcome{}, a.fail(err)
		}
		return Outcome{Message: i18n.T(a.lang(), i18n.MsgDone)}, nil
	}
	op, err := a.operation(r)
	if err != nil {
		return Outcome{}, err
	}
	res, err := guard.Do(ctx, op, func(safety.Assessment) (safety.Answer, error) {
		return safety.Answer{Approved: answer.Approved, Typed: answer.Typed}, nil
	}, safety.DoOptions{})
	if err != nil {
		return Outcome{}, a.fail(err)
	}
	out := Outcome{Message: i18n.T(a.lang(), i18n.MsgDone), Backup: res.Entry.Backup}
	if out.Backup == "" {
		out.Backup = res.Entry.WorktreeBackup
	}
	switch op.Kind {
	case safety.OpCommit, safety.OpAmend:
		subject, _, _ := strings.Cut(r.Message, "\n")
		out.Message = i18n.T(a.lang(), i18n.MsgCommitDone, short(res.Commit.Hash), subject)
	case safety.OpPush, safety.OpForcePush:
		if res.Push.Status == git.PushUpToDate {
			out.Message = i18n.T(a.lang(), i18n.MsgPushUpToDate)
		} else {
			out.Message = i18n.T(a.lang(), i18n.MsgPushDone, res.Push.Branch, res.Push.Remote, res.Push.RemoteBranch)
		}
	case safety.OpPull:
		if res.Pull.Updated {
			out.Message = i18n.T(a.lang(), i18n.MsgPullDone, short(res.Pull.Before), short(res.Pull.After))
		} else {
			out.Message = i18n.T(a.lang(), i18n.MsgPullUpToDate)
		}
	}
	return out, nil
}

// Fetch downloads from the remotes (safe: no local branch changes).
func (a *App) Fetch() error {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return err
	}
	return a.fail(runner.Fetch(ctx, root, git.FetchOptions{All: true, Prune: true}))
}

// CreateBranch creates a branch and switches to it.
func (a *App) CreateBranch(name string) error {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return err
	}
	if err := runner.ValidateBranchName(ctx, root, name); err != nil {
		if s := git.SuggestBranchName(name); s != "" && s != name {
			return errors.New(i18n.Describe(a.lang(), err) + "\n" + i18n.T(a.lang(), i18n.MsgBranchSuggest, s))
		}
		return a.fail(err)
	}
	return a.fail(runner.CreateAndSwitch(ctx, root, name, ""))
}

// AIStatus says whether the AI can be used and whether consent is needed.
type AIStatus struct {
	Enabled bool   `json:"enabled"`
	Consent bool   `json:"consent"`
	HasKey  bool   `json:"hasKey"`
	Name    string `json:"name"`
	// Notice is the consent question to show.
	Notice string `json:"notice"`
}

// AIStatus reports the AI setup.
func (a *App) AIStatus() AIStatus {
	p := string(a.cfg.AI.Provider)
	return AIStatus{
		Enabled: a.cfg.AI.Provider != config.ProviderNone, Consent: a.cfg.AI.ConsentGiven,
		HasKey: !ai.NeedsKey(p) || a.HasAIKey(), Name: p,
		Notice: strings.TrimSuffix(strings.TrimSuffix(i18n.T(a.lang(), i18n.MsgAIConsent, p), " [e/H] "), " [y/N] "),
	}
}

// GiveAIConsent records that the user agreed to sending diffs.
func (a *App) GiveAIConsent() error { return a.SetSetting("ai.consent_given", "true") }

// Suggestion is the AI's proposal for a commit message.
type Suggestion struct {
	Messages []string `json:"messages"`
	Split    string   `json:"split"`
	Sent     string   `json:"sent"`
	Withheld []string `json:"withheld"`
}

// SuggestCommit asks the AI for commit messages for the staged changes.
func (a *App) SuggestCommit() (Suggestion, error) {
	ctx, runner, root, _, err := a.current()
	if err != nil {
		return Suggestion{}, err
	}
	if !a.cfg.AI.ConsentGiven {
		return Suggestion{}, errors.New(i18n.T(a.lang(), i18n.MsgAIConsentNeeded))
	}
	p := string(a.cfg.AI.Provider)
	key := ""
	if a.deps.Secrets != nil {
		key, _ = a.deps.Secrets.Get(secrets.AIKey(p))
	}
	provider, err := a.deps.NewAI(ai.Settings{Provider: p, Model: a.cfg.AI.Model, Key: key})
	if err != nil {
		return Suggestion{}, a.fail(err)
	}
	files, err := runner.Diff(ctx, root, git.DiffOptions{Staged: true})
	if err != nil {
		return Suggestion{}, a.fail(err)
	}
	if len(files) == 0 {
		return Suggestion{}, errors.New(i18n.Describe(a.lang(), &git.Error{Kind: git.KindNothingToCommit}))
	}
	recent, _ := runner.RecentMessages(ctx, root, 10)
	branch, _ := runner.CurrentBranch(ctx, root)
	payload := ai.Prepare(files, 0)
	got, err := provider.GenerateCommitMessages(ctx, ai.CommitInput{
		Payload: payload, RecentMessages: recent, Branch: branch, Style: ai.Style(a.cfg.Commit.Style),
		Language: a.cfg.Commit.MessageLanguage, Suggestions: a.cfg.Commit.Suggestions, MaxSubjectLength: a.cfg.Commit.MaxSubjectLength,
	})
	if err != nil {
		return Suggestion{}, a.fail(err)
	}
	withheld := payload.Withheld
	if withheld == nil {
		withheld = []string{}
	}
	return Suggestion{
		Messages: got.Messages, Split: got.Split, Withheld: withheld,
		Sent: i18n.T(a.lang(), i18n.MsgAISent, payload.Chars(), payload.ApproxTokens()),
	}, nil
}

// recentFile keeps the recently opened repositories.
func (a *App) recentFile() string {
	path, err := config.LogPath(a.deps.ConfigEnv)
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(path), "recent.json")
}

// maxRecent is how many repositories the welcome screen lists.
const maxRecent = 10

// RecentRepos lists recently opened repositories that still exist.
func (a *App) RecentRepos() []string {
	data, err := os.ReadFile(a.recentFile())
	var list []string
	if err != nil || json.Unmarshal(data, &list) != nil {
		return []string{}
	}
	return slices.DeleteFunc(list, func(p string) bool {
		_, err := os.Stat(p)
		return err != nil
	})
}

func (a *App) remember(root string) {
	file := a.recentFile()
	if file == "" {
		return
	}
	list := slices.DeleteFunc(a.RecentRepos(), func(p string) bool { return p == root })
	list = append([]string{root}, list...)
	if len(list) > maxRecent {
		list = list[:maxRecent]
	}
	data, _ := json.Marshal(list)
	_ = os.MkdirAll(filepath.Dir(file), 0o700)
	_ = os.WriteFile(file, data, 0o600)
}

func short(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

// isStep reports whether kind moves an operation in progress on.
func isStep(kind string) bool {
	return kind == string(safety.OpContinue) || kind == string(safety.OpSkip)
}
