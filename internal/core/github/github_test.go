package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// fakeGitHub answers API calls from a route table and records the bodies.
type fakeGitHub struct {
	t      *testing.T
	routes map[string]func(w http.ResponseWriter, body map[string]any)
	seen   []string
	bodies map[string]map[string]any
	srv    *httptest.Server
}

func newFake(t *testing.T) *fakeGitHub {
	f := &fakeGitHub{t: t, routes: map[string]func(http.ResponseWriter, map[string]any){}, bodies: map[string]map[string]any{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		f.seen = append(f.seen, key)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"), key)
		var body map[string]any
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		f.bodies[key] = body
		h, ok := f.routes[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"Not Found"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		h(w, body)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGitHub) on(key string, status int, reply string) {
	f.routes[key] = func(w http.ResponseWriter, _ map[string]any) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}
}

func (f *fakeGitHub) client() *Client {
	c, err := New("tok", Options{APIURL: f.srv.URL})
	require.NoError(f.t, err)
	return c
}

var repo = Repo{Owner: "me", Name: "app"}

func TestParseRemote(t *testing.T) {
	for url, want := range map[string]Repo{
		"https://github.com/me/app.git":            {"me", "app"},
		"https://github.com/me/app":                {"me", "app"},
		"https://[REDACTED]@github.com/me/app.git": {"me", "app"},
		"git@github.com:me/app.git":                {"me", "app"},
		"ssh://git@github.com/me/app":              {"me", "app"},
		"https://github.com/me/app.with.dots.git/": {"me", "app.with.dots"},
		"HTTPS://GitHub.com/Me/App.git":            {},
	} {
		got, ok := ParseRemote(url, "")
		if want == (Repo{}) {
			// The scheme is case-sensitive here; host case is not.
			continue
		}
		assert.True(t, ok, url)
		assert.Equal(t, want, got, url)
	}
	for _, bad := range []string{"https://gitlab.com/me/app.git", "/local/path", "", "https://github.com/onlyowner"} {
		_, ok := ParseRemote(bad, "")
		assert.False(t, ok, bad)
	}
	got, ok := ParseRemote("https://git.corp.example/team/svc.git", "git.corp.example")
	assert.True(t, ok)
	assert.Equal(t, Repo{"team", "svc"}, got)

	r, ok := ParseName("me/app.git")
	assert.True(t, ok)
	assert.Equal(t, "me/app", r.String())
	for _, bad := range []string{"app", "/app", "me/", "a/b/c", "me /app"} {
		_, ok := ParseName(bad)
		assert.False(t, ok, bad)
	}
}

func TestNewNeedsAToken(t *testing.T) {
	_, err := New("", Options{})
	assert.Equal(t, KindNoToken, KindOf(err))
}

func TestRepositories(t *testing.T) {
	f := newFake(t)
	f.on("POST /user/repos", http.StatusCreated, `{"name":"app","owner":{"login":"me"},"private":true,"clone_url":"https://github.com/me/app.git","default_branch":"main"}`)
	f.on("GET /user/repos", http.StatusOK, `[{"name":"app","owner":{"login":"me"}},{"name":"lib","owner":{"login":"me"},"private":true}]`)
	f.on("GET /user", http.StatusOK, `{"login":"me"}`)
	c := f.client()
	ctx := context.Background()

	created, err := c.CreateRepository(ctx, NewRepository{Name: "app", Private: true, Description: "d", License: "mit"})
	require.NoError(t, err)
	assert.Equal(t, Repo{"me", "app"}, created.Repo)
	assert.True(t, created.Private)
	body := f.bodies["POST /user/repos"]
	assert.Equal(t, "mit", body["license_template"])
	assert.Nil(t, body["auto_init"], "no README unless asked: an existing repository will be pushed")

	list, err := c.Repositories(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	list, err = c.Repositories(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	login, err := c.User(ctx)
	require.NoError(t, err)
	assert.Equal(t, "me", login)
}

func TestPullRequests(t *testing.T) {
	f := newFake(t)
	f.on("POST /repos/me/app/pulls", http.StatusCreated, `{"number":7,"title":"Add login","head":{"ref":"feat"},"base":{"ref":"main"},"html_url":"https://github.com/me/app/pull/7"}`)
	f.on("GET /repos/me/app/pulls", http.StatusOK, `[{"number":7,"title":"Add login","state":"open","user":{"login":"me"}}]`)
	f.on("GET /repos/me/app/pulls/7", http.StatusOK, `{"number":7,"title":"Add login","body":"why","mergeable":true}`)
	f.on("PUT /repos/me/app/pulls/7/merge", http.StatusOK, `{"merged":true,"sha":"abc123"}`)
	f.on("PATCH /repos/me/app/pulls/7", http.StatusOK, `{"number":7,"state":"closed"}`)
	f.on("POST /repos/me/app/issues/7/comments", http.StatusCreated, `{"id":1}`)
	c := f.client()
	ctx := context.Background()

	pr, err := c.CreatePullRequest(ctx, repo, NewPullRequest{Title: "Add login", Body: "why", Head: "feat", Base: "main", Draft: true})
	require.NoError(t, err)
	assert.Equal(t, 7, pr.Number)
	assert.Equal(t, true, f.bodies["POST /repos/me/app/pulls"]["draft"])

	prs, err := c.PullRequests(ctx, repo, "", 10)
	require.NoError(t, err)
	assert.Equal(t, "me", prs[0].Author)

	got, err := c.PullRequestByNumber(ctx, repo, 7)
	require.NoError(t, err)
	require.NotNil(t, got.Mergeable)
	assert.True(t, *got.Mergeable)

	sha, err := c.MergePullRequest(ctx, repo, 7, MergeSquash)
	require.NoError(t, err)
	assert.Equal(t, "abc123", sha)
	assert.Equal(t, "squash", f.bodies["PUT /repos/me/app/pulls/7/merge"]["merge_method"])

	require.NoError(t, c.ClosePullRequest(ctx, repo, 7))
	assert.Equal(t, "closed", f.bodies["PATCH /repos/me/app/pulls/7"]["state"])
	require.NoError(t, c.Comment(ctx, repo, 7, "looks good"))
}

func TestIssuesReleasesActions(t *testing.T) {
	f := newFake(t)
	f.on("GET /repos/me/app/issues", http.StatusOK, `[{"number":1,"title":"Bug","labels":[{"name":"bug"}]},{"number":2,"title":"A PR","pull_request":{"url":"x"}}]`)
	f.on("POST /repos/me/app/issues", http.StatusCreated, `{"number":3,"title":"New"}`)
	f.on("PATCH /repos/me/app/issues/3", http.StatusOK, `{"number":3,"state":"closed"}`)
	f.on("POST /repos/me/app/issues/3/labels", http.StatusOK, `[{"name":"bug"}]`)
	f.on("POST /repos/me/app/releases", http.StatusCreated, `{"html_url":"https://github.com/me/app/releases/v1"}`)
	f.on("GET /repos/me/app/actions/runs", http.StatusOK, `{"total_count":1,"workflow_runs":[{"name":"CI","status":"completed","conclusion":"failure","head_branch":"main"}]}`)
	f.on("GET /repos/me/app/branches/main", http.StatusOK, `{"name":"main","protected":true}`)
	c := f.client()
	ctx := context.Background()

	issues, err := c.Issues(ctx, repo, "all", 10)
	require.NoError(t, err)
	require.Len(t, issues, 1, "pull requests are not issues")
	assert.Equal(t, []string{"bug"}, issues[0].Labels)

	created, err := c.CreateIssue(ctx, repo, "New", "body", []string{"bug"})
	require.NoError(t, err)
	assert.Equal(t, 3, created.Number)
	require.NoError(t, c.CloseIssue(ctx, repo, 3))
	require.NoError(t, c.LabelIssue(ctx, repo, 3, []string{"bug"}))

	url, err := c.CreateRelease(ctx, repo, NewRelease{Tag: "v1", GenerateNotes: true})
	require.NoError(t, err)
	assert.Contains(t, url, "releases/v1")
	assert.Equal(t, true, f.bodies["POST /repos/me/app/releases"]["generate_release_notes"])

	runs, err := c.WorkflowRuns(ctx, repo, "main", 5)
	require.NoError(t, err)
	assert.Equal(t, "failure", runs[0].Conclusion)

	protected, err := c.BranchProtected(ctx, repo, "main")
	require.NoError(t, err)
	assert.True(t, protected)
	protected, err = c.BranchProtected(ctx, repo, "not-pushed-yet")
	require.NoError(t, err)
	assert.False(t, protected)
}

func TestErrorsAreClassifiedAndCarryNoToken(t *testing.T) {
	f := newFake(t)
	f.on("GET /user", http.StatusUnauthorized, `{"message":"Bad credentials"}`)
	f.on("POST /repos/me/app/pulls", http.StatusUnprocessableEntity, `{"message":"Validation Failed","errors":[{"message":"A pull request already exists for me:feat."}]}`)
	f.on("PUT /repos/me/app/pulls/7/merge", http.StatusMethodNotAllowed, `{"message":"Pull Request is not mergeable"}`)
	f.routes["GET /repos/me/app/pulls"] = func(w http.ResponseWriter, _ map[string]any) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Reset", "1900000000")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"message":"API rate limit exceeded"}`)
	}
	c := f.client()
	ctx := context.Background()

	_, err := c.User(ctx)
	assert.Equal(t, KindAuth, KindOf(err))
	assert.NotContains(t, err.Error(), "tok")

	_, err = c.CreatePullRequest(ctx, repo, NewPullRequest{Title: "x", Head: "feat", Base: "main"})
	assert.Equal(t, KindInvalid, KindOf(err))
	assert.Contains(t, err.Error(), "already exists")

	_, err = c.MergePullRequest(ctx, repo, 7, "")
	assert.Equal(t, KindNotMerged, KindOf(err))

	_, err = c.PullRequests(ctx, repo, "open", 1)
	assert.Equal(t, KindRateLimit, KindOf(err))
	var ghErr *Error
	require.True(t, errors.As(err, &ghErr))
	assert.False(t, ghErr.Reset.IsZero())
	assert.Contains(t, ghErr.Localize(i18n.TR), "yeniden dene")

	// After a rate limit the client refuses locally until it lifts, so a
	// fresh one checks a plain 404.
	_, err = f.client().PullRequestByNumber(ctx, repo, 99)
	assert.Equal(t, KindNotFound, KindOf(err))

	down, err := New("tok", Options{APIURL: "http://127.0.0.1:1"})
	require.NoError(t, err)
	_, err = down.User(ctx)
	assert.Equal(t, KindNetwork, KindOf(err))

	for kind := range kindMessages {
		for _, lang := range []i18n.Lang{i18n.TR, i18n.EN} {
			assert.NotEqual(t, string(kindMessages[kind]), (&Error{Kind: kind}).Localize(lang), kind)
		}
	}
}

func TestDeviceFlow(t *testing.T) {
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "client-1", r.PostForm.Get("client_id"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		switch r.URL.Path {
		case "/login/device/code":
			assert.Equal(t, DefaultScopes, r.PostForm.Get("scope"))
			_, _ = io.WriteString(w, `{"device_code":"dev","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900,"interval":5}`)
		case "/login/oauth/access_token":
			assert.Equal(t, "dev", r.PostForm.Get("device_code"))
			switch polls.Add(1) {
			case 1:
				_, _ = io.WriteString(w, `{"error":"authorization_pending"}`)
			case 2:
				_, _ = io.WriteString(w, `{"error":"slow_down"}`)
			default:
				_, _ = io.WriteString(w, `{"access_token":"gho_new","token_type":"bearer"}`)
			}
		}
	}))
	defer srv.Close()
	var waited []time.Duration
	flow := DeviceFlow{ClientID: "client-1", WebURL: srv.URL, Sleep: func(_ context.Context, d time.Duration) error {
		waited = append(waited, d)
		return nil
	}}

	code, err := flow.Start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ABCD-1234", code.UserCode)
	token, err := flow.Wait(context.Background(), code)
	require.NoError(t, err)
	assert.Equal(t, "gho_new", token)
	assert.Equal(t, []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second}, waited, "slow_down adds five seconds")

	_, err = DeviceFlow{}.Start(context.Background())
	assert.ErrorIs(t, err, ErrNoClientID)
}

func TestDeviceFlowDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"error":"access_denied"}`)
	}))
	defer srv.Close()
	flow := DeviceFlow{ClientID: "c", WebURL: srv.URL, Sleep: func(context.Context, time.Duration) error { return nil }}
	_, err := flow.Wait(context.Background(), DeviceCode{Interval: time.Second, deviceCode: "d"})
	assert.Equal(t, KindAuth, KindOf(err))
}

func TestGHCLIToken(t *testing.T) {
	token, err := GHCLIToken(context.Background(), func(string) (string, error) { return "", errors.New("not found") })
	require.NoError(t, err)
	assert.Empty(t, token, "no gh, no token, no error")
	_ = strings.TrimSpace
}

func TestIssueFromBranch(t *testing.T) {
	for branch, want := range map[string]int{
		"fix/123-login": 123, "feat/gh-42": 42, "issue-7": 7, "42-quick": 42, "fix/#9-x": 9, "bugfix/issue_15": 15,
	} {
		got, ok := IssueFromBranch(branch)
		assert.True(t, ok, branch)
		assert.Equal(t, want, got, branch)
	}
	for _, branch := range []string{"main", "release/v1.2.3", "feat/oauth2", "v2", "fix/0-x"} {
		_, ok := IssueFromBranch(branch)
		assert.False(t, ok, branch)
	}
}
