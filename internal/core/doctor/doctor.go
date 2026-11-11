// Package doctor checks whether the machine is ready for gitpal: git itself,
// the commit identity, the settings, and the optional GitHub and AI
// credentials (spec §8, `gitpal doctor`).
//
// Only git and the identity can fail the check-up. GitHub and AI are optional
// by design (spec §1: the core git workflow works without them), so a missing
// token or key is a warning.
package doctor

import (
	"context"
	"errors"
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
)

// Status is the outcome of one check.
type Status string

// Check outcomes.
const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check IDs, stable for scripts reading `doctor --json`.
const (
	CheckGitVersion  = "git.version"
	CheckGitIdentity = "git.identity"
	CheckConfig      = "config"
	CheckGitHubAuth  = "github.auth"
	CheckAIKey       = "ai.key"
)

// Check is the result of one check, already worded for the user.
type Check struct {
	ID      string   `json:"id"`
	Status  Status   `json:"status"`
	Message string   `json:"message"`
	Hint    string   `json:"hint,omitempty"`
	Details []string `json:"details,omitempty"`
}

// Report is the result of a full check-up.
type Report struct {
	Checks []Check `json:"checks"`
}

// Healthy reports whether nothing failed. Warnings do not count.
func (r Report) Healthy() bool { return r.Count(StatusFail) == 0 }

// Count returns how many checks ended with status.
func (r Report) Count(status Status) int {
	n := 0
	for _, check := range r.Checks {
		if check.Status == status {
			n++
		}
	}
	return n
}

// GitProbe is the part of git.Runner the check-up needs.
type GitProbe interface {
	Version(ctx context.Context) (git.Version, error)
	Identity(ctx context.Context, dir string) (git.Identity, error)
}

// Options are the things Run inspects.
type Options struct {
	Git GitProbe
	// Dir is where the identity is read from; a repository there can
	// override the global identity.
	Dir string
	// Lang is the language of the messages.
	Lang i18n.Lang
	// Config and ConfigErr are the outcome of config.Load.
	Config    config.Loaded
	ConfigErr error
	// Secrets is the keychain. Values are only tested for existence.
	Secrets secrets.Store
	// LookPath finds an executable on PATH (exec.LookPath).
	LookPath func(file string) (string, error)
}

// Run performs every check. It does not stop at the first problem: the point
// is to show everything that needs attention at once.
func Run(ctx context.Context, opts Options) Report {
	d := doctor{opts: opts, loc: i18n.For(opts.Lang)}

	var report Report
	gitCheck, gitUsable := d.gitVersion(ctx)
	report.Checks = append(report.Checks, gitCheck)
	if gitUsable {
		report.Checks = append(report.Checks, d.identity(ctx))
	}
	report.Checks = append(report.Checks, d.config(), d.github(), d.ai())
	return report
}

type doctor struct {
	opts Options
	loc  i18n.Localizer
}

// gitVersion also reports whether git can be run at all, which later checks
// depend on.
func (d doctor) gitVersion(ctx context.Context) (check Check, usable bool) {
	check = Check{ID: CheckGitVersion, Status: StatusFail}
	version, err := d.opts.Git.Version(ctx)
	switch {
	case git.KindOf(err) == git.KindGitNotFound:
		check.Message = d.loc.T(i18n.MsgDoctorGitMissing)
		check.Hint = d.loc.T(i18n.MsgDoctorGitMissingHint)
		return check, false
	case err != nil:
		check.Message = d.loc.T(i18n.MsgDoctorGitError, i18n.Describe(d.opts.Lang, err))
		return check, false
	case !version.AtLeast(git.MinVersion):
		check.Message = d.loc.T(i18n.MsgDoctorGitTooOld, version, git.MinVersion)
		check.Hint = d.loc.T(i18n.MsgDoctorGitTooOldHint)
		return check, true
	}
	check.Status = StatusOK
	check.Message = d.loc.T(i18n.MsgDoctorGitOK, version, git.MinVersion)
	return check, true
}

func (d doctor) identity(ctx context.Context) Check {
	check := Check{ID: CheckGitIdentity, Status: StatusFail}
	id, err := d.opts.Git.Identity(ctx, d.opts.Dir)
	if err != nil {
		check.Message = d.loc.T(i18n.MsgDoctorIdentityError, i18n.Describe(d.opts.Lang, err))
		return check
	}
	if !id.Complete() {
		var missing []string
		if id.Name == "" {
			missing = append(missing, "user.name")
		}
		if id.Email == "" {
			missing = append(missing, "user.email")
		}
		check.Message = d.loc.T(i18n.MsgDoctorIdentityMissing, strings.Join(missing, ", "))
		check.Hint = d.loc.T(i18n.MsgDoctorIdentityMissingHint)
		return check
	}
	check.Status = StatusOK
	check.Message = d.loc.T(i18n.MsgDoctorIdentityOK, id.Name, id.Email)
	return check
}

func (d doctor) config() Check {
	check := Check{ID: CheckConfig, Status: StatusOK}
	loaded := d.opts.Config

	if err := d.opts.ConfigErr; err != nil {
		check.Status = StatusFail
		check.Message = d.loc.T(i18n.MsgDoctorConfigInvalid)
		check.Details = []string{i18n.Describe(d.opts.Lang, err)}
		var fileErr *config.FileError
		if errors.As(err, &fileErr) {
			check.Hint = d.loc.T(i18n.MsgDoctorConfigHint, fileErr.Path)
		}
		return check
	}

	if loaded.GlobalFound {
		check.Message = d.loc.T(i18n.MsgDoctorConfigOK, loaded.GlobalPath)
	} else {
		check.Message = d.loc.T(i18n.MsgDoctorConfigDefaults, loaded.GlobalPath)
	}
	if loaded.RepoFound {
		check.Details = append(check.Details, d.loc.T(i18n.MsgConfigRepoFileLocation, loaded.RepoPath))
	}
	if len(loaded.Warnings) > 0 {
		check.Status = StatusWarn
		check.Message = d.loc.T(i18n.MsgDoctorConfigWarnings, loaded.GlobalPath, len(loaded.Warnings))
		for _, warning := range loaded.Warnings {
			check.Details = append(check.Details, warning.Localize(d.opts.Lang))
		}
	}
	return check
}

func (d doctor) github() Check {
	check := Check{ID: CheckGitHubAuth, Status: StatusWarn}

	if d.opts.Config.Config.GitHub.Auth == config.AuthGHCLI {
		path, err := d.opts.LookPath("gh")
		if err != nil {
			check.Message = d.loc.T(i18n.MsgDoctorGitHubCLIMissing)
			check.Hint = d.loc.T(i18n.MsgDoctorGitHubCLIHint)
			return check
		}
		check.Status = StatusOK
		check.Message = d.loc.T(i18n.MsgDoctorGitHubCLIOK, path)
		return check
	}

	switch found, err := d.hasSecret(secrets.GitHubToken); {
	case err != nil:
		check.Message = d.loc.T(i18n.MsgDoctorKeychainError, err)
	case !found:
		check.Message = d.loc.T(i18n.MsgDoctorGitHubTokenMissing)
	default:
		check.Status = StatusOK
		check.Message = d.loc.T(i18n.MsgDoctorGitHubTokenOK)
	}
	return check
}

func (d doctor) ai() Check {
	check := Check{ID: CheckAIKey, Status: StatusOK}
	provider := d.opts.Config.Config.AI.Provider

	switch provider {
	case config.ProviderNone:
		check.Message = d.loc.T(i18n.MsgDoctorAIOff)
		return check
	case config.ProviderOllama:
		check.Message = d.loc.T(i18n.MsgDoctorAILocal, provider)
		return check
	}

	switch found, err := d.hasSecret(secrets.AIKey(string(provider))); {
	case err != nil:
		check.Status = StatusWarn
		check.Message = d.loc.T(i18n.MsgDoctorKeychainError, err)
	case !found:
		check.Status = StatusWarn
		check.Message = d.loc.T(i18n.MsgDoctorAIKeyMissing, provider)
	default:
		check.Message = d.loc.T(i18n.MsgDoctorAIKeyOK, provider)
	}
	return check
}

// hasSecret reports whether a secret exists. The value is read only to be
// discarded: it must never reach a Check.
func (d doctor) hasSecret(name string) (bool, error) {
	_, err := d.opts.Secrets.Get(name)
	switch {
	case errors.Is(err, secrets.ErrNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}
