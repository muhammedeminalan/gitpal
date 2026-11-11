package logging_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/muhammedeminalan/gitpal/internal/core/logging"
)

func TestOpenWritesToAPrivateFileInANewDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "gitpal", "gitpal.log")

	logger, closer, err := logging.Open(logging.Options{Path: path, Level: slog.LevelInfo})
	require.NoError(t, err)
	logger.Info("command finished", slog.String("command", "doctor"), slog.Int("exit", 0))
	require.NoError(t, closer.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "command finished")
	assert.Contains(t, string(data), "command=doctor")
	assert.Contains(t, string(data), "exit=0")

	if runtime.GOOS != "windows" { // Windows has no owner-only permission bits
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "the log may name files and branches; keep it private")
		dirInfo, err := os.Stat(filepath.Dir(path))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
	}
}

func TestOpenAppendsToAnExistingLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitpal.log")

	for _, message := range []string{"first run", "second run"} {
		logger, closer, err := logging.Open(logging.Options{Path: path, Level: slog.LevelInfo})
		require.NoError(t, err)
		logger.Info(message)
		require.NoError(t, closer.Close())
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "first run")
	assert.Contains(t, string(data), "second run")
}

func TestLevelDecidesWhatIsWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitpal.log")

	logger, closer, err := logging.Open(logging.Options{Path: path, Level: slog.LevelWarn})
	require.NoError(t, err)
	logger.Debug("debug line")
	logger.Info("info line")
	logger.Warn("warn line")
	logger.Error("error line")
	require.NoError(t, closer.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "debug line")
	assert.NotContains(t, string(data), "info line")
	assert.Contains(t, string(data), "warn line")
	assert.Contains(t, string(data), "error line")
}

func TestOpenFailsWhenTheFileCannotBeCreated(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))

	_, _, err := logging.Open(logging.Options{Path: filepath.Join(blocker, "gitpal.log")})

	assert.Error(t, err)
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{in: "debug", want: slog.LevelDebug},
		{in: "info", want: slog.LevelInfo},
		{in: "warn", want: slog.LevelWarn},
		{in: "error", want: slog.LevelError},
		{in: " WARN ", want: slog.LevelWarn},
		{in: "", want: slog.LevelInfo},
		{in: "verbose", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := logging.ParseLevel(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDiscardDropsEverything(t *testing.T) {
	logger := logging.Discard()

	assert.False(t, logger.Enabled(context.Background(), slog.LevelError))
}
