package secrets_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/eminalan/gitpal/internal/core/secrets"
	"github.com/eminalan/gitpal/internal/core/secrets/secretstest"
)

// keyring.MockInit swaps go-keyring's backend for an in-memory one, so these
// tests exercise the real Keyring store without touching the OS keychain.

func TestKeyringStoreRoundTrip(t *testing.T) {
	keyring.MockInit()
	store := secrets.Keyring()

	_, err := store.Get(secrets.GitHubToken)
	require.ErrorIs(t, err, secrets.ErrNotFound)

	require.NoError(t, store.Set(secrets.GitHubToken, "ghp_example"))
	got, err := store.Get(secrets.GitHubToken)
	require.NoError(t, err)
	assert.Equal(t, "ghp_example", got)

	require.NoError(t, store.Delete(secrets.GitHubToken))
	_, err = store.Get(secrets.GitHubToken)
	require.ErrorIs(t, err, secrets.ErrNotFound)
}

func TestKeyringStoreDeleteIsIdempotent(t *testing.T) {
	keyring.MockInit()

	assert.NoError(t, secrets.Keyring().Delete("never-stored"))
}

// The service and account names are a storage contract: change them and every
// user's saved token and key silently disappear.
func TestKeyringStoreUsesStableNames(t *testing.T) {
	keyring.MockInit()
	store := secrets.Keyring()
	require.NoError(t, store.Set(secrets.GitHubToken, "token-value"))
	require.NoError(t, store.Set(secrets.AIKey("anthropic"), "key-value"))

	token, err := keyring.Get("gitpal", "github-token")
	require.NoError(t, err)
	assert.Equal(t, "token-value", token)
	key, err := keyring.Get("gitpal", "ai-anthropic-api-key")
	require.NoError(t, err)
	assert.Equal(t, "key-value", key)
}

func TestKeyringStoreReportsAnUnavailableKeychain(t *testing.T) {
	backendErr := errors.New("no secret service on this machine")
	keyring.MockInitWithError(backendErr)
	store := secrets.Keyring()

	_, err := store.Get(secrets.GitHubToken)
	require.ErrorIs(t, err, backendErr)
	assert.NotErrorIs(t, err, secrets.ErrNotFound, "an unreachable keychain is not the same as a missing secret")

	err = store.Set(secrets.GitHubToken, "ghp_do_not_print")
	require.ErrorIs(t, err, backendErr)
	assert.NotContains(t, err.Error(), "ghp_do_not_print", "a secret must never appear in an error")

	require.ErrorIs(t, store.Delete(secrets.GitHubToken), backendErr)
}

func TestMemoryStoreBehavesLikeTheKeyring(t *testing.T) {
	store := secretstest.NewMemory()

	_, err := store.Get("a")
	require.ErrorIs(t, err, secrets.ErrNotFound)

	require.NoError(t, store.Set("a", "1"))
	got, err := store.Get("a")
	require.NoError(t, err)
	assert.Equal(t, "1", got)

	require.NoError(t, store.Delete("a"))
	require.NoError(t, store.Delete("a"))
	_, err = store.Get("a")
	require.ErrorIs(t, err, secrets.ErrNotFound)
}

func TestMemoryStoreCanSimulateABrokenKeychain(t *testing.T) {
	broken := errors.New("keychain locked")
	store := secretstest.NewMemory()
	store.Err = broken

	_, err := store.Get("a")
	require.ErrorIs(t, err, broken)
	require.ErrorIs(t, store.Set("a", "1"), broken)
	require.ErrorIs(t, store.Delete("a"), broken)
}
