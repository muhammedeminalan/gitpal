package cli_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/cli"
	"github.com/muhammedeminalan/gitpal/internal/core/ai"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
)

// fakeAI records what it was given and answers with fixed suggestions.
type fakeAI struct {
	inputs  []ai.CommitInput
	answers []ai.CommitSuggestions
	err     error
}

func (f *fakeAI) Name() string { return "fake" }

func (f *fakeAI) GenerateCommitMessages(_ context.Context, in ai.CommitInput) (ai.CommitSuggestions, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return ai.CommitSuggestions{}, f.err
	}
	answer := f.answers[0]
	if len(f.answers) > 1 {
		f.answers = f.answers[1:]
	}
	return answer, nil
}

func (f *fakeAI) GeneratePRDescription(context.Context, ai.PRInput) (ai.PRText, error) {
	return ai.PRText{}, nil
}

func (h *harness) useAI(f *fakeAI) *ai.Settings {
	var got ai.Settings
	h.env.NewAI = func(s ai.Settings) (ai.Provider, error) {
		got = s
		if s.Key == "" && ai.NeedsKey(s.Provider) {
			return nil, &ai.Error{Kind: ai.KindNoKey, Provider: s.Provider}
		}
		return f, nil
	}
	return &got
}

func TestCommitWithAI(t *testing.T) {
	h, repo := inRepo(t)
	h.writeConfig("language: en\nai:\n  provider: anthropic\n")
	require.NoError(t, h.secrets.Set(secrets.AIKey("anthropic"), "sk-from-keychain"))
	fake := &fakeAI{answers: []ai.CommitSuggestions{
		{Messages: []string{"feat: first idea", "feat: second idea\n\nbecause"}, Split: "a.txt and .env are unrelated"},
		{Messages: []string{"feat: regenerated"}},
	}}
	settings := h.useAI(fake)
	repo.WriteFile("a.txt", "two\n")
	repo.WriteFile(".env", "PASSWORD=from-the-env-file\n")
	repo.Git("add", "a.txt", ".env")

	// Agree, regenerate, then take the first.
	h.answer("y", "r", "1", "y")

	code := h.run("commit", "--ai")

	require.Equal(t, cli.ExitOK, code, h.stderr.String())
	assert.Equal(t, "feat: regenerated", repo.Git("log", "-1", "--format=%s"))
	assert.Equal(t, "sk-from-keychain", settings.Key)
	require.Len(t, fake.inputs, 2)
	in := fake.inputs[0]
	assert.NotContains(t, in.Payload.Text, "from-the-env-file", ".env content never reaches the provider")
	assert.Equal(t, []string{".env"}, in.Payload.Withheld)
	assert.Equal(t, "work", in.Branch)
	assert.Equal(t, []string{"first"}, in.RecentMessages)
	assert.Equal(t, 3, in.Suggestions)

	errOut := h.stderr.String()
	assert.Contains(t, errOut, "Not sent (may contain secrets): .env")
	assert.Contains(t, errOut, "2) feat: second idea")
	assert.Contains(t, errOut, "could be split")
	assert.Contains(t, errOut, "characters (~")

	cfg, err := os.ReadFile(h.vars["GITPAL_CONFIG"])
	require.NoError(t, err)
	assert.Contains(t, string(cfg), "consent_given: true", "the consent is remembered")

	t.Run("plain commit asks the AI by default once agreed", func(t *testing.T) {
		repo.WriteFile("a.txt", "three\n")
		repo.Git("add", "a.txt")
		fake.answers = []ai.CommitSuggestions{{Messages: []string{"fix: by default"}}}
		h.answer("1")
		require.Equal(t, cli.ExitOK, h.run("commit"), h.stderr.String())
		assert.Equal(t, "fix: by default", repo.Git("log", "-1", "--format=%s"))
	})

	t.Run("m writes it by hand", func(t *testing.T) {
		repo.WriteFile("a.txt", "four\n")
		repo.Git("add", "a.txt")
		h.answer("m", "my own words")
		require.Equal(t, cli.ExitOK, h.run("commit"))
		assert.Equal(t, "my own words", repo.Git("log", "-1", "--format=%s"))
	})
}

func TestAIConsentDenied(t *testing.T) {
	h, repo := inRepo(t)
	h.writeConfig("language: en\nai:\n  provider: ollama\n  model: llama\n")
	fake := &fakeAI{answers: []ai.CommitSuggestions{{Messages: []string{"never used"}}}}
	h.useAI(fake)
	repo.WriteFile("a.txt", "two\n")
	repo.Git("add", "a.txt")

	h.answer("n", "typed instead")
	require.Equal(t, cli.ExitOK, h.run("commit", "--ai"))

	assert.Empty(t, fake.inputs, "nothing was sent")
	assert.Contains(t, h.stderr.String(), "not sent anywhere")
	assert.Equal(t, "typed instead", repo.Git("log", "-1", "--format=%s"))
}

func TestAIFailureFallsBackToTyping(t *testing.T) {
	h, repo := inRepo(t)
	h.writeConfig("language: en\nai:\n  provider: anthropic\n  consent_given: true\n")
	h.vars["ANTHROPIC_API_KEY"] = "sk-from-env"
	h.useAI(&fakeAI{err: &ai.Error{Kind: ai.KindNetwork, Provider: "anthropic"}})
	repo.WriteFile("a.txt", "two\n")
	repo.Git("add", "a.txt")

	h.answer("by hand")
	require.Equal(t, cli.ExitOK, h.run("commit", "--ai"))
	assert.Contains(t, h.stderr.String(), "Could not reach anthropic")
	assert.Equal(t, "by hand", repo.Git("log", "-1", "--format=%s"))

	t.Run("a script asking for --ai gets the error", func(t *testing.T) {
		repo.WriteFile("a.txt", "three\n")
		repo.Git("add", "a.txt")
		h.interactive = false
		assert.Equal(t, cli.ExitError, h.run("commit", "--ai"))
	})
}

func TestAIWithoutConsentInAScript(t *testing.T) {
	h, repo := inRepo(t)
	h.writeConfig("language: en\nai:\n  provider: anthropic\n")
	h.vars["ANTHROPIC_API_KEY"] = "k"
	fake := &fakeAI{answers: []ai.CommitSuggestions{{Messages: []string{"x"}}}}
	h.useAI(fake)
	repo.WriteFile("a.txt", "two\n")
	repo.Git("add", "a.txt")

	assert.Equal(t, cli.ExitError, h.run("commit", "--ai"))
	assert.Contains(t, h.stderr.String(), "consent_given")
	assert.Empty(t, fake.inputs)
}

func TestAIScriptTakesTheFirstSuggestion(t *testing.T) {
	h, repo := inRepo(t)
	h.writeConfig("language: en\nai:\n  provider: anthropic\n  consent_given: true\n")
	h.vars["ANTHROPIC_API_KEY"] = "k"
	h.useAI(&fakeAI{answers: []ai.CommitSuggestions{{Messages: []string{"feat: scripted", "feat: other"}}}})
	repo.WriteFile("a.txt", "two\n")

	require.Equal(t, cli.ExitOK, h.run("save", "--all", "--ai", "--no-push"), h.stderr.String())
	assert.Equal(t, "feat: scripted", repo.Git("log", "-1", "--format=%s"))
}

func TestAuthAI(t *testing.T) {
	h := newHarness(t)
	h.writeConfig("language: en\nai:\n  provider: anthropic\n")
	h.env.Stdin = strings.NewReader("sk-secret-value\n")

	require.Equal(t, cli.ExitOK, h.run("auth", "ai"))
	got, err := h.secrets.Get(secrets.AIKey("anthropic"))
	require.NoError(t, err)
	assert.Equal(t, "sk-secret-value", got)
	assert.NotContains(t, h.stdout.String()+h.stderr.String(), "sk-secret-value")

	h.env.Stdin = strings.NewReader("\n")
	assert.Equal(t, cli.ExitError, h.run("auth", "ai"))

	require.Equal(t, cli.ExitOK, h.run("auth", "logout", "ai"))
	_, err = h.secrets.Get(secrets.AIKey("anthropic"))
	assert.Error(t, err)

	h.writeConfig("language: en\nai:\n  provider: ollama\n")
	require.Equal(t, cli.ExitOK, h.run("auth", "ai"))
	assert.Contains(t, h.stdout.String(), "needs no key")
}
