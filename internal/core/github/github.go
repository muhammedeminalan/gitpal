// Package github talks to GitHub for gitpal (spec §5): repositories, pull
// requests, issues, releases and Actions status.
//
// Everything here is optional. Without a token the commands that need one
// explain how to sign in; the git workflow never depends on this package.
package github

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// ErrorKind classifies a failed GitHub request.
type ErrorKind string

// Error kinds.
const (
	KindNoToken   ErrorKind = "no_token"
	KindAuth      ErrorKind = "auth"
	KindRateLimit ErrorKind = "rate_limit"
	KindNotFound  ErrorKind = "not_found"
	KindInvalid   ErrorKind = "invalid"
	KindNetwork   ErrorKind = "network"
	KindServer    ErrorKind = "server"
	KindNotGitHub ErrorKind = "not_github"
	KindNotMerged ErrorKind = "not_mergeable"
)

// Error is a failed request. It carries no response body and no token:
// GitHub's own message is kept only when it is a validation message about
// the request itself.
type Error struct {
	Kind   ErrorKind
	Status int
	// Detail is GitHub's explanation of a rejected request.
	Detail string
	// Reset is when a rate limit lifts.
	Reset time.Time
}

func (e *Error) Error() string {
	text := "github: " + string(e.Kind)
	if e.Status != 0 {
		text += fmt.Sprintf(" (HTTP %d)", e.Status)
	}
	if e.Detail != "" {
		text += ": " + e.Detail
	}
	return text
}

var kindMessages = map[ErrorKind]i18n.Key{
	KindNoToken:   i18n.MsgGHNoToken,
	KindAuth:      i18n.MsgGHAuth,
	KindRateLimit: i18n.MsgGHRateLimit,
	KindNotFound:  i18n.MsgGHNotFound,
	KindInvalid:   i18n.MsgGHInvalid,
	KindNetwork:   i18n.MsgGHNetwork,
	KindServer:    i18n.MsgGHServer,
	KindNotGitHub: i18n.MsgGHNotGitHub,
	KindNotMerged: i18n.MsgGHNotMergeable,
}

// Localize implements i18n.Localizable.
func (e *Error) Localize(lang i18n.Lang) string {
	text := i18n.T(lang, kindMessages[e.Kind])
	if e.Kind == KindRateLimit && !e.Reset.IsZero() {
		text += " " + i18n.T(lang, i18n.MsgGHRateLimitReset, e.Reset.Local().Format("15:04"))
	}
	if e.Detail != "" {
		text += "\n" + e.Detail
	}
	return text
}

// KindOf returns the kind of a GitHub failure, or "".
func KindOf(err error) ErrorKind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return ""
}

// classify turns go-github's errors into an *Error.
func classify(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var rate *gh.RateLimitError
	if errors.As(err, &rate) {
		return &Error{Kind: KindRateLimit, Status: http.StatusForbidden, Reset: rate.Rate.Reset.Time}
	}
	var abuse *gh.AbuseRateLimitError
	if errors.As(err, &abuse) {
		e := &Error{Kind: KindRateLimit, Status: http.StatusForbidden}
		if abuse.RetryAfter != nil {
			e.Reset = time.Now().Add(*abuse.RetryAfter)
		}
		return e
	}
	var resp *gh.ErrorResponse
	if errors.As(err, &resp) && resp.Response != nil {
		status := resp.Response.StatusCode
		switch {
		case status == http.StatusUnauthorized:
			return &Error{Kind: KindAuth, Status: status}
		case status == http.StatusForbidden:
			return &Error{Kind: KindAuth, Status: status, Detail: resp.Message}
		case status == http.StatusNotFound:
			return &Error{Kind: KindNotFound, Status: status}
		case status == http.StatusMethodNotAllowed || status == http.StatusConflict:
			return &Error{Kind: KindNotMerged, Status: status, Detail: resp.Message}
		case status == http.StatusUnprocessableEntity || status == http.StatusBadRequest:
			detail := resp.Message
			for _, fe := range resp.Errors {
				if fe.Message != "" {
					detail += " — " + fe.Message
				}
			}
			return &Error{Kind: KindInvalid, Status: status, Detail: detail}
		case status >= 500:
			return &Error{Kind: KindServer, Status: status}
		}
		return &Error{Kind: KindServer, Status: status}
	}
	var netErr net.Error
	var urlErr *url.Error
	if errors.As(err, &netErr) || errors.As(err, &urlErr) {
		return &Error{Kind: KindNetwork}
	}
	return &Error{Kind: KindServer}
}

// Repo names a repository on GitHub.
type Repo struct{ Owner, Name string }

func (r Repo) String() string { return r.Owner + "/" + r.Name }

var remoteURLRE = regexp.MustCompile(`^(?:https?://(?:[^@/]+@)?|ssh://(?:[^@/]+@)?|git@)([^/:]+)[/:]([^/]+)/([^/]+?)(?:\.git)?/?$`)

// ParseRemote reads owner and name from a remote URL on host (github.com
// when empty): https://github.com/o/r.git, git@github.com:o/r.git,
// ssh://git@github.com/o/r.
func ParseRemote(remoteURL, host string) (Repo, bool) {
	if host == "" {
		host = "github.com"
	}
	m := remoteURLRE.FindStringSubmatch(strings.TrimSpace(remoteURL))
	if m == nil || !strings.EqualFold(strings.TrimSuffix(m[1], ":443"), host) {
		return Repo{}, false
	}
	return Repo{Owner: m[2], Name: m[3]}, true
}

// ParseName reads "owner/name".
func ParseName(s string) (Repo, bool) {
	owner, name, ok := strings.Cut(s, "/")
	if !ok || owner == "" || name == "" || strings.ContainsAny(name, "/ ") || strings.ContainsAny(owner, " ") {
		return Repo{}, false
	}
	return Repo{Owner: owner, Name: strings.TrimSuffix(name, ".git")}, true
}

// Client is an authenticated GitHub client.
type Client struct {
	gh *gh.Client
}

// Options adjusts New.
type Options struct {
	// APIURL is the REST API's base URL (GitHub Enterprise, tests); empty is
	// api.github.com.
	APIURL string
	// HTTPClient carries the requests; nil uses a client with a timeout.
	HTTPClient *http.Client
}

// requestTimeout bounds one call to GitHub.
const requestTimeout = 30 * time.Second

// New returns a client acting with token.
func New(token string, opts Options) (*Client, error) {
	if token == "" {
		return nil, &Error{Kind: KindNoToken}
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}
	clientOpts := []gh.ClientOptionsFunc{gh.WithHTTPClient(httpClient), gh.WithAuthToken(token), gh.WithUserAgent("gitpal")}
	if opts.APIURL != "" {
		base := strings.TrimRight(opts.APIURL, "/") + "/"
		clientOpts = append(clientOpts, gh.WithURLs(&base, &base))
	}
	c, err := gh.NewClient(clientOpts...)
	if err != nil {
		return nil, err
	}
	return &Client{gh: c}, nil
}

// User returns the login the token belongs to.
func (c *Client) User(ctx context.Context) (string, error) {
	u, _, err := c.gh.Users.Get(ctx, "")
	if err != nil {
		return "", classify(ctx, err)
	}
	return u.GetLogin(), nil
}

// classifyStatus explains a bare HTTP status.
func classifyStatus(status int) *Error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &Error{Kind: KindAuth, Status: status}
	case http.StatusNotFound:
		return &Error{Kind: KindNotFound, Status: status}
	}
	return &Error{Kind: KindServer, Status: status}
}

var issueInBranchRE = regexp.MustCompile(`(?:^|[/_-])(?:issue-?|gh-?|#)?(\d{1,6})(?:[/_-]|$)`)

// IssueFromBranch finds an issue number in a branch name such as
// "fix/123-login" or "feat/gh-42", for the `Fixes #123` suggestion.
func IssueFromBranch(branch string) (int, bool) {
	m := issueInBranchRE.FindStringSubmatch(branch)
	if m == nil {
		return 0, false
	}
	n := 0
	for _, c := range m[1] {
		n = n*10 + int(c-'0')
	}
	return n, n > 0
}
