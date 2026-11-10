package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// EnvConfigPath names the environment variable that overrides the location of
// the global config file.
const EnvConfigPath = "GITPAL_CONFIG"

const (
	appDirName     = "gitpal"
	globalFileName = "config.yaml"
	logFileName    = "gitpal.log"
)

// Env is the part of the process environment that file locations depend on.
// It is a value so path logic can be tested for every platform on any host.
type Env struct {
	GOOS    string
	Getenv  func(key string) string
	HomeDir func() (string, error)
}

// SystemEnv returns the real process environment.
func SystemEnv() Env {
	return Env{GOOS: runtime.GOOS, Getenv: os.Getenv, HomeDir: os.UserHomeDir}
}

// GlobalPath returns the location of the user's config file (spec §10):
// $GITPAL_CONFIG if set; otherwise %APPDATA%\gitpal\config.yaml on Windows and
// $XDG_CONFIG_HOME/gitpal/config.yaml, defaulting to ~/.config, elsewhere.
func GlobalPath(env Env) (string, error) {
	if path := env.Getenv(EnvConfigPath); path != "" {
		return path, nil
	}
	base, err := env.baseDir("APPDATA", []string{"AppData", "Roaming"}, "XDG_CONFIG_HOME", []string{".config"})
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName, globalFileName), nil
}

// LogPath returns the location of the log file (spec §11):
// %LOCALAPPDATA%\gitpal\gitpal.log on Windows and
// $XDG_STATE_HOME/gitpal/gitpal.log, defaulting to ~/.local/state, elsewhere.
func LogPath(env Env) (string, error) {
	base, err := env.baseDir("LOCALAPPDATA", []string{"AppData", "Local"}, "XDG_STATE_HOME", []string{".local", "state"})
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName, logFileName), nil
}

// baseDir picks the per-user base directory: the platform's environment
// variable when it holds an absolute path, otherwise a path under the home
// directory.
func (e Env) baseDir(winVar string, winFallback []string, xdgVar string, xdgFallback []string) (string, error) {
	envVar, fallback := xdgVar, xdgFallback
	if e.GOOS == "windows" {
		envVar, fallback = winVar, winFallback
	}
	if dir := e.Getenv(envVar); filepath.IsAbs(dir) {
		return dir, nil
	}
	home, err := e.HomeDir()
	if err == nil && home == "" {
		err = errors.New("home directory is empty")
	}
	if err != nil {
		return "", &HomeError{Err: err}
	}
	return filepath.Join(append([]string{home}, fallback...)...), nil
}
