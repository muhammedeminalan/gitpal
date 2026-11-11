package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/spf13/viper"
)

// LoadOptions selects the files Load reads.
type LoadOptions struct {
	// GlobalPath is the user's config file (see GlobalPath). Empty skips it.
	GlobalPath string
	// RepoDir is the repository root whose RepoFileName is applied on top.
	// Empty means gitpal is not working inside a repository.
	RepoDir string
}

// Loaded is the result of Load.
type Loaded struct {
	Config Config

	GlobalPath  string
	GlobalFound bool
	RepoPath    string
	RepoFound   bool

	// Warnings are problems that did not stop loading: unknown keys, and
	// repository settings that were ignored.
	Warnings []Warning
}

// repoKeys are the settings a repository file may set. Everything else is read
// from the user's global file only — in particular the whole `ai` section,
// which decides whether and where code leaves the machine.
var repoKeys = map[string]bool{
	"commit.style":                 true,
	"commit.message_language":      true,
	"commit.suggestions":           true,
	"commit.max_subject_length":    true,
	"pull.strategy":                true,
	"safety.protected_branches":    true,
	"safety.large_file_mb":         true,
	"safety.backup_retention_days": true,
	"safety.warn_on_secrets":       true,
}

var settingKeys, sectionKeys = collectKeys(reflect.TypeOf(Config{}), "")

// Load builds the effective settings: defaults, then the global file, then the
// repository file.
//
// A global file that cannot be parsed or holds an invalid value is an error:
// the user owns it and can fix it. The returned Loaded then carries the
// defaults.
//
// The repository file is untrusted. It may only set the keys in repoKeys, and
// its safety settings can only tighten what the global layer says: protected
// branches are added, never removed; the large-file threshold can go down and
// backup retention up, not the other way round; secret warnings cannot be
// switched off. A repository file that is malformed or invalid is skipped as a
// whole and reported as a warning, so a broken file in a clone can neither
// stop gitpal nor loosen protection.
func Load(opts LoadOptions) (Loaded, error) {
	out := Loaded{Config: Default(), GlobalPath: opts.GlobalPath}
	cfg := Default()

	if opts.GlobalPath != "" {
		global, found, err := readLayer(opts.GlobalPath)
		if err != nil {
			return out, &FileError{Path: opts.GlobalPath, Err: err}
		}
		if found {
			out.GlobalFound = true
			out.Warnings = append(out.Warnings, global.unknownKeyWarnings()...)
			if err := global.decodeInto(&cfg); err != nil {
				return out, &FileError{Path: opts.GlobalPath, Err: err}
			}
			cfg.normalize()
			if err := cfg.Validate(); err != nil {
				return out, &FileError{Path: opts.GlobalPath, Err: err}
			}
		}
	}

	if opts.RepoDir != "" {
		out.RepoPath = filepath.Join(opts.RepoDir, RepoFileName)
		merged, warnings, found, err := applyRepoFile(cfg, out.RepoPath)
		out.RepoFound = found
		if err != nil {
			out.Warnings = append(out.Warnings, Warning{Code: WarnRepoFileIgnored, File: out.RepoPath, Err: err})
		} else {
			cfg = merged
			out.Warnings = append(out.Warnings, warnings...)
		}
	}

	out.Config = cfg
	return out, nil
}

// applyRepoFile returns base with the repository file at path applied under
// the rules described on Load. On error base is returned unchanged.
func applyRepoFile(base Config, path string) (cfg Config, warnings []Warning, found bool, err error) {
	repo, found, err := readLayer(path)
	if err != nil || !found {
		return base, nil, found, err
	}

	var over Config
	if err := repo.decodeInto(&over); err != nil {
		return base, nil, true, err
	}
	over.normalize()

	warn := func(code WarningCode, key string) {
		warnings = append(warnings, Warning{Code: code, File: path, Key: key})
	}
	for _, key := range repo.keys() {
		switch {
		case sectionKeys[key]:
			// An empty section sets nothing.
		case !settingKeys[key]:
			warn(WarnUnknownKey, key)
		case !repoKeys[key]:
			warn(WarnRepoKeyLocked, key)
		}
	}

	cfg = base
	if repo.has("commit.style") {
		cfg.Commit.Style = over.Commit.Style
	}
	if repo.has("commit.message_language") {
		cfg.Commit.MessageLanguage = over.Commit.MessageLanguage
	}
	if repo.has("commit.suggestions") {
		cfg.Commit.Suggestions = over.Commit.Suggestions
	}
	if repo.has("commit.max_subject_length") {
		cfg.Commit.MaxSubjectLength = over.Commit.MaxSubjectLength
	}
	if repo.has("pull.strategy") {
		cfg.Pull.Strategy = over.Pull.Strategy
	}

	if repo.has("safety.protected_branches") {
		cfg.Safety.ProtectedBranches = branchList(base.Safety.ProtectedBranches, over.Safety.ProtectedBranches)
	}
	if repo.has("safety.warn_on_secrets") {
		switch {
		case over.Safety.WarnOnSecrets:
			cfg.Safety.WarnOnSecrets = true
		case base.Safety.WarnOnSecrets:
			warn(WarnRepoWeaker, "safety.warn_on_secrets")
		}
	}
	if repo.has("safety.large_file_mb") {
		if over.Safety.LargeFileMB <= base.Safety.LargeFileMB {
			cfg.Safety.LargeFileMB = over.Safety.LargeFileMB
		} else {
			warn(WarnRepoWeaker, "safety.large_file_mb")
		}
	}
	if repo.has("safety.backup_retention_days") {
		if over.Safety.BackupRetentionDays >= base.Safety.BackupRetentionDays {
			cfg.Safety.BackupRetentionDays = over.Safety.BackupRetentionDays
		} else {
			warn(WarnRepoWeaker, "safety.backup_retention_days")
		}
	}

	if err := cfg.Validate(); err != nil {
		return base, nil, true, err
	}
	return cfg, warnings, true, nil
}

// layer is one parsed config file.
type layer struct {
	path string
	v    *viper.Viper
}

func readLayer(path string) (l *layer, found bool, err error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, true, err
	}
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, true, err
	}
	return &layer{path: path, v: v}, true, nil
}

// keys returns the dotted names of everything the file sets, sorted.
func (l *layer) keys() []string {
	keys := l.v.AllKeys()
	slices.Sort(keys)
	return keys
}

func (l *layer) has(key string) bool { return l.v.IsSet(key) }

// decodeInto writes the file's values over cfg. Settings the file does not
// mention keep the value cfg already has.
func (l *layer) decodeInto(cfg *Config) error {
	if l.has("safety.protected_branches") {
		// A list in the file replaces the existing list; it is never merged
		// element by element.
		cfg.Safety.ProtectedBranches = nil
	}
	if l.has("workspace.repos") {
		cfg.Workspace.Repos = nil
	}
	return l.v.Unmarshal(cfg)
}

func (l *layer) unknownKeyWarnings() []Warning {
	var warnings []Warning
	for _, key := range l.keys() {
		if !settingKeys[key] && !sectionKeys[key] {
			warnings = append(warnings, Warning{Code: WarnUnknownKey, File: l.path, Key: key})
		}
	}
	return warnings
}

// collectKeys walks a settings struct and returns the dotted names of its
// settings and of its sections, as named by the mapstructure tags.
func collectKeys(t reflect.Type, prefix string) (settings, sections map[string]bool) {
	settings, sections = map[string]bool{}, map[string]bool{}
	for i := range t.NumField() {
		field := t.Field(i)
		name := prefix + field.Tag.Get("mapstructure")
		if field.Type.Kind() != reflect.Struct {
			settings[name] = true
			continue
		}
		sections[name] = true
		subSettings, subSections := collectKeys(field.Type, name+".")
		for key := range subSettings {
			settings[key] = true
		}
		for key := range subSections {
			sections[key] = true
		}
	}
	return settings, sections
}
