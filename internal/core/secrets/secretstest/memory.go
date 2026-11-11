// Package secretstest provides an in-memory secrets.Store for tests, so no
// test ever reads or writes the real keychain.
package secretstest

import (
	"sync"

	"github.com/muhammedeminalan/gitpal/internal/core/secrets"
)

// Memory is a secrets.Store that keeps secrets in a map.
type Memory struct {
	// Err, when set, is returned by every call, to simulate a keychain that
	// cannot be reached.
	Err error

	mu     sync.Mutex
	values map[string]string
}

var _ secrets.Store = (*Memory)(nil)

// NewMemory returns an empty store.
func NewMemory() *Memory { return &Memory{values: map[string]string{}} }

// Get implements secrets.Store.
func (m *Memory) Get(name string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.values[name]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return value, nil
}

// Set implements secrets.Store.
func (m *Memory) Set(name, value string) error {
	if m.Err != nil {
		return m.Err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[name] = value
	return nil
}

// Delete implements secrets.Store.
func (m *Memory) Delete(name string) error {
	if m.Err != nil {
		return m.Err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, name)
	return nil
}
