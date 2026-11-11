package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// DefaultScopes are what gitpal asks for: repositories (including private
// ones), and workflow files so pushes that touch .github/workflows work.
const DefaultScopes = "repo workflow"

// DeviceCode is the first half of the device flow: what the user must type,
// and where.
type DeviceCode struct {
	UserCode        string
	VerificationURI string
	ExpiresIn       time.Duration
	Interval        time.Duration
	deviceCode      string
}

// DeviceFlow signs a user in without a browser redirect (RFC 8628): gitpal
// shows a code, the user enters it on github.com, gitpal receives a token.
type DeviceFlow struct {
	ClientID string
	// WebURL is github.com, or a test server.
	WebURL string
	HTTP   *http.Client
	// Sleep waits between polls; tests replace it.
	Sleep func(ctx context.Context, d time.Duration) error
}

// ErrNoClientID is returned when this build has no OAuth app to sign in with.
var ErrNoClientID = errors.New("github: no OAuth client id in this build")

// Start asks GitHub for a device code.
func (f DeviceFlow) Start(ctx context.Context) (DeviceCode, error) {
	if f.ClientID == "" {
		return DeviceCode{}, ErrNoClientID
	}
	var out struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Error           string `json:"error"`
	}
	if err := f.post(ctx, "/login/device/code", url.Values{"client_id": {f.ClientID}, "scope": {DefaultScopes}}, &out); err != nil {
		return DeviceCode{}, err
	}
	if out.Error != "" || out.DeviceCode == "" {
		return DeviceCode{}, &Error{Kind: KindInvalid, Detail: out.Error}
	}
	return DeviceCode{
		UserCode: out.UserCode, VerificationURI: out.VerificationURI, deviceCode: out.DeviceCode,
		ExpiresIn: time.Duration(out.ExpiresIn) * time.Second, Interval: time.Duration(max(out.Interval, 1)) * time.Second,
	}, nil
}

// Wait polls until the user has entered the code, and returns the token.
func (f DeviceFlow) Wait(ctx context.Context, code DeviceCode) (string, error) {
	sleep := f.Sleep
	if sleep == nil {
		sleep = func(ctx context.Context, d time.Duration) error {
			select {
			case <-time.After(d):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	interval := code.Interval
	deadline := time.Now().Add(code.ExpiresIn)
	for {
		if err := sleep(ctx, interval); err != nil {
			return "", err
		}
		var out struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}
		err := f.post(ctx, "/login/oauth/access_token", url.Values{
			"client_id": {f.ClientID}, "device_code": {code.deviceCode},
			"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"},
		}, &out)
		if err != nil {
			return "", err
		}
		switch out.Error {
		case "":
			if out.AccessToken == "" {
				return "", &Error{Kind: KindInvalid}
			}
			return out.AccessToken, nil
		case "authorization_pending":
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token":
			return "", &Error{Kind: KindAuth, Detail: "expired"}
		case "access_denied":
			return "", &Error{Kind: KindAuth, Detail: "denied"}
		default:
			return "", &Error{Kind: KindInvalid, Detail: out.Error}
		}
		if code.ExpiresIn > 0 && time.Now().After(deadline) {
			return "", &Error{Kind: KindAuth, Detail: "expired"}
		}
	}
}

func (f DeviceFlow) post(ctx context.Context, path string, form url.Values, out any) error {
	base := f.WebURL
	if base == "" {
		base = "https://github.com"
	}
	client := f.HTTP
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+path, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return classify(ctx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &Error{Kind: KindServer, Status: resp.StatusCode}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return &Error{Kind: KindServer, Status: resp.StatusCode}
	}
	return nil
}

// GHCLIToken reads the token the GitHub CLI holds (`gh auth token`), for
// users who already signed in there (spec §5). An empty token with no error
// means gh is not installed or not signed in.
//
// This is the one place outside internal/core/git that starts a process
// other than the user's editor; the command line is fixed.
func GHCLIToken(ctx context.Context, lookPath func(string) (string, error)) (string, error) {
	bin, err := lookPath("gh")
	if err != nil {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "auth", "token") //nolint:gosec,forbidigo // fixed arguments, see above
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", nil // not signed in
		}
		return "", fmt.Errorf("gh auth token: %w", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}
