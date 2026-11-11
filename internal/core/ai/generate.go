package ai

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
)

// maxAttempts is how often an empty or unusable answer is asked for again
// before falling back to writing by hand (spec §6.5: retry at most twice).
const maxAttempts = 3

// completer sends one request and returns the model's text.
type completer interface {
	complete(ctx context.Context, system, user string) (string, error)
}

// llm implements Provider on top of any completer.
type llm struct {
	name string
	c    completer
}

func (m *llm) Name() string { return m.name }

func (m *llm) GenerateCommitMessages(ctx context.Context, in CommitInput) (CommitSuggestions, error) {
	system, user := commitPrompt(in)
	for range maxAttempts {
		text, err := m.c.complete(ctx, system, user)
		if err != nil {
			return CommitSuggestions{}, err
		}
		if out := parseCommit(text, in.MaxSubjectLength); len(out.Messages) > 0 {
			if n := max(in.Suggestions, 1); len(out.Messages) > n {
				out.Messages = out.Messages[:n]
			}
			return out, nil
		}
	}
	return CommitSuggestions{}, &Error{Kind: KindEmpty, Provider: m.name}
}

func (m *llm) GeneratePRDescription(ctx context.Context, in PRInput) (PRText, error) {
	system, user := prPrompt(in)
	for range maxAttempts {
		text, err := m.c.complete(ctx, system, user)
		if err != nil {
			return PRText{}, err
		}
		if out := parsePR(text); out.Title != "" {
			return out, nil
		}
	}
	return PRText{}, &Error{Kind: KindEmpty, Provider: m.name}
}

// classifyHTTP turns an HTTP status into an error kind.
func classifyHTTP(provider string, status int) *Error {
	kind := KindUnavailable
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		kind = KindAuth
	case http.StatusTooManyRequests:
		kind = KindRateLimit
	}
	return &Error{Kind: kind, Provider: provider, Status: status}
}

// classifyTransport turns a failure to get any answer into an error kind.
func classifyTransport(ctx context.Context, provider string, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var netErr net.Error
	var urlErr *url.Error
	if errors.As(err, &netErr) || errors.As(err, &urlErr) {
		return &Error{Kind: KindNetwork, Provider: provider}
	}
	return &Error{Kind: KindUnavailable, Provider: provider}
}
