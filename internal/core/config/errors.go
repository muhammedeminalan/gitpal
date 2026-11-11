package config

import (
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// ValidationError reports a setting whose value gitpal cannot work with.
type ValidationError struct {
	// Key is the dotted setting name, e.g. "pull.strategy".
	Key string
	// Value is the rejected value as written.
	Value string
	// Allowed lists the accepted values of a multiple-choice setting.
	Allowed []string
	// Min and Max bound a numeric setting; they are zero for choices.
	Min, Max int

	number int
}

func (e *ValidationError) Error() string { return e.Localize(i18n.EN) }

// Localize implements i18n.Localizable.
func (e *ValidationError) Localize(lang i18n.Lang) string {
	if len(e.Allowed) > 0 {
		return i18n.T(lang, i18n.MsgConfigInvalidChoice, e.Key, e.Value, strings.Join(e.Allowed, ", "))
	}
	return i18n.T(lang, i18n.MsgConfigInvalidRange, e.Key, e.number, e.Min, e.Max)
}

// FileError reports a config file that could not be read, parsed or accepted.
type FileError struct {
	Path string
	Err  error
}

func (e *FileError) Error() string { return e.Localize(i18n.EN) }

func (e *FileError) Unwrap() error { return e.Err }

// Localize implements i18n.Localizable.
func (e *FileError) Localize(lang i18n.Lang) string {
	return i18n.T(lang, i18n.MsgConfigUnreadable, e.Path, i18n.Describe(lang, e.Err))
}

// HomeError reports that the user's home directory is unknown, so the default
// config and log locations cannot be worked out.
type HomeError struct{ Err error }

func (e *HomeError) Error() string { return e.Localize(i18n.EN) }

func (e *HomeError) Unwrap() error { return e.Err }

// Localize implements i18n.Localizable.
func (e *HomeError) Localize(lang i18n.Lang) string {
	return i18n.T(lang, i18n.MsgConfigNoHome, e.Err)
}

// WarningCode classifies a non-fatal problem found while loading settings.
type WarningCode string

// Warning codes.
const (
	// WarnUnknownKey: a file sets a key gitpal does not know (often a typo).
	WarnUnknownKey WarningCode = "unknown_key"
	// WarnRepoKeyLocked: a repository file sets a key that is only read from the global file.
	WarnRepoKeyLocked WarningCode = "repo_key_locked"
	// WarnRepoWeaker: a repository file tried to loosen a safety setting.
	WarnRepoWeaker WarningCode = "repo_weaker"
	// WarnRepoFileIgnored: a repository file was unusable and skipped as a whole.
	WarnRepoFileIgnored WarningCode = "repo_file_ignored"
)

// Warning is a non-fatal problem found while loading settings.
type Warning struct {
	Code WarningCode
	// File is the config file the warning is about.
	File string
	// Key is the setting concerned; empty for file-level warnings.
	Key string
	// Err is the reason a file was ignored (WarnRepoFileIgnored only).
	Err error
}

// Localize implements i18n.Localizable.
func (w Warning) Localize(lang i18n.Lang) string {
	switch w.Code {
	case WarnUnknownKey:
		return i18n.T(lang, i18n.MsgConfigUnknownKey, w.File, w.Key)
	case WarnRepoKeyLocked:
		return i18n.T(lang, i18n.MsgConfigRepoKeyLocked, w.File, w.Key)
	case WarnRepoWeaker:
		return i18n.T(lang, i18n.MsgConfigRepoWeaker, w.File, w.Key)
	case WarnRepoFileIgnored:
		return i18n.T(lang, i18n.MsgConfigRepoFileIgnored, w.File, i18n.Describe(lang, w.Err))
	default:
		return string(w.Code)
	}
}
