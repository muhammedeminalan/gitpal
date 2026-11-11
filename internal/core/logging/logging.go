// Package logging sets up gitpal's log file (spec §11).
//
// The log records what gitpal did — commands, exit codes, failures — and
// never what it handled: no tokens, no API keys, no diff content. Callers are
// responsible for passing only such metadata; the file is still kept private
// because it names files and branches.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Options configure Open.
type Options struct {
	// Path is the log file; see config.LogPath for the default location.
	Path string
	// Level is the least severe level that is written.
	Level slog.Level
	// MaxBytes is the size at which the log is rotated when opened: the file
	// becomes Path + ".1", replacing the previous one. 0 means DefaultMaxBytes.
	MaxBytes int64
}

// DefaultMaxBytes is the log size that triggers rotation.
const DefaultMaxBytes = 5 << 20

// Open returns a logger that appends to opts.Path, creating the file and its
// directory if needed. Close the returned closer when done.
func Open(opts Options) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o700); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}
	rotate(opts.Path, opts.MaxBytes)
	file, err := os.OpenFile(opts.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	handler := slog.NewTextHandler(file, &slog.HandlerOptions{Level: opts.Level})
	return slog.New(handler), file, nil
}

// rotate keeps the log from growing without end. It checks only when the log
// is opened, which for gitpal is once per command; one backup is kept. A
// failure is ignored: the log then just keeps growing.
func rotate(path string, maxBytes int64) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxBytes {
		return
	}
	// Rename replaces the old backup on Unix; Windows refuses, so remove it first.
	_ = os.Remove(path + ".1")
	_ = os.Rename(path, path+".1")
}

// Discard returns a logger that writes nothing, for when the log file cannot
// be opened: logging must never be the reason a command fails.
func Discard() *slog.Logger {
	return slog.New(discardHandler{})
}

// ParseLevel converts "debug", "info", "warn" or "error" to a level. An empty
// string means info.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q (use debug, info, warn or error)", s)
	}
}
