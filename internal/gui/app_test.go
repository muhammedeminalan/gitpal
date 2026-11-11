package gui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/ai"
	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets/secretstest"
)

type stubAI struct{ in ai.CommitInput }

func (s *stubAI) Name() string { return "stub" }
func (s *stubAI) GenerateCommitMessages(_ context.Context, in ai.CommitInput) (ai.CommitSuggestions, error) {
	s.in = in
	return ai.CommitSuggestions{Messages: []string{"feat: suggested"}}, nil
}

func (s *stubAI) GeneratePRDescription(context.Context, ai.PRInput) (ai.PRText, error) {
	return ai.PRText{}, nil
}

func newApp(t *testing.T, configYAML string) (*App, *gittest.Repo, *stubAI) {
	t.Helper()
	home := gittest.TempDir(t)
	vars := map[string]string{"GITPAL_CONFIG": filepath.Join(home, "config.yaml"), "XDG_STATE_HOME": filepath.Join(home, "state"), "LOCALAPPDATA": filepath.Join(home, "state")}
	env := config.Env{GOOS: "linux", Getenv: func(k string) string { return vars[k] }, HomeDir: func() (string, error) { return home, nil }}
	if configYAML != "" {
		path, _ := config.GlobalPath(env)
		require.NoError(t, config.SetGlobal(path, "language", "en"))
		for _, kv := range strings.Split(configYAML, ";") {
			k, v, _ := strings.Cut(kv, "=")
			require.NoError(t, config.SetGlobal(path, k, v))
		}
	}
	stub := &stubAI{}
	app := New(Deps{
		NewRunner: func(opts ...git.Option) *git.Runner { return gittest.NewRunner(t, gittest.WithGitOptions(opts...)) },
		ConfigEnv: env, Secrets: secretstest.NewMemory(),
		NewAI: func(ai.Settings) (ai.Provider, error) { return stub, nil },
	})
	repo := gittest.NewRepo(t)
	repo.WriteFile("a.txt", "one\n")
	repo.Commit("first")
	repo.Git("switch", "--quiet", "-c", "work")
	return app, repo, stub
}

func TestNeedsARepository(t *testing.T) {
	app, _, _ := newApp(t, "language=en")
	_, err := app.State()
	assert.EqualError(t, err, "Open a repository first.")
	info := app.Info()
	assert.Equal(t, "en", info.Lang)
	assert.Equal(t, "Choose a repository", info.Texts["gui.welcome.title"])
	assert.Greater(t, len(info.Texts), 50)
}

func TestOpenStageCommitUndo(t *testing.T) {
	app, repo, _ := newApp(t, "language=en")
	root, err := app.OpenRepo(repo.Dir)
	require.NoError(t, err)
	assert.Equal(t, repo.Dir, root)
	assert.Equal(t, []string{repo.Dir}, app.RecentRepos())
	// The window opened with a repository (gitpal-gui <path>) starts in it.
	assert.Equal(t, repo.Dir, app.Info().Root)

	repo.WriteFile("a.txt", "two\n")
	repo.WriteFile("new.txt", "new\n")
	st, err := app.State()
	require.NoError(t, err)
	assert.Equal(t, "work", st.Branch.Head)
	require.Len(t, st.Files, 2)
	assert.False(t, st.Protected)

	diff, err := app.Diff("a.txt", false)
	require.NoError(t, err)
	assert.Equal(t, "two", diff[0].Hunks[0].Lines[1].Text)

	require.NoError(t, app.Stage([]string{"a.txt"}))
	require.NoError(t, app.StageAll())
	require.NoError(t, app.Unstage([]string{"new.txt"}))

	req := OpRequest{Kind: "commit", Message: "feat: from the window"}
	as, err := app.Assess(req)
	require.NoError(t, err)
	assert.Equal(t, "safe", as.Risk)
	assert.Equal(t, "none", as.Confirm)
	out, err := app.Do(req, Answer{})
	require.NoError(t, err)
	assert.Contains(t, out.Message, "feat: from the window")
	assert.Equal(t, "feat: from the window", repo.Git("log", "-1", "--format=%s"))

	log, err := app.Log(10, false, "")
	require.NoError(t, err)
	require.Len(t, log, 2)
	assert.Equal(t, "*", log[0].Graph)

	plan, err := app.PlanUndo()
	require.NoError(t, err)
	assert.True(t, plan.Possible)
	assert.Equal(t, "commit", plan.Op)
	require.NoError(t, app.Undo())
	assert.Equal(t, "first", repo.Git("log", "-1", "--format=%s"))

	entries, err := app.HistoryEntries()
	require.NoError(t, err)
	assert.Equal(t, "undo", entries[0].Op, "newest first")
	assert.NotEmpty(t, app.Commands(), "the commands panel sees what ran")
}

func TestTheFrontendCannotSkipAConfirmation(t *testing.T) {
	app, repo, _ := newApp(t, "language=en")
	_, err := app.OpenRepo(repo.Dir)
	require.NoError(t, err)
	repo.WriteFile("a.txt", "precious\n")
	req := OpRequest{Kind: "discard", Paths: []string{"a.txt"}}

	as, err := app.Assess(req)
	require.NoError(t, err)
	assert.Equal(t, "dangerous", as.Risk)
	assert.Equal(t, "explicit", as.Confirm)
	assert.NotEmpty(t, as.Backups)

	_, err = app.Do(req, Answer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not confirmed")
	assert.Equal(t, "precious\n", repo.ReadFile("a.txt"))

	out, err := app.Do(req, Answer{Approved: true})
	require.NoError(t, err)
	assert.Contains(t, out.Backup, "refs/gitpal/backup-worktree/")
	assert.Equal(t, "one\n", repo.ReadFile("a.txt"))

	backups, err := app.Backups()
	require.NoError(t, err)
	assert.Len(t, backups, 1)
}

func TestProtectedBranchAndForcePush(t *testing.T) {
	app, repo, _ := newApp(t, "language=en")
	origin := gittest.NewBareRepo(t)
	repo.Git("remote", "add", "origin", origin.Dir)
	repo.Git("switch", "--quiet", "main")
	repo.Git("push", "--quiet", "--set-upstream", "origin", "main")
	repo.Git("commit", "--quiet", "--amend", "-m", "rewritten")
	_, err := app.OpenRepo(repo.Dir)
	require.NoError(t, err)

	st, err := app.State()
	require.NoError(t, err)
	assert.True(t, st.Protected)

	as, err := app.Assess(OpRequest{Kind: "force-push"})
	require.NoError(t, err)
	assert.True(t, as.Blocked)
	_, err = app.Do(OpRequest{Kind: "force-push"}, Answer{Approved: true, Typed: "main"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

func TestBranchesAndSettings(t *testing.T) {
	app, repo, _ := newApp(t, "language=en")
	_, err := app.OpenRepo(repo.Dir)
	require.NoError(t, err)

	err = app.CreateBranch("Yeni Özellik")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yeni-ozellik")
	require.NoError(t, app.CreateBranch("feat/x"))
	branches, err := app.Branches()
	require.NoError(t, err)
	assert.Len(t, branches, 3)

	_, err = app.Do(OpRequest{Kind: "switch", Branch: "work"}, Answer{})
	require.NoError(t, err)
	assert.Equal(t, "work", repo.Git("branch", "--show-current"))

	settings, err := app.Settings()
	require.NoError(t, err)
	assert.Equal(t, "en", settings["language"])
	require.NoError(t, app.SetSetting("language", "tr"))
	assert.Equal(t, "tr", app.Info().Lang)
	assert.Error(t, app.SetSetting("language", "de"))

	require.NoError(t, app.SetIdentity("Ayşe", "", false))
	id, err := app.Identity()
	require.NoError(t, err)
	assert.Equal(t, "Ayşe", id.Name)

	assert.False(t, app.HasAIKey())
	require.NoError(t, app.SetAIKey(" sk-key "))
	assert.True(t, app.HasAIKey())
	got, _ := app.deps.Secrets.Get(secrets.AIKey("anthropic"))
	assert.Equal(t, "sk-key", got)

	_, err = app.Assess(OpRequest{Kind: "nope"})
	assert.Error(t, err)
}

func TestSuggestCommitNeedsConsent(t *testing.T) {
	app, repo, stub := newApp(t, "language=en")
	_, err := app.OpenRepo(repo.Dir)
	require.NoError(t, err)
	repo.WriteFile(".env", "SECRET=from-env\n")
	repo.WriteFile("a.txt", "two\n")
	repo.Git("add", "--all")

	status := app.AIStatus()
	assert.False(t, status.Consent)
	assert.Contains(t, status.Notice, "anthropic")
	_, err = app.SuggestCommit()
	require.Error(t, err)

	require.NoError(t, app.GiveAIConsent())
	got, err := app.SuggestCommit()
	require.NoError(t, err)
	assert.Equal(t, []string{"feat: suggested"}, got.Messages)
	assert.Equal(t, []string{".env"}, got.Withheld)
	assert.NotContains(t, stub.in.Payload.Text, "from-env")
}

func TestResolveAConflictAndContinue(t *testing.T) {
	app, repo, _ := newApp(t, "language=en")
	repo.Git("switch", "--quiet", "-c", "other")
	repo.WriteFile("a.txt", "theirs\n")
	repo.Commit("other side")
	repo.Git("switch", "--quiet", "work")
	repo.WriteFile("a.txt", "ours\n")
	repo.Commit("our side")
	_, err := app.OpenRepo(repo.Dir)
	require.NoError(t, err)

	_, err = app.Do(OpRequest{Kind: "merge", Rev: "other"}, Answer{Approved: true})
	require.Error(t, err)
	st, err := app.State()
	require.NoError(t, err)
	assert.Equal(t, "merge", st.Operation)
	require.Len(t, st.Files, 1)
	assert.True(t, st.Files[0].Conflicted)

	assert.Error(t, app.Resolve([]string{"a.txt"}, "sideways"))
	require.NoError(t, app.Resolve([]string{"a.txt"}, "theirs"))
	assert.Equal(t, "theirs\n", repo.ReadFile("a.txt"))

	as, err := app.Assess(OpRequest{Kind: "continue"})
	require.NoError(t, err)
	assert.Equal(t, "none", as.Confirm)
	_, err = app.Do(OpRequest{Kind: "continue"}, Answer{Approved: true})
	require.NoError(t, err)
	st, err = app.State()
	require.NoError(t, err)
	assert.Empty(t, st.Operation)
}

func TestPickFolder(t *testing.T) {
	app, _, _ := newApp(t, "")
	// Without a native dialog (tests, a stripped build) there is nothing to pick.
	dir, err := app.PickFolder()
	require.NoError(t, err)
	assert.Empty(t, dir)

	app.deps.PickFolder = func(context.Context) (string, error) { return "/somewhere", nil }
	dir, err = app.PickFolder()
	require.NoError(t, err)
	assert.Equal(t, "/somewhere", dir)
}
