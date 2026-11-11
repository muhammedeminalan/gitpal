package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// fileDiff builds a diff of added lines.
func fileDiff(path string, lines ...string) git.FileDiff {
	h := git.Hunk{OldStart: 0, NewStart: 1, NewLines: len(lines)}
	for i, l := range lines {
		h.Lines = append(h.Lines, git.DiffLine{Kind: git.LineAdded, Text: l, NewLine: i + 1})
	}
	return git.FileDiff{Path: path, Change: git.ChangeAdded, Hunks: []git.Hunk{h}}
}

var (
	testToken = "ghp" + "_" + strings.Repeat("a1B2", 9)
	randomKey = "Zq8" + "vN2xLpT7wR4kY9mB3cH6jF1dS5gA0e"
)

func TestRedact(t *testing.T) {
	in := "token = " + testToken + "\nkey: " + randomKey + "\ncommit 3f9a1c0d7b2e4a6f3f9a1c0d7b2e4a6f3f9a1c0d\nTestDashboardTypedConfirmationWorks\n"
	out := Redact(in)
	assert.NotContains(t, out, testToken)
	assert.NotContains(t, out, randomKey, "a random-looking token is redacted by entropy")
	assert.Contains(t, out, "3f9a1c0d7b2e4a6f3f9a1c0d7b2e4a6f3f9a1c0d", "a commit hash is not a secret")
	assert.Contains(t, out, "TestDashboardTypedConfirmationWorks", "an identifier is not a secret")
}

// Spec §12 (mandatory): the content of .env never reaches the payload.
func TestEnvFileContentIsNeverSent(t *testing.T) {
	p := Prepare([]git.FileDiff{
		fileDiff(".env", "DATABASE_PASSWORD=hunter2-very-secret", "STRIPE=sk_live_abcdefghijklmnop1234"),
		fileDiff("config/.env.production", "API=prod-secret-value"),
		fileDiff("certs/server.pem", "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQ"),
		fileDiff("notes.txt", "-----BEGIN OPENSSH PRIVATE KEY-----", "b3BlbnNzaC1rZXktdjEAAAAABG5vbmU"),
		fileDiff("main.go", "package main", `const token = "`+testToken+`"`),
	}, 0)

	for _, secret := range []string{"hunter2-very-secret", "sk_live_abcdefghijklmnop1234", "prod-secret-value", "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQ", "b3BlbnNzaC1rZXktdjEAAAAABG5vbmU", testToken} {
		assert.NotContains(t, p.Text, secret)
	}
	assert.ElementsMatch(t, []string{".env", "config/.env.production", "certs/server.pem", "notes.txt"}, p.Withheld)
	assert.Contains(t, p.Text, ".env", "the file is named, so the model knows it changed")
	assert.Contains(t, p.Text, "package main")
	assert.Contains(t, p.Text, "[REDACTED]")
}

func TestPrepareSummarizesGeneratedFiles(t *testing.T) {
	p := Prepare([]git.FileDiff{
		fileDiff("go.sum", "github.com/x v1.0.0 h1:abc"),
		fileDiff("web/app.min.js", "var a=1"),
		fileDiff("bundle.js", strings.Repeat("x", 2000)),
		{Path: "logo.png", Change: git.ChangeModified, Binary: true},
		fileDiff("src/real.go", "func Real() {}"),
	}, 0)

	assert.ElementsMatch(t, []string{"go.sum", "web/app.min.js", "bundle.js", "logo.png"}, p.Summarized)
	assert.NotContains(t, p.Text, "h1:abc")
	assert.NotContains(t, p.Text, "var a=1")
	assert.Contains(t, p.Text, "func Real() {}")
	assert.Contains(t, p.Text, "added src/real.go (+1 -0)")
	assert.False(t, p.Truncated)
	assert.Equal(t, len(p.Text), p.Chars())
	assert.Equal(t, (p.Chars()+3)/4, p.ApproxTokens())
}

func TestPrepareKeepsALargeDiffWithinBudget(t *testing.T) {
	big := make([]string, 5000)
	for i := range big {
		big[i] = "line of a very large generated-looking but legitimate change"
	}
	p := Prepare([]git.FileDiff{fileDiff("big.txt", big...), fileDiff("small.go", "func Small() {}")}, 10_000)

	assert.True(t, p.Truncated)
	assert.Less(t, p.Chars(), 12_000)
	assert.Contains(t, p.Text, "func Small() {}", "a big file does not crowd out the others")
	assert.Contains(t, p.Text, "more lines not shown")
}

func TestCommitPrompt(t *testing.T) {
	system, user := commitPrompt(CommitInput{
		Payload: Payload{Text: "Changed files:\n- added a.go (+1 -0)\n"}, RecentMessages: []string{"feat: one", "fix: two " + testToken},
		Branch: "feat/login", Style: StyleConventional, Language: i18n.TR, Suggestions: 2, MaxSubjectLength: 50,
	})
	assert.Contains(t, system, "at most 50 characters")
	assert.Contains(t, system, "Conventional Commits")
	assert.Contains(t, system, "Turkish")
	assert.Contains(t, system, "2 alternative commit messages")
	assert.Contains(t, user, "Branch: feat/login")
	assert.Contains(t, user, "feat: one")
	assert.NotContains(t, user, testToken, "recent messages are redacted too")

	system, _ = commitPrompt(CommitInput{Style: StyleFree, Language: i18n.EN, Suggestions: 9})
	assert.Contains(t, system, "no prefix convention")
	assert.Contains(t, system, "3 alternative", "at most three")
	assert.Contains(t, system, "at most 72 characters")
}

func TestParseCommit(t *testing.T) {
	got := parseCommit("Here you go:\n```json\n{\"messages\": [\"feat: a\", \"  \", \"feat: b\\n\\nwhy\"], \"split\": \"a.go and b.go\"}\n```", 72)
	assert.Equal(t, []string{"feat: a", "feat: b\n\nwhy"}, got.Messages)
	assert.Equal(t, "a.go and b.go", got.Split)

	assert.Equal(t, []string{"fix: plain text answer"}, parseCommit("fix: plain text answer", 72).Messages)
	assert.Empty(t, parseCommit("", 72).Messages)
	assert.Empty(t, parseCommit("{broken json", 72).Messages)
}

// fakeAnthropic answers like the Messages API and records what it got.
func fakeAnthropic(t *testing.T, status int, text string, stop string) (*httptest.Server, *[]map[string]any, *[]http.Header) {
	t.Helper()
	var bodies []map[string]any
	var headers []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/messages", r.URL.Path)
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		require.NoError(t, json.Unmarshal(raw, &body))
		bodies = append(bodies, body)
		headers = append(headers, r.Header.Clone())
		w.Header().Set("Content-Type", "application/json")
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
			return
		}
		reply, _ := json.Marshal(map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant", "model": body["model"],
			"content":     []map[string]any{{"type": "text", "text": text}},
			"stop_reason": stop, "usage": map[string]any{"input_tokens": 10, "output_tokens": 5},
		})
		_, _ = w.Write(reply)
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies, &headers
}

func TestAnthropicProvider(t *testing.T) {
	srv, bodies, headers := fakeAnthropic(t, http.StatusOK, `{"messages":["feat: add login","feat(auth): add login"],"split":""}`, "end_turn")
	p := NewAnthropic("sk-test-key", "", srv.URL)

	got, err := p.GenerateCommitMessages(context.Background(), CommitInput{
		Payload: Prepare([]git.FileDiff{fileDiff(".env", "SECRET=from-env-file"), fileDiff("login.go", "func Login() {}")}, 0),
		Style:   StyleConventional, Language: i18n.EN, Suggestions: 2,
	})
	require.NoError(t, err)

	assert.Equal(t, "anthropic", p.Name())
	assert.Equal(t, []string{"feat: add login", "feat(auth): add login"}, got.Messages)
	require.Len(t, *bodies, 1)
	body := (*bodies)[0]
	assert.Equal(t, DefaultAnthropicModel, body["model"])
	assert.Equal(t, "default", body["fallbacks"])
	assert.Contains(t, (*headers)[0].Get("Anthropic-Beta"), "server-side-fallback-2026-07-01")
	h := (*headers)[0]
	assert.True(t, h.Get("X-Api-Key") == "sk-test-key" || h.Get("Authorization") == "Bearer sk-test-key", "the key is sent")
	raw, _ := json.Marshal(body)
	assert.NotContains(t, string(raw), "from-env-file", "what is sent over the wire holds no .env content")
	assert.Contains(t, string(raw), "func Login() {}")
}

func TestAnthropicErrors(t *testing.T) {
	t.Run("bad key", func(t *testing.T) {
		srv, _, _ := fakeAnthropic(t, http.StatusUnauthorized, "", "")
		_, err := NewAnthropic("sk-wrong-key-123", "", srv.URL).GenerateCommitMessages(context.Background(), CommitInput{})
		assert.Equal(t, KindAuth, KindOf(err))
		assert.NotContains(t, err.Error(), "sk-wrong-key-123")
		var aiErr *Error
		require.ErrorAs(t, err, &aiErr)
		assert.Contains(t, aiErr.Localize(i18n.TR), "gitpal auth ai")
	})

	t.Run("refusal", func(t *testing.T) {
		srv, _, _ := fakeAnthropic(t, http.StatusOK, "", "refusal")
		_, err := NewAnthropic("k", "", srv.URL).GenerateCommitMessages(context.Background(), CommitInput{})
		assert.Equal(t, KindRefused, KindOf(err))
	})

	t.Run("unusable answers are retried, then given up", func(t *testing.T) {
		srv, bodies, _ := fakeAnthropic(t, http.StatusOK, "  ", "end_turn")
		_, err := NewAnthropic("k", "", srv.URL).GenerateCommitMessages(context.Background(), CommitInput{})
		assert.Equal(t, KindEmpty, KindOf(err))
		assert.Len(t, *bodies, 3, "the first try and two retries")
	})

	t.Run("nothing listening", func(t *testing.T) {
		_, err := NewAnthropic("k", "", "http://127.0.0.1:1").GenerateCommitMessages(context.Background(), CommitInput{})
		assert.Equal(t, KindNetwork, KindOf(err))
	})
}

func TestOpenAICompatibleProvider(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-openai", r.Header.Get("Authorization"))
		var body struct {
			Model    string              `json:"model"`
			Messages []map[string]string `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "some-model", body.Model)
		assert.Equal(t, "system", body.Messages[0]["role"])
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"title\":\"Add login\",\"body\":\"Because.\"}"}}]}`)
	}))
	defer srv.Close()

	p := NewOpenAI("sk-openai", "some-model", srv.URL+"/v1/")
	pr, err := p.GeneratePRDescription(context.Background(), PRInput{Branch: "feat", Base: "main", Commits: []string{"feat: a"}})
	require.NoError(t, err)
	assert.Equal(t, PRText{Title: "Add login", Body: "Because."}, pr)

	t.Run("rate limit", func(t *testing.T) {
		limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer limited.Close()
		_, err := NewOpenAI("k", "m", limited.URL).GenerateCommitMessages(context.Background(), CommitInput{})
		assert.Equal(t, KindRateLimit, KindOf(err))
	})
}

func TestOllamaProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Empty(t, r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, `{"message":{"content":"{\"messages\":[\"chore: local\"]}"}}`)
	}))
	defer srv.Close()

	got, err := NewOllama("llama", srv.URL).GenerateCommitMessages(context.Background(), CommitInput{Suggestions: 1})
	require.NoError(t, err)
	assert.Equal(t, []string{"chore: local"}, got.Messages)
}

func TestNew(t *testing.T) {
	for _, tt := range []struct {
		s    Settings
		kind ErrorKind
	}{
		{Settings{Provider: "anthropic"}, KindNoKey},
		{Settings{Provider: "openai", Key: "k"}, KindNoModel},
		{Settings{Provider: "openai"}, KindNoKey},
		{Settings{Provider: "ollama"}, KindNoModel},
		{Settings{Provider: "none"}, KindDisabled},
	} {
		_, err := New(tt.s)
		assert.Equal(t, tt.kind, KindOf(err), tt.s.Provider)
	}
	for _, s := range []Settings{{Provider: "anthropic", Key: "k"}, {Provider: "openai", Key: "k", Model: "m"}, {Provider: "ollama", Model: "m"}} {
		p, err := New(s)
		require.NoError(t, err)
		assert.Equal(t, s.Provider, p.Name())
	}
	assert.True(t, NeedsKey("anthropic"))
	assert.False(t, NeedsKey("ollama"))

	for kind := range kindMessages {
		for _, lang := range []i18n.Lang{i18n.TR, i18n.EN} {
			text := (&Error{Kind: kind, Provider: "x"}).Localize(lang)
			assert.NotEqual(t, string(kindMessages[kind]), text, kind)
			assert.NotContains(t, text, "%!", kind)
		}
	}
}
