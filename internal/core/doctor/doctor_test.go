package doctor_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/doctor"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets/secretstest"
)

const (
	testToken = "ghp_supersecrettoken"
	testAIKey = "sk-ant-supersecretkey"
)

type fakeGit struct {
	version     git.Version
	versionErr  error
	identity    git.Identity
	identityErr error
}

func (f fakeGit) Version(context.Context) (git.Version, error) { return f.version, f.versionErr }

func (f fakeGit) Identity(context.Context, string) (git.Identity, error) {
	return f.identity, f.identityErr
}

// healthy describes a machine where every check passes; tests break one thing.
func healthy() doctor.Options {
	store := secretstest.NewMemory()
	_ = store.Set(secrets.GitHubToken, testToken)
	_ = store.Set(secrets.AIKey("anthropic"), testAIKey)
	return doctor.Options{
		Git: fakeGit{
			version:  git.Version{Major: 2, Minor: 54, Patch: 0},
			identity: git.Identity{Name: "Emin", Email: "emin@example.com"},
		},
		Lang: i18n.EN,
		Config: config.Loaded{
			Config:      config.Default(),
			GlobalPath:  "/home/emin/.config/gitpal/config.yaml",
			GlobalFound: true,
		},
		Secrets:  store,
		LookPath: func(string) (string, error) { return "/usr/local/bin/gh", nil },
	}
}

func find(t *testing.T, report doctor.Report, id string) doctor.Check {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("no check %q in %+v", id, report.Checks)
	return doctor.Check{}
}

func ids(report doctor.Report) []string {
	var out []string
	for _, check := range report.Checks {
		out = append(out, check.ID)
	}
	return out
}

func TestHealthySetupPassesEveryCheck(t *testing.T) {
	report := doctor.Run(context.Background(), healthy())

	assert.Equal(t, []string{"git.version", "git.identity", "config", "github.auth", "ai.key"}, ids(report))
	for _, check := range report.Checks {
		assert.Equal(t, doctor.StatusOK, check.Status, "check %s: %s", check.ID, check.Message)
		assert.NotEmpty(t, check.Message)
	}
	assert.True(t, report.Healthy())
	assert.Equal(t, 5, report.Count(doctor.StatusOK))
	assert.Contains(t, find(t, report, "git.version").Message, "2.54.0")
	assert.Contains(t, find(t, report, "git.identity").Message, "Emin <emin@example.com>")
	assert.Contains(t, find(t, report, "config").Message, "/home/emin/.config/gitpal/config.yaml")
}

func TestMissingGitFailsAndSkipsWhatDependsOnIt(t *testing.T) {
	opts := healthy()
	opts.Git = fakeGit{versionErr: &git.Error{Kind: git.KindGitNotFound, ExitCode: -1}}

	report := doctor.Run(context.Background(), opts)

	check := find(t, report, "git.version")
	assert.Equal(t, doctor.StatusFail, check.Status)
	assert.Contains(t, check.Hint, "https://git-scm.com/downloads")
	assert.NotContains(t, ids(report), "git.identity", "identity cannot be read without git")
	assert.False(t, report.Healthy())
}

func TestUnreadableGitVersionFails(t *testing.T) {
	opts := healthy()
	opts.Git = fakeGit{versionErr: errors.New("permission denied")}

	check := find(t, doctor.Run(context.Background(), opts), "git.version")

	assert.Equal(t, doctor.StatusFail, check.Status)
	assert.Contains(t, check.Message, "permission denied")
}

func TestGitOlderThanMinimumFails(t *testing.T) {
	opts := healthy()
	opts.Git = fakeGit{version: git.Version{Major: 2, Minor: 29, Patch: 3}, identity: git.Identity{Name: "Emin", Email: "e@x.io"}}

	report := doctor.Run(context.Background(), opts)

	check := find(t, report, "git.version")
	assert.Equal(t, doctor.StatusFail, check.Status)
	assert.Contains(t, check.Message, "2.29.3")
	assert.Contains(t, check.Message, "2.30.0")
	assert.NotEmpty(t, check.Hint)
	assert.False(t, report.Healthy())
}

func TestOldestSupportedGitPasses(t *testing.T) {
	opts := healthy()
	opts.Git = fakeGit{version: git.Version{Major: 2, Minor: 30, Patch: 0}, identity: git.Identity{Name: "Emin", Email: "e@x.io"}}

	assert.Equal(t, doctor.StatusOK, find(t, doctor.Run(context.Background(), opts), "git.version").Status)
}

func TestIncompleteIdentityFails(t *testing.T) {
	tests := []struct {
		name        string
		identity    git.Identity
		wantMissing []string
		wantPresent []string
	}{
		{name: "no email", identity: git.Identity{Name: "Emin"}, wantMissing: []string{"user.email"}, wantPresent: []string{"user.name"}},
		{name: "no name", identity: git.Identity{Email: "e@x.io"}, wantMissing: []string{"user.name"}, wantPresent: []string{"user.email"}},
		{name: "nothing", identity: git.Identity{}, wantMissing: []string{"user.name", "user.email"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := healthy()
			opts.Git = fakeGit{version: git.Version{Major: 2, Minor: 54}, identity: tt.identity}

			report := doctor.Run(context.Background(), opts)

			check := find(t, report, "git.identity")
			assert.Equal(t, doctor.StatusFail, check.Status)
			for _, key := range tt.wantMissing {
				assert.Contains(t, check.Message, key)
			}
			for _, key := range tt.wantPresent {
				assert.NotContains(t, check.Message, key)
			}
			assert.Contains(t, check.Hint, "git config --global user.name")
			assert.False(t, report.Healthy())
		})
	}
}

func TestUnreadableIdentityFails(t *testing.T) {
	opts := healthy()
	opts.Git = fakeGit{version: git.Version{Major: 2, Minor: 54}, identityErr: errors.New("config locked")}

	check := find(t, doctor.Run(context.Background(), opts), "git.identity")

	assert.Equal(t, doctor.StatusFail, check.Status)
	assert.Contains(t, check.Message, "config locked")
}

func TestConfigCheck(t *testing.T) {
	t.Run("no file means defaults, which is fine", func(t *testing.T) {
		opts := healthy()
		opts.Config.GlobalFound = false

		check := find(t, doctor.Run(context.Background(), opts), "config")

		assert.Equal(t, doctor.StatusOK, check.Status)
		assert.Contains(t, check.Message, "defaults")
		assert.Contains(t, check.Message, "/home/emin/.config/gitpal/config.yaml")
	})

	t.Run("warnings are listed", func(t *testing.T) {
		opts := healthy()
		opts.Config.Warnings = []config.Warning{
			{Code: config.WarnUnknownKey, File: "/home/emin/.config/gitpal/config.yaml", Key: "comit.style"},
			{Code: config.WarnRepoKeyLocked, File: "/repo/.gitpal.yaml", Key: "ai.consent_given"},
		}

		report := doctor.Run(context.Background(), opts)

		check := find(t, report, "config")
		assert.Equal(t, doctor.StatusWarn, check.Status)
		require.Len(t, check.Details, 2)
		assert.Contains(t, check.Details[0], "comit.style")
		assert.Contains(t, check.Details[1], "ai.consent_given")
		assert.True(t, report.Healthy(), "warnings do not make the setup unhealthy")
	})

	t.Run("an unusable file fails and says which file to fix", func(t *testing.T) {
		opts := healthy()
		opts.ConfigErr = &config.FileError{
			Path: "/home/emin/.config/gitpal/config.yaml",
			Err:  &config.ValidationError{Key: "pull.strategy", Value: "yolo", Allowed: []string{"merge", "rebase", "ff-only"}},
		}

		report := doctor.Run(context.Background(), opts)

		check := find(t, report, "config")
		assert.Equal(t, doctor.StatusFail, check.Status)
		text := check.Message + " " + strings.Join(check.Details, " ") + " " + check.Hint
		assert.Contains(t, text, "pull.strategy")
		assert.Contains(t, text, "/home/emin/.config/gitpal/config.yaml")
		assert.False(t, report.Healthy())
	})

	t.Run("a repository file in use is mentioned", func(t *testing.T) {
		opts := healthy()
		opts.Config.RepoFound = true
		opts.Config.RepoPath = "/repo/.gitpal.yaml"

		check := find(t, doctor.Run(context.Background(), opts), "config")

		assert.Contains(t, strings.Join(check.Details, " "), "/repo/.gitpal.yaml")
	})
}

func TestGitHubCheck(t *testing.T) {
	t.Run("device flow without a token is only a warning", func(t *testing.T) {
		opts := healthy()
		opts.Secrets = secretstest.NewMemory()

		report := doctor.Run(context.Background(), opts)

		assert.Equal(t, doctor.StatusWarn, find(t, report, "github.auth").Status)
		assert.True(t, report.Healthy(), "GitHub is optional: the core git workflow must not depend on it")
	})

	t.Run("gh-cli mode with gh installed", func(t *testing.T) {
		opts := healthy()
		opts.Config.Config.GitHub.Auth = config.AuthGHCLI
		opts.Secrets = secretstest.NewMemory()

		check := find(t, doctor.Run(context.Background(), opts), "github.auth")

		assert.Equal(t, doctor.StatusOK, check.Status)
		assert.Contains(t, check.Message, "/usr/local/bin/gh")
	})

	t.Run("gh-cli mode without gh", func(t *testing.T) {
		opts := healthy()
		opts.Config.Config.GitHub.Auth = config.AuthGHCLI
		opts.LookPath = func(string) (string, error) { return "", errors.New("not found") }

		check := find(t, doctor.Run(context.Background(), opts), "github.auth")

		assert.Equal(t, doctor.StatusWarn, check.Status)
		assert.Contains(t, check.Hint, "https://cli.github.com")
	})
}

func TestAICheck(t *testing.T) {
	tests := []struct {
		name       string
		provider   config.AIProvider
		storedKeys map[string]string
		want       doctor.Status
		wantText   string
	}{
		{name: "anthropic with key", provider: config.ProviderAnthropic, storedKeys: map[string]string{"ai-anthropic-api-key": testAIKey}, want: doctor.StatusOK, wantText: "anthropic"},
		{name: "anthropic without key", provider: config.ProviderAnthropic, want: doctor.StatusWarn, wantText: "anthropic"},
		{name: "openai only looks at its own key", provider: config.ProviderOpenAI, storedKeys: map[string]string{"ai-anthropic-api-key": testAIKey}, want: doctor.StatusWarn, wantText: "openai"},
		{name: "ollama needs no key", provider: config.ProviderOllama, want: doctor.StatusOK, wantText: "ollama"},
		{name: "none is a valid choice", provider: config.ProviderNone, want: doctor.StatusOK, wantText: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := healthy()
			opts.Config.Config.AI.Provider = tt.provider
			store := secretstest.NewMemory()
			for name, value := range tt.storedKeys {
				require.NoError(t, store.Set(name, value))
			}
			opts.Secrets = store

			report := doctor.Run(context.Background(), opts)

			check := find(t, report, "ai.key")
			assert.Equal(t, tt.want, check.Status)
			assert.Contains(t, check.Message, tt.wantText)
			assert.True(t, report.Healthy(), "AI is optional: a missing key must never fail the setup")
		})
	}
}

func TestBrokenKeychainIsAWarningNotACrash(t *testing.T) {
	opts := healthy()
	store := secretstest.NewMemory()
	store.Err = errors.New("keychain is locked")
	opts.Secrets = store

	report := doctor.Run(context.Background(), opts)

	for _, id := range []string{"github.auth", "ai.key"} {
		check := find(t, report, id)
		assert.Equal(t, doctor.StatusWarn, check.Status)
		assert.Contains(t, check.Message, "keychain is locked")
	}
	assert.True(t, report.Healthy())
}

// Spec §12: an API key must never show up in output. doctor reads secrets only
// to learn whether they exist.
func TestReportNeverContainsSecretValues(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.TR, i18n.EN} {
		opts := healthy()
		opts.Lang = lang

		report := doctor.Run(context.Background(), opts)

		for _, check := range report.Checks {
			text := check.Message + "\n" + check.Hint + "\n" + strings.Join(check.Details, "\n")
			assert.NotContains(t, text, testToken, "check %s leaks the GitHub token", check.ID)
			assert.NotContains(t, text, testAIKey, "check %s leaks the AI key", check.ID)
		}
	}
}

func TestMessagesFollowTheConfiguredLanguage(t *testing.T) {
	english := healthy()
	english.Secrets = secretstest.NewMemory()
	turkish := healthy()
	turkish.Secrets = secretstest.NewMemory()
	turkish.Lang = i18n.TR

	en := find(t, doctor.Run(context.Background(), english), "github.auth").Message
	tr := find(t, doctor.Run(context.Background(), turkish), "github.auth").Message

	assert.Contains(t, en, "no token")
	assert.Contains(t, tr, "token yok")
}

func TestReportCountsByStatus(t *testing.T) {
	report := doctor.Report{Checks: []doctor.Check{
		{Status: doctor.StatusOK}, {Status: doctor.StatusWarn}, {Status: doctor.StatusWarn}, {Status: doctor.StatusFail},
	}}

	assert.Equal(t, 1, report.Count(doctor.StatusOK))
	assert.Equal(t, 2, report.Count(doctor.StatusWarn))
	assert.Equal(t, 1, report.Count(doctor.StatusFail))
	assert.False(t, report.Healthy())
}
