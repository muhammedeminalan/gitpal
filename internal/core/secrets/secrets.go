// Package secrets stores API keys and tokens in the operating system keychain
// (macOS Keychain, Windows Credential Manager, Secret Service on Linux).
//
// Secrets are never written to config files or logs (spec §10, §11). Errors
// returned from this package never contain a secret's value.
package secrets

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// service is the keychain service all gitpal secrets are stored under.
// Changing it orphans every secret users have saved.
const service = "gitpal"

// GitHubToken is the name of the GitHub access token.
const GitHubToken = "github-token"

// AIKey returns the name of the API key for an AI provider, e.g. "anthropic".
func AIKey(provider string) string { return "ai-" + provider + "-api-key" }

// ErrNotFound is returned by Store.Get when no secret is stored under a name.
var ErrNotFound = errors.New("secret not found")

// Store reads and writes named secrets.
type Store interface {
	// Get returns the secret, or ErrNotFound.
	Get(name string) (string, error)
	// Set stores or replaces the secret.
	Set(name, value string) error
	// Delete removes the secret. Deleting a secret that does not exist is not
	// an error.
	Delete(name string) error
}

// Keyring returns the Store backed by the operating system keychain.
func Keyring() Store { return keyringStore{} }

type keyringStore struct{}

func (keyringStore) Get(name string) (string, error) {
	value, err := keyring.Get(service, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read %q from the keychain: %w", name, err)
	}
	return value, nil
}

func (keyringStore) Set(name, value string) error {
	if err := keyring.Set(service, name, value); err != nil {
		return fmt.Errorf("store %q in the keychain: %w", name, err)
	}
	return nil
}

func (keyringStore) Delete(name string) error {
	err := keyring.Delete(service, name)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete %q from the keychain: %w", name, err)
	}
	return nil
}
