//go:build windows

package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// In a terminal git must keep sharing the terminal's console: that is where
// ssh asks for a passphrase. Without a console (gitpal-gui) every git would
// otherwise pop up a console window of its own.
func TestProcessAttrHidesGitOnlyWhenThereIsNoConsole(t *testing.T) {
	assert.Nil(t, processAttr(true), "with a console, git must be started the ordinary way")

	attr := processAttr(false)
	require.NotNil(t, attr)
	assert.True(t, attr.HideWindow)
	assert.Equal(t, uint32(createNoWindow), attr.CreationFlags)
}

// The settings used by the desktop app must not break running git or reading
// its output. This runs the real git the way gitpal-gui does.
func TestGitRunsAndIsReadableWithoutAConsoleWindow(t *testing.T) {
	original := hasConsoleWindow
	hasConsoleWindow = func() bool { return false }
	t.Cleanup(func() { hasConsoleWindow = original })

	res, err := NewRunner().Run(context.Background(), Cmd{Args: []string{"version"}})

	require.NoError(t, err)
	assert.Contains(t, string(res.Stdout), "git version")
}
