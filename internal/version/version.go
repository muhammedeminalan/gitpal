// Package version carries the build's version information.
package version

import "fmt"

// Set at build time with -ldflags "-X ..." (see Makefile).
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
	// GitHubClientID is the OAuth app gitpal signs in to GitHub with
	// (device flow). A release build sets it; without it `gitpal auth
	// github` offers the gh CLI or a personal access token instead.
	GitHubClientID = ""
)

// String returns the version for display, e.g.
// "v0.1.0 (abc1234, 2026-11-11T10:00:00Z)".
func String() string { return format(Version, Commit, Date) }

func format(version, commit, date string) string {
	if commit == "" || commit == "none" {
		return version
	}
	if date == "" {
		return fmt.Sprintf("%s (%s)", version, commit)
	}
	return fmt.Sprintf("%s (%s, %s)", version, commit, date)
}
