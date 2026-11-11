package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func globalFile(t *testing.T, content string) string {
	t.Helper()
	return writeFile(t, filepath.Join(t.TempDir(), "config.yaml"), content)
}

func repoWith(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".gitpal.yaml"), content)
	return dir
}

func hasWarning(warnings []config.Warning, code config.WarningCode, key string) bool {
	for _, w := range warnings {
		if w.Code == code && w.Key == key {
			return true
		}
	}
	return false
}

// The defaults are the ones written in spec §10; several of them are safety
// decisions (protected branches, ff-only pulls, no AI consent).
func TestLoadWithoutFilesReturnsSpecDefaults(t *testing.T) {
	got, err := config.Load(config.LoadOptions{})
	require.NoError(t, err)

	want := config.Config{
		Language: i18n.TR,
		Commit: config.CommitConfig{
			Style:            config.StyleConventional,
			MessageLanguage:  i18n.TR,
			Suggestions:      3,
			MaxSubjectLength: 72,
		},
		AI: config.AIConfig{Provider: config.ProviderAnthropic, Model: "", ConsentGiven: false},
		Safety: config.SafetyConfig{
			ProtectedBranches:   []string{"main", "master", "develop"},
			LargeFileMB:         5,
			BackupRetentionDays: 30,
			WarnOnSecrets:       true,
		},
		Pull:   config.PullConfig{Strategy: config.PullFFOnly},
		GitHub: config.GitHubConfig{Auth: config.AuthDeviceFlow},
		UI:     config.UIConfig{Theme: config.ThemeSystem},
	}
	assert.Equal(t, want, got.Config)
	assert.False(t, got.GlobalFound)
	assert.False(t, got.RepoFound)
	assert.Empty(t, got.Warnings)
}

func TestLoadReadsEveryKeyFromGlobalFile(t *testing.T) {
	path := globalFile(t, `
language: en
commit:
  style: free
  message_language: en
  suggestions: 2
  max_subject_length: 50
ai:
  provider: ollama
  model: "llama3"
  consent_given: true
safety:
  protected_branches: [trunk]
  large_file_mb: 10
  backup_retention_days: 7
  warn_on_secrets: false
pull:
  strategy: rebase
github:
  auth: gh-cli
ui:
  theme: dark
workspace:
  repos: [/src/a, /src/b]
`)

	got, err := config.Load(config.LoadOptions{GlobalPath: path})
	require.NoError(t, err)

	want := config.Config{
		Language: i18n.EN,
		Commit: config.CommitConfig{
			Style:            config.StyleFree,
			MessageLanguage:  i18n.EN,
			Suggestions:      2,
			MaxSubjectLength: 50,
		},
		AI: config.AIConfig{Provider: config.ProviderOllama, Model: "llama3", ConsentGiven: true},
		Safety: config.SafetyConfig{
			ProtectedBranches:   []string{"trunk"},
			LargeFileMB:         10,
			BackupRetentionDays: 7,
			WarnOnSecrets:       false,
		},
		Pull:      config.PullConfig{Strategy: config.PullRebase},
		GitHub:    config.GitHubConfig{Auth: config.AuthGHCLI},
		UI:        config.UIConfig{Theme: config.ThemeDark},
		Workspace: config.WorkspaceConfig{Repos: []string{"/src/a", "/src/b"}},
	}
	assert.Equal(t, want, got.Config)
	assert.True(t, got.GlobalFound)
	assert.Equal(t, path, got.GlobalPath)
	assert.Empty(t, got.Warnings)
}

func TestLoadKeepsDefaultsForKeysTheFileOmits(t *testing.T) {
	path := globalFile(t, "pull:\n  strategy: merge\n")

	got, err := config.Load(config.LoadOptions{GlobalPath: path})
	require.NoError(t, err)

	assert.Equal(t, config.PullMerge, got.Config.Pull.Strategy)
	assert.Equal(t, []string{"main", "master", "develop"}, got.Config.Safety.ProtectedBranches)
	assert.Equal(t, 72, got.Config.Commit.MaxSubjectLength)
	assert.True(t, got.Config.Safety.WarnOnSecrets)
}

func TestLoadTreatsMissingGlobalFileAsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	got, err := config.Load(config.LoadOptions{GlobalPath: path})
	require.NoError(t, err)

	assert.False(t, got.GlobalFound)
	assert.Equal(t, path, got.GlobalPath)
	assert.Equal(t, config.Default(), got.Config)
}

func TestLoadNormalizesValues(t *testing.T) {
	path := globalFile(t, `
language: " EN "
pull:
  strategy: Rebase
safety:
  protected_branches: [" main ", "", release, main]
`)

	got, err := config.Load(config.LoadOptions{GlobalPath: path})
	require.NoError(t, err)

	assert.Equal(t, i18n.EN, got.Config.Language)
	assert.Equal(t, config.PullRebase, got.Config.Pull.Strategy)
	assert.Equal(t, []string{"main", "release"}, got.Config.Safety.ProtectedBranches)
}

func TestLoadRejectsInvalidGlobalValues(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantKey string
	}{
		{name: "language", yaml: "language: de", wantKey: "language"},
		{name: "commit style", yaml: "commit: {style: emoji}", wantKey: "commit.style"},
		{name: "message language", yaml: "commit: {message_language: fr}", wantKey: "commit.message_language"},
		{name: "too few suggestions", yaml: "commit: {suggestions: 0}", wantKey: "commit.suggestions"},
		{name: "too many suggestions", yaml: "commit: {suggestions: 4}", wantKey: "commit.suggestions"},
		{name: "subject length too small", yaml: "commit: {max_subject_length: 5}", wantKey: "commit.max_subject_length"},
		{name: "ai provider", yaml: "ai: {provider: gpt}", wantKey: "ai.provider"},
		{name: "large file threshold", yaml: "safety: {large_file_mb: 0}", wantKey: "safety.large_file_mb"},
		{name: "backup retention", yaml: "safety: {backup_retention_days: 0}", wantKey: "safety.backup_retention_days"},
		{name: "pull strategy", yaml: "pull: {strategy: fast-forward}", wantKey: "pull.strategy"},
		{name: "github auth", yaml: "github: {auth: oauth}", wantKey: "github.auth"},
		{name: "theme", yaml: "ui: {theme: neon}", wantKey: "ui.theme"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := globalFile(t, tt.yaml)

			got, err := config.Load(config.LoadOptions{GlobalPath: path})

			var verr *config.ValidationError
			require.ErrorAs(t, err, &verr)
			assert.Equal(t, tt.wantKey, verr.Key)
			var ferr *config.FileError
			require.ErrorAs(t, err, &ferr, "the error must name the file to fix")
			assert.Equal(t, path, ferr.Path)
			assert.Equal(t, config.Default(), got.Config, "a rejected file must leave the defaults in place")
		})
	}
}

func TestLoadRejectsUnparsableGlobalFile(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "malformed yaml", yaml: "commit: [unclosed"},
		{name: "wrong type", yaml: "safety: {large_file_mb: big}"},
		{name: "scalar where a section is expected", yaml: "commit: 5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := globalFile(t, tt.yaml)

			_, err := config.Load(config.LoadOptions{GlobalPath: path})

			var ferr *config.FileError
			require.ErrorAs(t, err, &ferr)
			assert.Equal(t, path, ferr.Path)
		})
	}
}

// A misspelled key must not silently fall back to a default: the user thinks
// "release" is protected while it is not.
func TestLoadWarnsAboutUnknownKeys(t *testing.T) {
	path := globalFile(t, `
comit:
  style: free
safety:
  protected_branchs: [release]
`)

	got, err := config.Load(config.LoadOptions{GlobalPath: path})
	require.NoError(t, err)

	assert.True(t, hasWarning(got.Warnings, config.WarnUnknownKey, "comit.style"), "warnings: %+v", got.Warnings)
	assert.True(t, hasWarning(got.Warnings, config.WarnUnknownKey, "safety.protected_branchs"), "warnings: %+v", got.Warnings)
	assert.Len(t, got.Warnings, 2)
	assert.Equal(t, config.Default(), got.Config)
}

func TestRepoFileOverridesCommitAndPullSettings(t *testing.T) {
	repo := repoWith(t, `
commit:
  style: free
  message_language: en
  suggestions: 1
  max_subject_length: 50
pull:
  strategy: rebase
`)

	got, err := config.Load(config.LoadOptions{RepoDir: repo})
	require.NoError(t, err)

	assert.True(t, got.RepoFound)
	assert.Equal(t, filepath.Join(repo, ".gitpal.yaml"), got.RepoPath)
	assert.Equal(t, config.CommitConfig{
		Style: config.StyleFree, MessageLanguage: i18n.EN, Suggestions: 1, MaxSubjectLength: 50,
	}, got.Config.Commit)
	assert.Equal(t, config.PullRebase, got.Config.Pull.Strategy)
	assert.Empty(t, got.Warnings)
}

func TestRepoFileBeatsGlobalFileForAllowedKeys(t *testing.T) {
	global := globalFile(t, "pull: {strategy: merge}\ncommit: {suggestions: 2}\n")
	repo := repoWith(t, "pull: {strategy: rebase}\n")

	got, err := config.Load(config.LoadOptions{GlobalPath: global, RepoDir: repo})
	require.NoError(t, err)

	assert.Equal(t, config.PullRebase, got.Config.Pull.Strategy)
	assert.Equal(t, 2, got.Config.Commit.Suggestions, "global value stays when the repo file omits the key")
}

// A repository file travels with a clone, so it is written by whoever wrote the
// repository. It may add protected branches but never remove the user's own.
func TestRepoFileCanOnlyAddProtectedBranches(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string
	}{
		{name: "adds new branches", yaml: "safety: {protected_branches: [release, main]}", want: []string{"main", "master", "develop", "release"}},
		{name: "empty list removes nothing", yaml: "safety: {protected_branches: []}", want: []string{"main", "master", "develop"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.Load(config.LoadOptions{RepoDir: repoWith(t, tt.yaml)})
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Config.Safety.ProtectedBranches)
		})
	}
}

// Consent to send code to an AI provider, and the choice of provider, belong to
// the user. A cloned repository must not be able to grant or redirect them.
func TestRepoFileCannotTouchAISettings(t *testing.T) {
	repo := repoWith(t, `
ai:
  provider: openai
  model: gpt-x
  consent_given: true
`)

	got, err := config.Load(config.LoadOptions{RepoDir: repo})
	require.NoError(t, err)

	assert.Equal(t, config.AIConfig{Provider: config.ProviderAnthropic, Model: "", ConsentGiven: false}, got.Config.AI)
	assert.True(t, hasWarning(got.Warnings, config.WarnRepoKeyLocked, "ai.provider"), "warnings: %+v", got.Warnings)
	assert.True(t, hasWarning(got.Warnings, config.WarnRepoKeyLocked, "ai.model"), "warnings: %+v", got.Warnings)
	assert.True(t, hasWarning(got.Warnings, config.WarnRepoKeyLocked, "ai.consent_given"), "warnings: %+v", got.Warnings)
}

func TestRepoFileCannotChangeGlobalOnlySettings(t *testing.T) {
	repo := repoWith(t, "language: en\ngithub: {auth: gh-cli}\nui: {theme: dark}\n")

	got, err := config.Load(config.LoadOptions{RepoDir: repo})
	require.NoError(t, err)

	assert.Equal(t, i18n.TR, got.Config.Language)
	assert.Equal(t, config.AuthDeviceFlow, got.Config.GitHub.Auth)
	assert.Equal(t, config.ThemeSystem, got.Config.UI.Theme)
	assert.True(t, hasWarning(got.Warnings, config.WarnRepoKeyLocked, "language"), "warnings: %+v", got.Warnings)
	assert.True(t, hasWarning(got.Warnings, config.WarnRepoKeyLocked, "github.auth"), "warnings: %+v", got.Warnings)
	assert.True(t, hasWarning(got.Warnings, config.WarnRepoKeyLocked, "ui.theme"), "warnings: %+v", got.Warnings)
}

func TestRepoFileCannotWeakenSafety(t *testing.T) {
	repo := repoWith(t, `
safety:
  warn_on_secrets: false
  large_file_mb: 100
  backup_retention_days: 1
`)

	got, err := config.Load(config.LoadOptions{RepoDir: repo})
	require.NoError(t, err)

	assert.True(t, got.Config.Safety.WarnOnSecrets)
	assert.Equal(t, 5, got.Config.Safety.LargeFileMB)
	assert.Equal(t, 30, got.Config.Safety.BackupRetentionDays)
	assert.True(t, hasWarning(got.Warnings, config.WarnRepoWeaker, "safety.warn_on_secrets"), "warnings: %+v", got.Warnings)
	assert.True(t, hasWarning(got.Warnings, config.WarnRepoWeaker, "safety.large_file_mb"), "warnings: %+v", got.Warnings)
	assert.True(t, hasWarning(got.Warnings, config.WarnRepoWeaker, "safety.backup_retention_days"), "warnings: %+v", got.Warnings)
}

func TestRepoFileCanTightenSafety(t *testing.T) {
	global := globalFile(t, "safety: {warn_on_secrets: false}\n")
	repo := repoWith(t, `
safety:
  warn_on_secrets: true
  large_file_mb: 1
  backup_retention_days: 90
`)

	got, err := config.Load(config.LoadOptions{GlobalPath: global, RepoDir: repo})
	require.NoError(t, err)

	assert.True(t, got.Config.Safety.WarnOnSecrets)
	assert.Equal(t, 1, got.Config.Safety.LargeFileMB)
	assert.Equal(t, 90, got.Config.Safety.BackupRetentionDays)
	assert.Empty(t, got.Warnings)
}

// A broken file in someone else's repository must not stop gitpal from
// working there, and must never loosen what the global config protects.
func TestBrokenRepoFileIsIgnoredWithAWarning(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "malformed yaml", yaml: "commit: [unclosed"},
		{name: "invalid value", yaml: "pull: {strategy: yolo}\nsafety: {protected_branches: [release]}"},
		{name: "wrong type", yaml: "commit: {suggestions: many}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repoWith(t, tt.yaml)

			got, err := config.Load(config.LoadOptions{RepoDir: repo})
			require.NoError(t, err)

			assert.Equal(t, config.Default(), got.Config)
			require.Len(t, got.Warnings, 1)
			assert.Equal(t, config.WarnRepoFileIgnored, got.Warnings[0].Code)
			assert.Equal(t, filepath.Join(repo, ".gitpal.yaml"), got.Warnings[0].File)
			assert.Error(t, got.Warnings[0].Err)
		})
	}
}

func TestRepoDirWithoutFileIsFine(t *testing.T) {
	got, err := config.Load(config.LoadOptions{RepoDir: t.TempDir()})
	require.NoError(t, err)

	assert.False(t, got.RepoFound)
	assert.Equal(t, config.Default(), got.Config)
	assert.Empty(t, got.Warnings)
}

func TestErrorsAndWarningsDescribeThemselvesInBothLanguages(t *testing.T) {
	path := globalFile(t, "pull: {strategy: fast-forward}")
	_, err := config.Load(config.LoadOptions{GlobalPath: path})
	var loc i18n.Localizable
	require.ErrorAs(t, err, &loc)

	for _, lang := range []i18n.Lang{i18n.TR, i18n.EN} {
		text := loc.Localize(lang)
		assert.Contains(t, text, path, "must say which file to fix")
		assert.Contains(t, text, "pull.strategy")
		assert.Contains(t, text, "fast-forward")
		assert.Contains(t, text, "ff-only", "must list the allowed values")
	}
	assert.NotEqual(t, loc.Localize(i18n.TR), loc.Localize(i18n.EN))

	warning := config.Warning{Code: config.WarnUnknownKey, File: "/x/config.yaml", Key: "comit.style"}
	for _, lang := range []i18n.Lang{i18n.TR, i18n.EN} {
		assert.Contains(t, warning.Localize(lang), "/x/config.yaml")
		assert.Contains(t, warning.Localize(lang), "comit.style")
	}
}

func TestRangeErrorNamesTheBounds(t *testing.T) {
	_, err := config.Load(config.LoadOptions{GlobalPath: globalFile(t, "commit: {suggestions: 9}")})

	var verr *config.ValidationError
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Localize(i18n.EN), "between 1 and 3")
	assert.Contains(t, verr.Localize(i18n.EN), "9")
}
