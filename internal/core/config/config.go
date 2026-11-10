// Package config loads gitpal's settings (spec §10).
//
// Settings come from three layers: built-in defaults, the user's global file
// and an optional per-repository file. The repository file is treated as
// untrusted input, because it arrives with a clone: it can tighten safety and
// adjust commit conventions, but it can never loosen protection or decide
// where the user's code is sent. See Load.
package config

import (
	"slices"
	"strconv"
	"strings"

	"github.com/eminalan/gitpal/internal/core/i18n"
)

// RepoFileName is the per-repository override file, read from the repository root.
const RepoFileName = ".gitpal.yaml"

// CommitStyle selects how commit messages are shaped.
type CommitStyle string

// Commit message styles.
const (
	StyleConventional CommitStyle = "conventional"
	StyleFree         CommitStyle = "free"
)

// AIProvider selects who generates commit messages.
type AIProvider string

// AI providers. ProviderNone turns AI features off.
const (
	ProviderAnthropic AIProvider = "anthropic"
	ProviderOpenAI    AIProvider = "openai"
	ProviderOllama    AIProvider = "ollama"
	ProviderNone      AIProvider = "none"
)

// PullStrategy selects how `pull` integrates remote commits.
type PullStrategy string

// Pull strategies.
const (
	PullMerge  PullStrategy = "merge"
	PullRebase PullStrategy = "rebase"
	PullFFOnly PullStrategy = "ff-only"
)

// GitHubAuth selects where the GitHub token comes from.
type GitHubAuth string

// GitHub authentication methods.
const (
	AuthDeviceFlow GitHubAuth = "device-flow"
	AuthGHCLI      GitHubAuth = "gh-cli"
)

// Theme selects the GUI color scheme.
type Theme string

// GUI themes.
const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

// Limits for numeric settings.
const (
	minSuggestions, maxSuggestions     = 1, 3 // spec §6.1: "1–3 mesaj önerisi"
	minSubjectLength, maxSubjectLength = 20, 200
	minLargeFileMB, maxLargeFileMB     = 1, 10240
	minRetentionDays, maxRetentionDays = 1, 3650
)

// Config is the complete set of gitpal settings.
type Config struct {
	Language i18n.Lang    `mapstructure:"language" json:"language"`
	Commit   CommitConfig `mapstructure:"commit" json:"commit"`
	AI       AIConfig     `mapstructure:"ai" json:"ai"`
	Safety   SafetyConfig `mapstructure:"safety" json:"safety"`
	Pull     PullConfig   `mapstructure:"pull" json:"pull"`
	GitHub   GitHubConfig `mapstructure:"github" json:"github"`
	UI       UIConfig     `mapstructure:"ui" json:"ui"`
}

// CommitConfig shapes commit messages.
type CommitConfig struct {
	Style            CommitStyle `mapstructure:"style" json:"style"`
	MessageLanguage  i18n.Lang   `mapstructure:"message_language" json:"message_language"`
	Suggestions      int         `mapstructure:"suggestions" json:"suggestions"`
	MaxSubjectLength int         `mapstructure:"max_subject_length" json:"max_subject_length"`
}

// AIConfig controls AI-generated text. API keys are never stored here; they
// live in the OS keychain.
type AIConfig struct {
	Provider     AIProvider `mapstructure:"provider" json:"provider"`
	Model        string     `mapstructure:"model" json:"model"`
	ConsentGiven bool       `mapstructure:"consent_given" json:"consent_given"`
}

// SafetyConfig tunes the safety layer.
type SafetyConfig struct {
	ProtectedBranches   []string `mapstructure:"protected_branches" json:"protected_branches"`
	LargeFileMB         int      `mapstructure:"large_file_mb" json:"large_file_mb"`
	BackupRetentionDays int      `mapstructure:"backup_retention_days" json:"backup_retention_days"`
	WarnOnSecrets       bool     `mapstructure:"warn_on_secrets" json:"warn_on_secrets"`
}

// PullConfig controls `pull`.
type PullConfig struct {
	Strategy PullStrategy `mapstructure:"strategy" json:"strategy"`
}

// GitHubConfig controls GitHub integration. Tokens are never stored here.
type GitHubConfig struct {
	Auth GitHubAuth `mapstructure:"auth" json:"auth"`
}

// UIConfig controls the GUI.
type UIConfig struct {
	Theme Theme `mapstructure:"theme" json:"theme"`
}

// Default returns the built-in settings (spec §10).
func Default() Config {
	return Config{
		Language: i18n.TR,
		Commit: CommitConfig{
			Style:            StyleConventional,
			MessageLanguage:  i18n.TR,
			Suggestions:      3,
			MaxSubjectLength: 72,
		},
		AI: AIConfig{Provider: ProviderAnthropic},
		Safety: SafetyConfig{
			ProtectedBranches:   []string{"main", "master", "develop"},
			LargeFileMB:         5,
			BackupRetentionDays: 30,
			WarnOnSecrets:       true,
		},
		Pull:   PullConfig{Strategy: PullFFOnly},
		GitHub: GitHubConfig{Auth: AuthDeviceFlow},
		UI:     UIConfig{Theme: ThemeSystem},
	}
}

// Validate reports the first setting whose value gitpal cannot work with, as a
// *ValidationError.
func (c Config) Validate() error {
	checks := []error{
		oneOf("language", string(c.Language), string(i18n.TR), string(i18n.EN)),
		oneOf("commit.style", string(c.Commit.Style), string(StyleConventional), string(StyleFree)),
		oneOf("commit.message_language", string(c.Commit.MessageLanguage), string(i18n.TR), string(i18n.EN)),
		inRange("commit.suggestions", c.Commit.Suggestions, minSuggestions, maxSuggestions),
		inRange("commit.max_subject_length", c.Commit.MaxSubjectLength, minSubjectLength, maxSubjectLength),
		oneOf("ai.provider", string(c.AI.Provider),
			string(ProviderAnthropic), string(ProviderOpenAI), string(ProviderOllama), string(ProviderNone)),
		inRange("safety.large_file_mb", c.Safety.LargeFileMB, minLargeFileMB, maxLargeFileMB),
		inRange("safety.backup_retention_days", c.Safety.BackupRetentionDays, minRetentionDays, maxRetentionDays),
		oneOf("pull.strategy", string(c.Pull.Strategy), string(PullMerge), string(PullRebase), string(PullFFOnly)),
		oneOf("github.auth", string(c.GitHub.Auth), string(AuthDeviceFlow), string(AuthGHCLI)),
		oneOf("ui.theme", string(c.UI.Theme), string(ThemeSystem), string(ThemeLight), string(ThemeDark)),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}

func oneOf(key, value string, allowed ...string) error {
	if slices.Contains(allowed, value) {
		return nil
	}
	return &ValidationError{Key: key, Value: value, Allowed: allowed}
}

func inRange(key string, value, lo, hi int) error {
	if value >= lo && value <= hi {
		return nil
	}
	return &ValidationError{Key: key, Value: strconv.Itoa(value), Min: lo, Max: hi, number: value}
}

// normalize makes values comparable: choices are trimmed and lower-cased, and
// the protected branch list loses blanks and duplicates.
func (c *Config) normalize() {
	c.Language = i18n.Lang(choice(string(c.Language)))
	c.Commit.Style = CommitStyle(choice(string(c.Commit.Style)))
	c.Commit.MessageLanguage = i18n.Lang(choice(string(c.Commit.MessageLanguage)))
	c.AI.Provider = AIProvider(choice(string(c.AI.Provider)))
	c.AI.Model = strings.TrimSpace(c.AI.Model)
	c.Pull.Strategy = PullStrategy(choice(string(c.Pull.Strategy)))
	c.GitHub.Auth = GitHubAuth(choice(string(c.GitHub.Auth)))
	c.UI.Theme = Theme(choice(string(c.UI.Theme)))
	c.Safety.ProtectedBranches = branchList(c.Safety.ProtectedBranches)
}

func choice(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// branchList returns names trimmed, without blanks or duplicates, in first-seen
// order. Branch names are case-sensitive, so their case is kept.
func branchList(lists ...[]string) []string {
	out := []string{}
	for _, list := range lists {
		for _, name := range list {
			name = strings.TrimSpace(name)
			if name == "" || slices.Contains(out, name) {
				continue
			}
			out = append(out, name)
		}
	}
	return out
}
