package git

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	branchHeads   = "refs/heads/"
	branchRemotes = "refs/remotes/"
)

// Branch is a local or remote-tracking branch.
type Branch struct {
	// Name is the short name: "main", or "origin/main" for a remote-tracking
	// branch. A local branch can be called "origin/main" too; Ref and Remote
	// tell them apart.
	Name string
	// Ref is the full ref name, e.g. "refs/heads/main".
	Ref string
	// Remote is set for remote-tracking branches.
	Remote bool
	// RemoteName is the remote a remote-tracking branch belongs to.
	RemoteName string
	// Hash is the commit the branch points at.
	Hash string
	// Current is set for the branch that is checked out.
	Current bool
	// Upstream is the short name of the branch this one tracks, if any.
	Upstream string
	// UpstreamGone is set when the upstream no longer exists.
	UpstreamGone  bool
	Ahead, Behind int
	// Subject is the first line of the last commit's message.
	Subject       string
	CommitterDate time.Time
}

// BranchListOptions adjusts Branches.
type BranchListOptions struct {
	// Remotes also lists remote-tracking branches.
	Remotes bool
}

// BranchPrefixes are the conventional prefixes offered when naming a branch.
var BranchPrefixes = []string{"feat/", "fix/", "chore/", "docs/", "refactor/", "test/"}

// branchFields is what Branches asks for-each-ref to print for each ref,
// separated by NUL. None of the fields can contain a NUL or a newline, so a
// line is exactly one ref.
var branchFields = []string{
	"%(refname)", "%(objectname)", "%(HEAD)", "%(upstream)", "%(upstream:track)", "%(symref)",
	"%(committerdate:iso-strict)", "%(contents:subject)",
}

// Branches lists the local branches, sorted by name, followed by the
// remote-tracking ones when asked for. Symbolic refs such as origin/HEAD are
// not branches and are left out.
func (r *Runner) Branches(ctx context.Context, dir string, opts BranchListOptions) ([]Branch, error) {
	args := []string{"for-each-ref", "--sort=refname", "--format=" + strings.Join(branchFields, "%00"), branchHeads}
	var remotes []string
	if opts.Remotes {
		var err error
		if remotes, err = r.remoteNames(ctx, dir); err != nil {
			return nil, err
		}
		args = append(args, branchRemotes)
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: args})
	if err != nil {
		return nil, err
	}
	return branchParseList(res.Stdout, remotes)
}

// branchParseList parses the output of the for-each-ref command Branches
// runs. remotes are the configured remote names.
//
// Short names are cut from the full ref here rather than asked from git:
// git's own short form changes when a name is ambiguous.
func branchParseList(out []byte, remotes []string) ([]Branch, error) {
	var branches []Branch
	for line := range strings.SplitSeq(strings.TrimSuffix(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(strings.TrimSuffix(line, "\r"), "\x00")
		if len(f) != len(branchFields) {
			return nil, errors.New("git for-each-ref: unexpected number of fields")
		}
		if f[5] != "" {
			continue // a symbolic ref
		}
		b := Branch{Ref: f[0], Hash: f[1], Current: f[2] == "*", Subject: f[7]}
		switch {
		case strings.HasPrefix(b.Ref, branchHeads):
			b.Name = b.Ref[len(branchHeads):]
		case strings.HasPrefix(b.Ref, branchRemotes):
			b.Name = b.Ref[len(branchRemotes):]
			b.Remote = true
			b.RemoteName = branchRemoteOf(b.Name, remotes)
		default:
			return nil, errors.New("git for-each-ref: unexpected ref")
		}
		b.Upstream = strings.TrimPrefix(strings.TrimPrefix(f[3], branchRemotes), branchHeads)
		var err error
		if b.Ahead, b.Behind, b.UpstreamGone, err = branchParseTrack(f[4]); err != nil {
			return nil, err
		}
		if b.CommitterDate, err = time.Parse(time.RFC3339, f[6]); err != nil {
			return nil, errors.New("git for-each-ref: malformed date")
		}
		branches = append(branches, b)
	}
	return branches, nil
}

// branchRemoteOf returns the remote a remote-tracking branch belongs to: the
// longest configured remote name its short name starts with, since remote
// names may contain slashes. For a ref left behind by a remote that is no
// longer configured, it falls back to the first path segment.
func branchRemoteOf(name string, remotes []string) string {
	best := ""
	for _, remote := range remotes {
		if strings.HasPrefix(name, remote+"/") && len(remote) > len(best) {
			best = remote
		}
	}
	if best == "" {
		best, _, _ = strings.Cut(name, "/")
	}
	return best
}

// branchParseTrack parses %(upstream:track): "", "[gone]", "[ahead 1]",
// "[behind 2]" or "[ahead 1, behind 2]".
func branchParseTrack(track string) (ahead, behind int, gone bool, err error) {
	if track == "" {
		return 0, 0, false, nil
	}
	malformed := errors.New("git for-each-ref: malformed tracking information")
	if !strings.HasPrefix(track, "[") || !strings.HasSuffix(track, "]") {
		return 0, 0, false, malformed
	}
	inner := track[1 : len(track)-1]
	if inner == "gone" {
		return 0, 0, true, nil
	}
	for part := range strings.SplitSeq(inner, ", ") {
		word, number, _ := strings.Cut(part, " ")
		n, convErr := strconv.Atoi(number)
		if convErr != nil {
			return 0, 0, false, malformed
		}
		switch word {
		case "ahead":
			ahead = n
		case "behind":
			behind = n
		default:
			return 0, 0, false, malformed
		}
	}
	return ahead, behind, false, nil
}

// remoteNames returns the names of the configured remotes.
func (r *Runner) remoteNames(ctx context.Context, dir string) ([]string, error) {
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"remote"}})
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(res.Stdout)), nil
}

// CurrentBranch returns the name of the checked-out branch, or "" when HEAD
// is detached. Before the first commit it is the branch about to be born.
func (r *Runner) CurrentBranch(ctx context.Context, dir string) (string, error) {
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"symbolic-ref", "--quiet", "HEAD"}})
	if err != nil {
		// With --quiet, a detached HEAD is exit status 1 and no message.
		var gerr *Error
		if code, exited := ExitCode(err); exited && code == 1 && errorsAs(err, &gerr) && gerr.Stderr == "" {
			return "", nil
		}
		return "", err
	}
	return strings.TrimPrefix(strings.TrimSpace(string(res.Stdout)), branchHeads), nil
}

// branchExists reports whether a local branch with that name exists.
func (r *Runner) branchExists(ctx context.Context, dir, name string) (bool, error) {
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"show-ref", "--verify", "--quiet", branchHeads + name}})
	if err == nil {
		return true, nil
	}
	var gerr *Error
	if code, exited := ExitCode(err); exited && code == 1 && errorsAs(err, &gerr) && gerr.Stderr == "" {
		return false, nil
	}
	return false, err
}

// branchRequire returns a KindRefNotFound error when the local branch does
// not exist, so every operation reports a missing branch the same way
// whatever git's wording for that subcommand is.
func (r *Runner) branchRequire(ctx context.Context, dir, name string) error {
	exists, err := r.branchExists(ctx, dir, name)
	if err != nil || exists {
		return err
	}
	return &Error{Kind: KindRefNotFound, Args: []string{"show-ref", "--verify", branchHeads + name}, ExitCode: 1}
}

// ValidateBranchName checks that name can be given to a new branch. A name
// git would refuse, or would read as something else, is reported as an
// *ArgError with code ArgInvalidBranch.
//
// Beyond git's own rules it refuses "HEAD" and "@", shortcuts such as
// "@{-1}" (which git expands to another branch's name) and names starting
// with "refs/" (git would create "refs/heads/refs/…", which is never what
// was meant).
func (r *Runner) ValidateBranchName(ctx context.Context, dir, name string) error {
	invalid := &ArgError{Code: ArgInvalidBranch, Value: name}
	if err := checkName(name, ArgInvalidBranch); err != nil {
		return err
	}
	if name == "HEAD" || name == "@" || strings.HasPrefix(name, "refs/") {
		return invalid
	}
	// name cannot start with "-" (checkName), so it cannot be read as an option.
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"check-ref-format", "--branch", name}})
	if err != nil {
		if _, exited := ExitCode(err); exited {
			return invalid
		}
		return err
	}
	if strings.TrimSuffix(string(res.Stdout), "\n") != name {
		return invalid
	}
	return nil
}

// branchTransliterate maps letters outside ASCII to their closest ASCII
// spelling. Turkish comes first; the rest covers common Western European
// letters. Letters not listed are dropped by SuggestBranchName.
var branchTransliterate = map[rune]string{
	'ç': "c", 'ğ': "g", 'ı': "i", 'ö': "o", 'ş': "s", 'ü': "u",
	'â': "a", 'î': "i", 'û': "u",
	'á': "a", 'à': "a", 'ä': "a", 'ã': "a", 'å': "a",
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
	'í': "i", 'ì': "i", 'ï': "i",
	'ó': "o", 'ò': "o", 'ô': "o", 'õ': "o",
	'ú': "u", 'ù': "u",
	'ñ': "n", 'ß': "ss",
}

// SuggestBranchName turns free text into a valid branch name: lower case,
// Turkish letters spelled in ASCII, anything else replaced by "-". Slashes
// are kept, so "feat/Yeni Özellik!" becomes "feat/yeni-ozellik". It returns
// "" when nothing usable is left.
func SuggestBranchName(input string) string {
	var segments []string
	for raw := range strings.SplitSeq(input, "/") {
		segment := branchSuggestSegment(raw)
		// A name starting with "refs/" is refused by ValidateBranchName.
		if segment == "" || (len(segments) == 0 && segment == "refs") {
			continue
		}
		segments = append(segments, segment)
	}
	return strings.Join(segments, "/")
}

func branchSuggestSegment(raw string) string {
	var b strings.Builder
	last := byte(0)
	write := func(s string) {
		for i := 0; i < len(s); i++ {
			// Runs of "-" collapse, and so do runs of ".": git refuses "..".
			if (s[i] == '-' || s[i] == '.') && s[i] == last {
				continue
			}
			b.WriteByte(s[i])
			last = s[i]
		}
	}
	for _, c := range raw {
		// Lower-casing the Turkish dotted capital I can leave a combining dot
		// behind; it is simply an "i" here.
		if c == 'İ' {
			c = 'i'
		}
		c = unicode.ToLower(c)
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '.':
			write(string(c))
		case branchTransliterate[c] != "":
			write(branchTransliterate[c])
		case c > unicode.MaxASCII && unicode.IsLetter(c):
			// A letter with no ASCII spelling here: dropped.
		default:
			write("-")
		}
	}

	// A segment may not start with "." or end with ".lock", and a leading or
	// trailing "-" is only noise.
	segment := b.String()
	for {
		trimmed := strings.Trim(strings.TrimSuffix(segment, ".lock"), "-.")
		if trimmed == segment {
			return segment
		}
		segment = trimmed
	}
}

// branchTrackFlag decides whether a new branch tracks its start point. The
// rule is fixed, whatever branch.autoSetupMerge says: a branch started from
// a remote-tracking branch tracks it, any other start point is not tracked.
func (r *Runner) branchTrackFlag(ctx context.Context, dir, start string) (string, error) {
	if start == "" {
		return "--no-track", nil
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{
		"rev-parse", "--symbolic-full-name", "--verify", "--quiet", endOfOptions, start,
	}})
	if err != nil {
		if code, exited := ExitCode(err); exited && code == 1 {
			// Not resolvable: let the command that follows report it.
			return "--no-track", nil
		}
		return "", err
	}
	if strings.HasPrefix(strings.TrimSpace(string(res.Stdout)), branchRemotes) {
		return "--track", nil
	}
	return "--no-track", nil
}

// branchNewArgs validates the name and start point of a branch to create and
// returns the tracking flag to use.
func (r *Runner) branchNewArgs(ctx context.Context, dir, name, start string) (track string, err error) {
	if err := r.ValidateBranchName(ctx, dir, name); err != nil {
		return "", err
	}
	if start != "" {
		if err := checkName(start, ArgInvalidRev); err != nil {
			return "", err
		}
	}
	return r.branchTrackFlag(ctx, dir, start)
}

// CreateBranch creates a branch without switching to it. An empty start
// means HEAD.
func (r *Runner) CreateBranch(ctx context.Context, dir, name, start string) error {
	track, err := r.branchNewArgs(ctx, dir, name, start)
	if err != nil {
		return err
	}
	if start == "" {
		_, born, err := r.Head(ctx, dir)
		if err != nil {
			return err
		}
		if !born {
			return &Error{Kind: KindUnbornBranch, Args: []string{"branch", name}, ExitCode: -1}
		}
	}
	args := []string{"branch", track, endOfOptions, name}
	if start != "" {
		args = append(args, start)
	}
	_, err = r.Run(ctx, Cmd{Dir: dir, Args: args})
	return err
}

// CreateAndSwitch creates a branch and switches to it. An empty start means
// HEAD. Before the first commit it renames the branch about to be born.
func (r *Runner) CreateAndSwitch(ctx context.Context, dir, name, start string) error {
	track, err := r.branchNewArgs(ctx, dir, name, start)
	if err != nil {
		return err
	}
	// The name is the value of --create, and start cannot begin with "-"
	// (checkName), so neither can be read as an option.
	args := []string{"switch", "--quiet", "--no-guess", track, "--create", name}
	if start != "" {
		args = append(args, start)
	}
	_, err = r.Run(ctx, Cmd{Dir: dir, Args: args})
	return err
}

// SwitchBranch checks out a branch.
//
// A name that is not a local branch but exists on exactly one remote creates
// a local branch tracking it. git does the same by guessing; here it is done
// explicitly, so checkout.guess and checkout.defaultRemote have no say. A
// name found on several remotes is refused with kind KindAmbiguousRef.
//
// Uncommitted changes that the switch would overwrite stop it
// (KindDirtyWorktree) and are left untouched.
func (r *Runner) SwitchBranch(ctx context.Context, dir, name string) error {
	if err := checkName(name, ArgInvalidBranch); err != nil {
		return err
	}
	exists, err := r.branchExists(ctx, dir, name)
	if err != nil {
		return err
	}
	// name cannot start with "-" (checkName), so it cannot be read as an option.
	args := []string{"switch", "--quiet", "--no-guess", name}
	if !exists {
		candidates, err := r.branchRemoteCandidates(ctx, dir, name)
		if err != nil {
			return err
		}
		switch len(candidates) {
		case 0:
			// Let git say what is wrong with the name.
		case 1:
			args = []string{"switch", "--quiet", "--no-guess", "--track", "--create", name, candidates[0]}
		default:
			return &Error{Kind: KindAmbiguousRef, Args: []string{"switch", name}, ExitCode: -1}
		}
	}
	_, err = r.Run(ctx, Cmd{Dir: dir, Args: args})
	return err
}

// branchRemoteCandidates returns the remote-tracking refs that name could
// stand for: refs/remotes/<remote>/<name> for each configured remote.
func (r *Runner) branchRemoteCandidates(ctx context.Context, dir, name string) ([]string, error) {
	remotes, err := r.remoteNames(ctx, dir)
	if err != nil {
		return nil, err
	}
	var candidates []string
	for _, remote := range remotes {
		ref := branchRemotes + remote + "/" + name
		_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"show-ref", "--verify", "--quiet", ref}})
		if err == nil {
			candidates = append(candidates, ref)
			continue
		}
		if code, exited := ExitCode(err); !exited || code != 1 {
			return nil, err
		}
	}
	return candidates, nil
}

// RenameBranch renames a local branch. It never overwrites an existing one.
func (r *Runner) RenameBranch(ctx context.Context, dir, oldName, newName string) error {
	if err := checkName(oldName, ArgInvalidBranch); err != nil {
		return err
	}
	if err := r.ValidateBranchName(ctx, dir, newName); err != nil {
		return err
	}
	if err := r.branchRequire(ctx, dir, oldName); err != nil {
		return err
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"branch", "--move", endOfOptions, oldName, newName}})
	return err
}

// DeleteBranch deletes a local branch.
//
// Without force, a branch with commits that are merged nowhere is kept
// (KindBranchNotMerged). With force it is deleted anyway and those commits
// become unreachable; deciding when that is acceptable, and keeping a backup,
// is the safety layer's job. The checked-out branch cannot be deleted
// (KindBranchCheckedOut).
func (r *Runner) DeleteBranch(ctx context.Context, dir, name string, force bool) error {
	if err := checkName(name, ArgInvalidBranch); err != nil {
		return err
	}
	if err := r.branchRequire(ctx, dir, name); err != nil {
		return err
	}
	args := []string{"branch", "--delete"}
	if force {
		args = append(args, "--force")
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: append(args, endOfOptions, name)})
	return err
}

// MergedBranches returns the local branches whose commits are all contained
// in into (HEAD when empty): the ones that can be deleted without losing
// anything. The target itself and the checked-out branch are left out.
func (r *Runner) MergedBranches(ctx context.Context, dir, into string) ([]string, error) {
	target := "HEAD"
	if into != "" {
		if err := checkName(into, ArgInvalidRev); err != nil {
			return nil, err
		}
		target = into
	} else {
		_, born, err := r.Head(ctx, dir)
		if err != nil || !born {
			return nil, err
		}
	}
	current, err := r.CurrentBranch(ctx, dir)
	if err != nil {
		return nil, err
	}
	// The target is the value of a single "--merged=<rev>" argument.
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{
		"for-each-ref", "--sort=refname", "--format=%(refname)", "--merged=" + target, branchHeads,
	}})
	if err != nil {
		return nil, err
	}
	var merged []string
	for _, ref := range strings.Fields(string(res.Stdout)) {
		name := strings.TrimPrefix(ref, branchHeads)
		if name == current || name == into || ref == into {
			continue
		}
		merged = append(merged, name)
	}
	return merged, nil
}

// SetUpstream makes a local branch track upstream, e.g. "origin/main".
func (r *Runner) SetUpstream(ctx context.Context, dir, branch, upstream string) error {
	if err := checkName(branch, ArgInvalidBranch); err != nil {
		return err
	}
	if err := checkName(upstream, ArgInvalidBranch); err != nil {
		return err
	}
	if err := r.branchRequire(ctx, dir, branch); err != nil {
		return err
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{
		"branch", "--set-upstream-to=" + upstream, endOfOptions, branch,
	}})
	return err
}

// UnsetUpstream makes a local branch track nothing. The error has kind
// KindNoUpstream when it already tracks nothing.
func (r *Runner) UnsetUpstream(ctx context.Context, dir, branch string) error {
	if err := checkName(branch, ArgInvalidBranch); err != nil {
		return err
	}
	if err := r.branchRequire(ctx, dir, branch); err != nil {
		return err
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"branch", "--unset-upstream", endOfOptions, branch}})
	return err
}
