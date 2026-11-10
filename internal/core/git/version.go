package git

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version is a git release number.
type Version struct {
	Major, Minor, Patch int
}

// MinVersion is the oldest git gitpal supports (spec §11).
var MinVersion = Version{Major: 2, Minor: 30}

var versionRE = regexp.MustCompile(`^git version (\d+)\.(\d+)(?:\.(\d+))?`)

// ParseVersion reads the output of `git version`, e.g.
// "git version 2.54.0 (Apple Git-157)" or "git version 2.47.1.windows.2".
func ParseVersion(output string) (Version, error) {
	match := versionRE.FindStringSubmatch(strings.TrimSpace(output))
	if match == nil {
		return Version{}, fmt.Errorf("unrecognized git version output %q", truncate(strings.TrimSpace(output), 80))
	}
	var v Version
	for i, target := range []*int{&v.Major, &v.Minor, &v.Patch} {
		if match[i+1] == "" {
			continue
		}
		n, err := strconv.Atoi(match[i+1])
		if err != nil {
			return Version{}, fmt.Errorf("parse git version %q: %w", match[0], err)
		}
		*target = n
	}
	return v, nil
}

// AtLeast reports whether v is the same as or newer than minimum.
func (v Version) AtLeast(minimum Version) bool {
	switch {
	case v.Major != minimum.Major:
		return v.Major > minimum.Major
	case v.Minor != minimum.Minor:
		return v.Minor > minimum.Minor
	default:
		return v.Patch >= minimum.Patch
	}
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Version returns the version of the git executable.
func (r *Runner) Version(ctx context.Context) (Version, error) {
	res, err := r.Run(ctx, Cmd{Args: []string{"version"}})
	if err != nil {
		return Version{}, err
	}
	return ParseVersion(string(res.Stdout))
}
