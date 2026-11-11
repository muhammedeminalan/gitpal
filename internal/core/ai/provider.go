// Package ai writes commit messages and pull request texts with a language
// model (spec §6).
//
// Nothing reaches a provider without passing through Prepare: files that
// hold credentials are left out entirely, every line is scrubbed of anything
// that looks like a secret, lock files, minified and binary files are
// reduced to a note, and the whole is held to a size budget.
package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// Provider is a language model service (spec §6.6).
type Provider interface {
	Name() string
	GenerateCommitMessages(ctx context.Context, in CommitInput) (CommitSuggestions, error)
	GeneratePRDescription(ctx context.Context, in PRInput) (PRText, error)
}

// Style is the shape of a commit message.
type Style string

// Styles.
const (
	StyleConventional Style = "conventional"
	StyleFree         Style = "free"
)

// CommitInput is everything the model sees to suggest a commit message. Build
// the Payload with Prepare.
type CommitInput struct {
	Payload Payload
	// RecentMessages teach the model the repository's own style.
	RecentMessages []string
	Branch         string
	Style          Style
	Language       i18n.Lang
	// Suggestions is how many messages to ask for (1–3).
	Suggestions      int
	MaxSubjectLength int
}

// CommitSuggestions is what the model proposed.
type CommitSuggestions struct {
	Messages []string
	// Split, when not empty, is the model's advice to split the commit:
	// which files belong together.
	Split string
}

// PRInput is what the model sees to write a pull request.
type PRInput struct {
	Payload  Payload
	Commits  []string
	Branch   string
	Base     string
	Language i18n.Lang
}

// PRText is a pull request title and description.
type PRText struct {
	Title string
	Body  string
}

// ErrorKind classifies a failure so the user gets a clear explanation and the
// caller knows to fall back to writing the message by hand (spec §6.5).
type ErrorKind string

// Error kinds.
const (
	KindNoKey       ErrorKind = "no_key"
	KindNoModel     ErrorKind = "no_model"
	KindAuth        ErrorKind = "auth"
	KindRateLimit   ErrorKind = "rate_limit"
	KindUnavailable ErrorKind = "unavailable"
	KindNetwork     ErrorKind = "network"
	KindRefused     ErrorKind = "refused"
	KindEmpty       ErrorKind = "empty"
	KindDisabled    ErrorKind = "disabled"
)

// Error is a failed request to a provider. It carries the kind and the HTTP
// status only: a provider's raw error text can echo request content, so it
// never reaches the user or the log.
type Error struct {
	Kind     ErrorKind
	Provider string
	Status   int
}

func (e *Error) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("ai %s: %s (HTTP %d)", e.Provider, e.Kind, e.Status)
	}
	return fmt.Sprintf("ai %s: %s", e.Provider, e.Kind)
}

var kindMessages = map[ErrorKind]i18n.Key{
	KindNoKey:       i18n.MsgAINoKey,
	KindNoModel:     i18n.MsgAINoModel,
	KindAuth:        i18n.MsgAIAuth,
	KindRateLimit:   i18n.MsgAIRateLimit,
	KindUnavailable: i18n.MsgAIUnavailable,
	KindNetwork:     i18n.MsgAINetwork,
	KindRefused:     i18n.MsgAIRefused,
	KindEmpty:       i18n.MsgAIEmpty,
	KindDisabled:    i18n.MsgAIDisabled,
}

// Localize implements i18n.Localizable.
func (e *Error) Localize(lang i18n.Lang) string {
	return i18n.T(lang, kindMessages[e.Kind], e.Provider)
}

// KindOf returns the kind of an AI failure, or "" for another error.
func KindOf(err error) ErrorKind {
	var aiErr *Error
	if errors.As(err, &aiErr) {
		return aiErr.Kind
	}
	return ""
}
