package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/cli"
	"github.com/muhammedeminalan/gitpal/internal/core/git/gittest"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
)

// fakeGitHub is a scripted GitHub API.
type fakeGitHub struct {
	mu     sync.Mutex
	routes map[string]string
	status map[string]int
	bodies map[string]map[string]any
	srv    *httptest.Server
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	f := &fakeGitHub{routes: map[string]string{}, status: map[string]int{}, bodies: map[string]map[string]any{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		key := r.Method + " " + r.URL.Path
		if r.Header.Get("Authorization") != "Bearer ghp_test" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"message":"Bad credentials"}`)
			return
		}
		var body map[string]any
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		f.bodies[key] = body
		reply, ok := f.routes[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"Not Found"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if s := f.status[key]; s != 0 {
			w.WriteHeader(s)
		}
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGitHub) on(key, reply string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[key] = reply
}

func (f *fakeGitHub) body(key string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[key]
}

// signedIn returns a harness in a repository whose origin is a local bare
// repository that the fake GitHub calls me/app.
func signedIn(t *testing.T) (*harness, *gittest.Repo, *gittest.Repo, *fakeGitHub) {
	t.Helper()
	h, repo := inRepo(t)
	f := newFakeGitHub(t)
	h.vars["GITPAL_GITHUB_API_URL"] = f.srv.URL
	require.NoError(t, h.secrets.Set(secrets.GitHubToken, "ghp_test"))
	origin := gittest.NewBareRepo(t)
	repo.Git("remote", "add", "origin", origin.Dir)
	repo.Git("push", "--quiet", "origin", "main")
	// gitpal reads owner and name from a GitHub URL; the push still goes to
	// the bare repository through a pushurl.
	repo.Git("remote", "set-url", "origin", "https://github.com/me/app.git")
	repo.Git("remote", "set-url", "--push", "origin", origin.Dir)
	repo.Git("config", "remote.origin.url", "https://github.com/me/app.git")
	return h, repo, origin, f
}

func TestGitHubNeedsSigningIn(t *testing.T) {
	h, _ := inRepo(t)
	assert.Equal(t, cli.ExitError, h.run("pr", "list"))
	assert.Contains(t, h.stderr.String(), "gitpal auth github")
}

func TestAuthGitHubWithAToken(t *testing.T) {
	h := newHarness(t)
	h.writeConfig("language: en\n")
	f := newFakeGitHub(t)
	f.on("GET /user", `{"login":"octo"}`)
	h.vars["GITPAL_GITHUB_API_URL"] = f.srv.URL

	h.env.Stdin = strings.NewReader("wrong\n")
	assert.Equal(t, cli.ExitError, h.run("auth", "github", "--token"))
	_, err := h.secrets.Get(secrets.GitHubToken)
	assert.Error(t, err, "a token GitHub refuses is not kept")

	h.env.Stdin = strings.NewReader("ghp_test\n")
	require.Equal(t, cli.ExitOK, h.run("auth", "github", "--token"), h.stderr.String())
	assert.Contains(t, h.stdout.String(), "Signed in to GitHub as octo")
	assert.NotContains(t, h.stdout.String()+h.stderr.String(), "ghp_test")
	got, err := h.secrets.Get(secrets.GitHubToken)
	require.NoError(t, err)
	assert.Equal(t, "ghp_test", got)

	t.Run("without an OAuth app in this build", func(t *testing.T) {
		assert.Equal(t, cli.ExitError, h.run("auth", "github"))
		assert.Contains(t, h.stderr.String(), "--token")
	})
}

func TestAuthGitHubDeviceFlow(t *testing.T) {
	h := newHarness(t)
	h.writeConfig("language: en\n")
	f := newFakeGitHub(t)
	f.on("GET /user", `{"login":"octo"}`)
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/device/code":
			_, _ = io.WriteString(w, `{"device_code":"d","user_code":"WXYZ-1234","verification_uri":"https://github.com/login/device","expires_in":60,"interval":0}`)
		default:
			_, _ = io.WriteString(w, `{"access_token":"ghp_test"}`)
		}
	}))
	defer web.Close()
	h.vars["GITPAL_GITHUB_API_URL"] = f.srv.URL
	h.vars["GITPAL_GITHUB_WEB_URL"] = web.URL
	h.vars["GITPAL_GITHUB_CLIENT_ID"] = "client"

	require.Equal(t, cli.ExitOK, h.run("auth", "github"), h.stderr.String())
	assert.Contains(t, h.stderr.String(), "WXYZ-1234")
	got, err := h.secrets.Get(secrets.GitHubToken)
	require.NoError(t, err)
	assert.Equal(t, "ghp_test", got)
}

func TestPullRequestFlow(t *testing.T) {
	h, repo, origin, f := signedIn(t)
	f.on("GET /repos/me/app", `{"name":"app","owner":{"login":"me"},"default_branch":"main"}`)
	f.on("POST /repos/me/app/pulls", `{"number":12,"title":"x","html_url":"https://github.com/me/app/pull/12"}`)
	f.status["POST /repos/me/app/pulls"] = http.StatusCreated
	f.on("GET /repos/me/app/pulls", `[{"number":12,"title":"Add login","user":{"login":"me"},"head":{"ref":"work"},"base":{"ref":"main"}}]`)
	f.on("GET /repos/me/app/pulls/12", `{"number":12,"title":"Add login","head":{"ref":"work"},"base":{"ref":"main"},"state":"open"}`)
	f.on("PUT /repos/me/app/pulls/12/merge", `{"merged":true,"sha":"0123456789abcdef"}`)
	f.on("GET /repos/me/app/branches/work", `{"name":"work","protected":false}`)
	repo.WriteFile("login.go", "package login\n")
	repo.Commit("feat: add login")

	require.Equal(t, cli.ExitOK, h.run("pr", "create"), h.stderr.String())
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), origin.Git("rev-parse", "refs/heads/work"), "the branch was pushed first")
	body := f.body("POST /repos/me/app/pulls")
	assert.Equal(t, "feat: add login", body["title"], "a single commit names the pull request")
	assert.Equal(t, "work", body["head"])
	assert.Equal(t, "main", body["base"])
	assert.Contains(t, h.stdout.String(), "#12")

	require.Equal(t, cli.ExitOK, h.run("pr", "list"))
	assert.Contains(t, h.stdout.String(), "#12 Add login")
	require.Equal(t, cli.ExitOK, h.run("pr", "view", "#12"))
	assert.Contains(t, h.stdout.String(), "work → main")

	assert.Equal(t, cli.ExitBlocked, h.run("pr", "merge", "12"), "merging needs a yes")
	h.answer("y")
	require.Equal(t, cli.ExitOK, h.run("pr", "merge", "12", "--squash"))
	assert.Equal(t, "squash", f.body("PUT /repos/me/app/pulls/12/merge")["merge_method"])

	assert.Equal(t, cli.ExitError, h.run("pr", "view", "abc"))
}

func TestIssuesReleaseActionsAndHints(t *testing.T) {
	h, repo, _, f := signedIn(t)
	f.on("GET /repos/me/app/issues", `[{"number":3,"title":"Crash on login","labels":[{"name":"bug"}]}]`)
	f.on("POST /repos/me/app/issues", `{"number":4,"title":"New","html_url":"u"}`)
	f.on("PATCH /repos/me/app/issues/4", `{"number":4}`)
	f.on("POST /repos/me/app/releases", `{"html_url":"https://github.com/me/app/releases/tag/v1"}`)
	f.on("GET /repos/me/app/actions/runs", `{"total_count":1,"workflow_runs":[{"name":"CI","status":"completed","conclusion":"failure","head_sha":"abcdef123"}]}`)
	f.on("GET /repos/me/app/branches/fix/3-crash", `{"name":"fix/3-crash","protected":true}`)

	require.Equal(t, cli.ExitOK, h.run("issue", "list"))
	assert.Contains(t, h.stdout.String(), "#3 Crash on login [bug]")
	require.Equal(t, cli.ExitOK, h.run("issue", "create", "-t", "New", "-l", "bug,ui"))
	assert.Equal(t, []any{"bug", "ui"}, f.body("POST /repos/me/app/issues")["labels"])
	require.Equal(t, cli.ExitOK, h.run("issue", "close", "4"))

	require.Equal(t, cli.ExitOK, h.run("release", "create", "v1", "--generate-notes"))
	assert.Contains(t, h.stdout.String(), "releases/tag/v1")

	require.Equal(t, cli.ExitOK, h.run("actions"))
	assert.Contains(t, h.stdout.String(), "✗ failure")

	repo.Git("switch", "--quiet", "-c", "fix/3-crash")
	repo.WriteFile("a.txt", "fixed\n")
	repo.Git("add", "a.txt")
	require.Equal(t, cli.ExitOK, h.run("commit", "-m", "fix: crash"))
	assert.Contains(t, h.stderr.String(), "Fixes #3")

	require.Equal(t, cli.ExitOK, h.run("push"), h.stderr.String())
	assert.Contains(t, h.stderr.String(), "protected on GitHub")
}

func TestRepoCreateConnectsAndPushes(t *testing.T) {
	h, repo := inRepo(t)
	f := newFakeGitHub(t)
	h.vars["GITPAL_GITHUB_API_URL"] = f.srv.URL
	require.NoError(t, h.secrets.Set(secrets.GitHubToken, "ghp_test"))
	origin := gittest.NewBareRepo(t)
	reply, _ := json.Marshal(map[string]any{
		"name": "newapp", "owner": map[string]any{"login": "me"}, "clone_url": origin.Dir, "html_url": "https://github.com/me/newapp",
	})
	f.on("POST /user/repos", string(reply))
	f.status["POST /user/repos"] = http.StatusCreated

	require.Equal(t, cli.ExitOK, h.run("repo", "create", "newapp", "--private", "--push"), h.stderr.String())
	assert.Equal(t, true, f.body("POST /user/repos")["private"])
	assert.Equal(t, origin.Dir, repo.Git("remote", "get-url", "origin"))
	assert.Equal(t, repo.Git("rev-parse", "HEAD"), origin.Git("rev-parse", "refs/heads/work"))

	require.Equal(t, cli.ExitOK, h.run("repo", "create", "again"))
	assert.Contains(t, h.stdout.String(), "already has an `origin`")
}
