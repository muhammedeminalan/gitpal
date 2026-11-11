package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/ai"
	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
)

// recentForStyle is how many earlier messages the model sees (spec §6.2).
const recentForStyle = 10

// keyEnv is the environment variable read when the keychain has no key, for
// machines without a keychain (CI, containers).
var keyEnv = map[string]string{"anthropic": "ANTHROPIC_API_KEY", "openai": "OPENAI_API_KEY"}

// aiKey returns the provider's API key: keychain first, then environment.
func (a *app) aiKey(provider string) string {
	if a.env.Secrets != nil {
		if key, err := a.env.Secrets.Get(secrets.AIKey(provider)); err == nil && key != "" {
			return key
		}
	}
	if name := keyEnv[provider]; name != "" {
		return a.env.Getenv(name)
	}
	return ""
}

// aiByDefault reports whether `commit` without -m asks the AI first: only for
// a person at the keyboard, with a provider set up and agreed to. Anyone
// else types the message, as before.
func (a *app) aiByDefault() bool {
	c, err := a.cfg()
	if err != nil || !a.interactive() || c.AI.Provider == config.ProviderNone || !c.AI.ConsentGiven {
		return false
	}
	p := string(c.AI.Provider)
	return !ai.NeedsKey(p) || a.aiKey(p) != ""
}

// aiProvider builds the configured provider.
func (a *app) aiProvider() (ai.Provider, config.Config, error) {
	c, err := a.cfg()
	if err != nil {
		return nil, c, err
	}
	p := string(c.AI.Provider)
	provider, err := a.env.NewAI(ai.Settings{
		Provider: p, Model: c.AI.Model, Key: a.aiKey(p), BaseURL: a.env.Getenv("GITPAL_AI_BASE_URL"),
	})
	return provider, c, err
}

// aiConsent makes sure the user agreed to sending the diff (spec §6.4) and
// remembers the answer.
func (a *app) aiConsent(c config.Config, provider string) (bool, error) {
	if c.AI.ConsentGiven {
		return true, nil
	}
	if !a.interactive() {
		return false, fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgAIConsentNeeded))
	}
	question := i18n.T(a.lang, i18n.MsgAIConsent, provider)
	if provider == "ollama" {
		where := a.env.Getenv("GITPAL_AI_BASE_URL")
		if where == "" {
			where = ai.DefaultOllamaURL
		}
		question = i18n.T(a.lang, i18n.MsgAIConsentLocal, where)
	}
	line, err := a.readLine(question)
	if err != nil {
		return false, err
	}
	if !a.yes(line, false) {
		fmt.Fprintln(a.env.Stderr, i18n.T(a.lang, i18n.MsgAIConsentDenied))
		return false, nil
	}
	path, err := config.GlobalPath(a.cfgEnv)
	if err != nil {
		return false, err
	}
	return true, config.SetGlobal(path, "ai.consent_given", "true")
}

// aiMessage asks the AI for a commit message. An empty message with no error
// means "write it yourself": the AI never blocks a commit (spec §6.5).
// explicit is --ai; without it, failures are only mentioned.
func (a *app) aiMessage(ctx context.Context, explicit bool) (string, error) {
	provider, c, err := a.aiProvider()
	if err != nil {
		return a.aiFallback(err, explicit)
	}
	if ok, err := a.aiConsent(c, provider.Name()); err != nil || !ok {
		return "", err
	}

	runner, root, err := a.git()
	if err != nil {
		return "", err
	}
	files, err := runner.Diff(ctx, root, git.DiffOptions{Staged: true})
	if err != nil || len(files) == 0 {
		// Nothing staged: the commit itself explains that.
		return "", err
	}
	recent, err := runner.RecentMessages(ctx, root, recentForStyle)
	if err != nil {
		return "", err
	}
	branch, err := runner.CurrentBranch(ctx, root)
	if err != nil {
		return "", err
	}
	in := ai.CommitInput{
		Payload: ai.Prepare(files, 0), RecentMessages: recent, Branch: branch,
		Style: ai.Style(c.Commit.Style), Language: c.Commit.MessageLanguage,
		Suggestions: c.Commit.Suggestions, MaxSubjectLength: c.Commit.MaxSubjectLength,
	}

	st := a.styleFor(a.env.Stderr)
	note := func(text string) { fmt.Fprintln(a.env.Stderr, st.dim(text)) }
	if len(in.Payload.Withheld) > 0 {
		note(i18n.T(a.lang, i18n.MsgAIWithheld, strings.Join(in.Payload.Withheld, ", ")))
	}
	if in.Payload.Truncated {
		note(i18n.T(a.lang, i18n.MsgAITruncated))
	}
	for {
		note(i18n.T(a.lang, i18n.MsgAIWorking, provider.Name()))
		note(i18n.T(a.lang, i18n.MsgAISent, in.Payload.Chars(), in.Payload.ApproxTokens()))
		got, err := provider.GenerateCommitMessages(ctx, in)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return "", err
			}
			return a.aiFallback(err, explicit)
		}
		if !a.interactive() {
			return got.Messages[0], nil
		}
		choice, err := a.chooseSuggestion(got)
		if err != nil || choice != "\x00regenerate" {
			return choice, err
		}
	}
}

// chooseSuggestion shows the suggestions and returns the chosen message, ""
// to write one by hand, or "\x00regenerate".
func (a *app) chooseSuggestion(got ai.CommitSuggestions) (string, error) {
	w := a.env.Stderr
	st := a.styleFor(w)
	fmt.Fprintln(w, st.bold(i18n.T(a.lang, i18n.MsgAISuggestions)))
	for i, m := range got.Messages {
		subject, body, _ := strings.Cut(m, "\n")
		fmt.Fprintf(w, "  %s %s\n", st.yellow(strconv.Itoa(i+1)+")"), subject)
		if body = strings.TrimSpace(body); body != "" {
			for line := range strings.SplitSeq(body, "\n") {
				fmt.Fprintf(w, "     %s\n", st.dim(line))
			}
		}
	}
	if got.Split != "" {
		fmt.Fprintln(w, st.yellow(i18n.T(a.lang, i18n.MsgAISplit, got.Split)))
	}
	for {
		line, err := a.readLine(i18n.T(a.lang, i18n.MsgAIChoose))
		if err != nil {
			return "", err
		}
		line = strings.ToLower(strings.TrimSpace(line))
		if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(got.Messages) {
			return got.Messages[n-1], nil
		}
		switch line {
		case "", "1":
			return got.Messages[0], nil
		case "r", "y":
			return "\x00regenerate", nil
		case "m":
			return "", nil
		}
	}
}

// aiFallback reports an AI failure. Asked for explicitly by a script, it is
// an error; otherwise the user writes the message instead.
func (a *app) aiFallback(err error, explicit bool) (string, error) {
	if explicit && !a.interactive() {
		return "", err
	}
	if explicit || ai.KindOf(err) != ai.KindDisabled {
		fmt.Fprintln(a.env.Stderr, a.styleFor(a.env.Stderr).yellow(i18n.Describe(a.lang, err)))
		fmt.Fprintln(a.env.Stderr, i18n.T(a.lang, i18n.MsgAIFallbackManual))
	}
	return "", nil
}

func (a *app) newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: i18n.T(a.lang, i18n.MsgAuthShort)}
	aiCmd := &cobra.Command{
		Use: "ai", Short: i18n.T(a.lang, i18n.MsgAuthAIShort), Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			c, err := a.cfg()
			if err != nil {
				return err
			}
			provider := string(c.AI.Provider)
			if !ai.NeedsKey(provider) {
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgAuthNotNeeded, provider))
				return nil
			}
			key, err := a.readSecret(i18n.T(a.lang, i18n.MsgAuthPrompt, provider))
			if err != nil {
				return err
			}
			if err := a.env.Secrets.Set(secrets.AIKey(provider), key); err != nil {
				return err
			}
			fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgAuthSaved))
			return nil
		},
	}
	logout := &cobra.Command{
		Use: "logout <ai|github>", Short: i18n.T(a.lang, i18n.MsgAuthLogoutShort), Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := a.cfg()
			if err != nil {
				return err
			}
			name := secrets.GitHubToken
			if args[0] == "ai" {
				name = secrets.AIKey(string(c.AI.Provider))
			}
			if err := a.env.Secrets.Delete(name); err != nil {
				return err
			}
			fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgAuthRemoved))
			return nil
		},
	}
	cmd.AddCommand(aiCmd, logout)
	return cmd
}

// readSecret reads a key without echoing it on a terminal, or a plain line
// from a pipe (`echo $KEY | gitpal auth ai`). The value is never printed or
// logged.
func (a *app) readSecret(prompt string) (string, error) {
	fmt.Fprint(a.env.Stderr, prompt)
	var key string
	var err error
	if a.env.ReadSecret != nil {
		key, err = a.env.ReadSecret()
		fmt.Fprintln(a.env.Stderr)
	}
	if a.env.ReadSecret == nil || errors.Is(err, errNotTerminal) {
		key, err = a.readLine("")
	}
	if err != nil {
		return "", err
	}
	if key = strings.TrimSpace(key); key == "" {
		return "", fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgAuthEmpty))
	}
	return key, nil
}
