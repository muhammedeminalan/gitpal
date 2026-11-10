package config_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eminalan/gitpal/internal/core/config"
)

func envWith(goos, home string, vars map[string]string) config.Env {
	return config.Env{
		GOOS:    goos,
		Getenv:  func(key string) string { return vars[key] },
		HomeDir: func() (string, error) { return home, nil },
	}
}

func TestGlobalPath(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(t.TempDir(), "xdg")
	appdata := filepath.Join(t.TempDir(), "AppData", "Roaming")
	custom := filepath.Join(t.TempDir(), "my.yaml")

	tests := []struct {
		name string
		goos string
		vars map[string]string
		want string
	}{
		{name: "linux default", goos: "linux", want: filepath.Join(home, ".config", "gitpal", "config.yaml")},
		{name: "macOS uses ~/.config like the spec says", goos: "darwin", want: filepath.Join(home, ".config", "gitpal", "config.yaml")},
		{name: "XDG_CONFIG_HOME", goos: "linux", vars: map[string]string{"XDG_CONFIG_HOME": xdg}, want: filepath.Join(xdg, "gitpal", "config.yaml")},
		{name: "relative XDG_CONFIG_HOME is ignored", goos: "linux", vars: map[string]string{"XDG_CONFIG_HOME": "relative/dir"}, want: filepath.Join(home, ".config", "gitpal", "config.yaml")},
		{name: "windows uses APPDATA", goos: "windows", vars: map[string]string{"APPDATA": appdata}, want: filepath.Join(appdata, "gitpal", "config.yaml")},
		{name: "windows without APPDATA", goos: "windows", want: filepath.Join(home, "AppData", "Roaming", "gitpal", "config.yaml")},
		{name: "windows ignores XDG_CONFIG_HOME", goos: "windows", vars: map[string]string{"APPDATA": appdata, "XDG_CONFIG_HOME": xdg}, want: filepath.Join(appdata, "gitpal", "config.yaml")},
		{name: "GITPAL_CONFIG wins", goos: "linux", vars: map[string]string{"GITPAL_CONFIG": custom, "XDG_CONFIG_HOME": xdg}, want: custom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.GlobalPath(envWith(tt.goos, home, tt.vars))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLogPath(t *testing.T) {
	home := t.TempDir()
	state := filepath.Join(t.TempDir(), "state")
	local := filepath.Join(t.TempDir(), "AppData", "Local")

	tests := []struct {
		name string
		goos string
		vars map[string]string
		want string
	}{
		{name: "linux default", goos: "linux", want: filepath.Join(home, ".local", "state", "gitpal", "gitpal.log")},
		{name: "macOS default", goos: "darwin", want: filepath.Join(home, ".local", "state", "gitpal", "gitpal.log")},
		{name: "XDG_STATE_HOME", goos: "linux", vars: map[string]string{"XDG_STATE_HOME": state}, want: filepath.Join(state, "gitpal", "gitpal.log")},
		{name: "windows uses LOCALAPPDATA", goos: "windows", vars: map[string]string{"LOCALAPPDATA": local}, want: filepath.Join(local, "gitpal", "gitpal.log")},
		{name: "windows without LOCALAPPDATA", goos: "windows", want: filepath.Join(home, "AppData", "Local", "gitpal", "gitpal.log")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.LogPath(envWith(tt.goos, home, tt.vars))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPathsFailWhenHomeIsUnknown(t *testing.T) {
	env := config.Env{
		GOOS:    "linux",
		Getenv:  func(string) string { return "" },
		HomeDir: func() (string, error) { return "", errors.New("no home") },
	}

	_, err := config.GlobalPath(env)
	assert.Error(t, err)
	_, err = config.LogPath(env)
	assert.Error(t, err)
}

func TestExplicitPathsDoNotNeedAHome(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "c.yaml")
	env := config.Env{
		GOOS:    "linux",
		Getenv:  func(key string) string { return map[string]string{"GITPAL_CONFIG": custom}[key] },
		HomeDir: func() (string, error) { return "", errors.New("no home") },
	}

	got, err := config.GlobalPath(env)
	require.NoError(t, err)
	assert.Equal(t, custom, got)
}
