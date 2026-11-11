package git

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
)

// Remote is a configured remote repository.
type Remote struct {
	Name string
	// FetchURL and PushURL are the addresses git will really use, after
	// url.<base>.insteadOf rewriting, with any embedded credentials removed.
	FetchURL, PushURL string
}

// Remotes lists the configured remotes, sorted by name.
//
// The URLs come from `git remote get-url` rather than from reading the
// configuration: that is the only form that applies insteadOf rewriting, and
// it needs no parsing of remote names, which may contain dots and slashes.
// They go through redact, so a token written into a URL never reaches a
// caller.
func (r *Runner) Remotes(ctx context.Context, dir string) ([]Remote, error) {
	names, err := r.remoteNames(ctx, dir)
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	remotes := make([]Remote, 0, len(names))
	for _, name := range names {
		remote := Remote{Name: name}
		if remote.FetchURL, err = r.remoteURL(ctx, dir, name, false); err != nil {
			return nil, err
		}
		if remote.PushURL, err = r.remoteURL(ctx, dir, name, true); err != nil {
			return nil, err
		}
		remotes = append(remotes, remote)
	}
	return remotes, nil
}

func (r *Runner) remoteURL(ctx context.Context, dir, name string, push bool) (string, error) {
	args := []string{"remote", "get-url"}
	if push {
		args = append(args, "--push")
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: append(args, endOfOptions, name)})
	if err != nil {
		return "", err
	}
	return redact(strings.TrimSuffix(string(res.Stdout), "\n")), nil
}

// remoteRequire checks a user-supplied remote name and returns a KindNoRemote
// error when no remote with that name is configured. A remote is always a
// name here, never a URL.
func (r *Runner) remoteRequire(ctx context.Context, dir, name string) error {
	if err := checkName(name, ArgInvalidRemote); err != nil {
		return err
	}
	names, err := r.remoteNames(ctx, dir)
	if err != nil {
		return err
	}
	if !slices.Contains(names, name) {
		return &Error{Kind: KindNoRemote, Args: []string{"remote", "get-url", name}, ExitCode: -1}
	}
	return nil
}

// FetchOptions adjusts Fetch.
type FetchOptions struct {
	// Remote is the remote to fetch from; empty means git's default for the
	// current branch.
	Remote string
	// All fetches from every remote; Remote is then ignored.
	All bool
	// Prune removes remote-tracking branches that no longer exist on the
	// remote. When false, the user's fetch.prune setting decides.
	Prune bool
}

// Fetch downloads new commits and updates the remote-tracking branches. It
// never changes a local branch or the working tree.
func (r *Runner) Fetch(ctx context.Context, dir string, opts FetchOptions) error {
	args := []string{"fetch", "--quiet"}
	if opts.Prune {
		args = append(args, "--prune")
	}
	switch {
	case opts.All:
		args = append(args, "--all")
	case opts.Remote != "":
		if err := r.remoteRequire(ctx, dir, opts.Remote); err != nil {
			return err
		}
		args = append(args, endOfOptions, opts.Remote)
	default:
		names, err := r.remoteNames(ctx, dir)
		if err != nil {
			return err
		}
		if len(names) == 0 {
			return &Error{Kind: KindNoRemote, Args: []string{"fetch"}, ExitCode: -1}
		}
	}
	_, err := r.Run(ctx, networkCmd(dir, args...))
	return err
}

// PullStrategy says how Pull combines local and remote commits. The values
// are those of gitpal's pull.strategy setting.
type PullStrategy string

// Pull strategies.
const (
	// PullMerge creates a merge commit when both sides have new commits.
	PullMerge PullStrategy = "merge"
	// PullRebase replays the local commits on top of the remote ones.
	PullRebase PullStrategy = "rebase"
	// PullFFOnly only moves the branch forward; it refuses when both sides
	// have new commits. It never creates or rewrites a commit.
	PullFFOnly PullStrategy = "ff-only"
)

// PullOptions adjusts Pull.
type PullOptions struct {
	// Strategy defaults to PullFFOnly, the one that cannot surprise.
	Strategy PullStrategy
	// Remote and Branch name the remote branch to pull. Set both, or neither
	// to pull from the current branch's upstream.
	Remote, Branch string
}

// PullResult says what a pull did.
type PullResult struct {
	// Before and After are HEAD before and after the pull.
	Before, After string
	// Updated is set when HEAD moved.
	Updated bool
}

// pullStrategyFlags spell each strategy out completely, so that nothing in
// the user's configuration can turn it into another one: pull.rebase and
// branch.<name>.rebase (--rebase / --no-rebase; plain --rebase also rules out
// "interactive" and "merges"), pull.ff and merge.ff (--ff / --ff-only), and
// branch.<name>.mergeOptions (--no-squash, --commit).
//
// --no-rebase next to --ff-only and --ff next to --rebase look redundant with
// a current git; before 2.34 pull.rebase and pull.ff still interfered there.
var pullStrategyFlags = map[PullStrategy][]string{
	PullFFOnly: {"--no-rebase", "--ff-only", "--no-squash", "--commit"},
	PullMerge:  {"--no-rebase", "--ff", "--no-squash", "--commit", "--no-edit"},
	PullRebase: {"--rebase", "--ff"},
}

// Pull fetches and integrates the remote branch into the current branch.
//
// `--no-autostash` is always passed: uncommitted changes that are in the way
// stop the pull (KindDirtyWorktree) instead of being stashed without the user
// knowing, whatever rebase.autoStash and merge.autoStash say.
//
// When the pull stops on conflicts the error has kind KindConflict; that is
// decided by looking at the repository, not at git's output. Both sides
// having new commits under PullFFOnly is KindDiverged.
func (r *Runner) Pull(ctx context.Context, dir string, opts PullOptions) (PullResult, error) {
	strategy := opts.Strategy
	if strategy == "" {
		strategy = PullFFOnly
	}
	flags, ok := pullStrategyFlags[strategy]
	if !ok {
		return PullResult{}, &ArgError{Code: ArgInvalidStrategy, Value: string(strategy)}
	}
	args := append([]string{"pull", "--quiet", "--no-autostash"}, flags...)

	branch, err := r.CurrentBranch(ctx, dir)
	if err != nil {
		return PullResult{}, err
	}
	if branch == "" {
		return PullResult{}, &Error{Kind: KindDetachedHead, Args: []string{"pull"}, ExitCode: -1}
	}

	switch {
	case opts.Remote != "" && opts.Branch != "":
		if err := checkName(opts.Branch, ArgInvalidBranch); err != nil {
			return PullResult{}, err
		}
		if err := r.remoteRequire(ctx, dir, opts.Remote); err != nil {
			return PullResult{}, err
		}
		// Neither can be read as an option: the remote passed checkName, and
		// the branch is spelled as a full ref.
		args = append(args, opts.Remote, branchHeads+opts.Branch)
	case opts.Remote != "":
		return PullResult{}, &ArgError{Code: ArgInvalidBranch}
	case opts.Branch != "":
		return PullResult{}, &ArgError{Code: ArgInvalidRemote}
	default:
		_, hasUpstream, err := r.Upstream(ctx, dir, branch)
		if err != nil {
			return PullResult{}, err
		}
		if !hasUpstream {
			return PullResult{}, &Error{Kind: KindNoUpstream, Args: []string{"pull"}, ExitCode: -1}
		}
	}

	before, _, err := r.Head(ctx, dir)
	if err != nil {
		return PullResult{}, err
	}
	if _, err := r.Run(ctx, networkCmd(dir, args...)); err != nil {
		var gerr *Error
		if errorsAs(err, &gerr) && gerr.ExitCode >= 0 && r.remoteStoppedOnConflict(ctx, dir) {
			gerr.Kind = KindConflict
		}
		return PullResult{}, err
	}
	after, _, err := r.Head(ctx, dir)
	if err != nil {
		return PullResult{}, err
	}
	return PullResult{Before: before, After: after, Updated: before != after}, nil
}

// remoteStoppedOnConflict reports whether a merge or rebase is waiting for
// conflicts to be resolved. It answers false when it cannot tell: the caller
// is already reporting another error.
func (r *Runner) remoteStoppedOnConflict(ctx context.Context, dir string) bool {
	st, err := r.Status(ctx, dir, StatusOptions{Untracked: UntrackedNone})
	if err != nil {
		return false
	}
	return len(st.Conflicted()) > 0 && (st.Operation == OpMerge || st.Operation == OpRebase)
}

// PushOptions adjusts Push. There is deliberately no way to ask for a plain
// forced push.
type PushOptions struct {
	// Remote is the remote to push to. Empty means the branch's push remote
	// (branch.<name>.pushRemote, remote.pushDefault), then its upstream's
	// remote, then "origin".
	Remote string
	// Branch is the local branch to push; empty means the current one.
	Branch string
	// SetUpstream makes the local branch track the pushed branch.
	SetUpstream bool
	// ForceWithLease allows replacing commits on the remote, but only if the
	// remote branch is still where the last fetch saw it and everything
	// fetched from it has been taken into account. Without it, a push that is
	// not a fast-forward is refused.
	ForceWithLease bool
	// ExpectedRemote, with ForceWithLease, is the commit the remote branch
	// must still be at, instead of what the last fetch saw.
	ExpectedRemote string
}

// PushStatus says what a push did to the remote branch.
type PushStatus string

// Push statuses.
const (
	PushNew         PushStatus = "new"
	PushFastForward PushStatus = "fast-forward"
	PushForced      PushStatus = "forced"
	PushUpToDate    PushStatus = "up-to-date"
)

// PushResult says what a push did.
type PushResult struct {
	Remote string
	// Branch is the local branch, RemoteBranch the branch it was pushed to.
	Branch, RemoteBranch string
	Status               PushStatus
	// Before and After are the commits the remote branch pointed at before
	// and after the push. Before is empty for a new branch; it is a full hash
	// when that commit is known locally, otherwise as abbreviated by git.
	Before, After string
}

// Push sends one local branch to a remote.
//
// Exactly one branch is pushed, with an explicit refspec, and tags are not
// sent along: push.default, push.followTags and remote.<name>.push cannot
// widen it. The remote branch is the upstream's branch when the local branch
// tracks one on that remote, otherwise a branch of the same name.
//
// Rejections are read from git's machine-readable output: KindRejected when
// the remote has commits the local branch lacks, KindStaleLease when a lease
// did not hold, KindRemoteDeclined when the server refused. A local pre-push
// hook that refuses gives KindHookFailed.
func (r *Runner) Push(ctx context.Context, dir string, opts PushOptions) (PushResult, error) {
	branch := opts.Branch
	if branch == "" {
		current, err := r.CurrentBranch(ctx, dir)
		if err != nil {
			return PushResult{}, err
		}
		if current == "" {
			return PushResult{}, &Error{Kind: KindDetachedHead, Args: []string{"push"}, ExitCode: -1}
		}
		branch = current
	} else if err := checkName(branch, ArgInvalidBranch); err != nil {
		return PushResult{}, err
	}
	if opts.ForceWithLease && opts.ExpectedRemote != "" {
		if err := checkName(opts.ExpectedRemote, ArgInvalidRev); err != nil {
			return PushResult{}, err
		}
	}
	if opts.Remote != "" {
		if err := checkName(opts.Remote, ArgInvalidRemote); err != nil {
			return PushResult{}, err
		}
	}
	if err := r.branchRequire(ctx, dir, branch); err != nil {
		return PushResult{}, err
	}

	remote, remoteBranch, err := r.pushDestination(ctx, dir, branch, opts.Remote)
	if err != nil {
		return PushResult{}, err
	}
	if err := r.remoteRequire(ctx, dir, remote); err != nil {
		return PushResult{}, err
	}
	after, err := r.resolveCommit(ctx, dir, branchHeads+branch)
	if err != nil {
		return PushResult{}, err
	}

	remoteRef := branchHeads + remoteBranch
	args := []string{"push", "--porcelain", "--no-follow-tags"}
	if opts.SetUpstream {
		args = append(args, "--set-upstream")
	}
	if opts.ForceWithLease {
		lease := "--force-with-lease=" + remoteRef
		if opts.ExpectedRemote != "" {
			lease += ":" + opts.ExpectedRemote
		}
		// --force-if-includes (git 2.30) also refuses when commits fetched
		// from the remote branch were never taken into the local one: a
		// background fetch must not make a lease pass by itself.
		args = append(args, lease, "--force-if-includes")
	}
	args = append(args, endOfOptions, remote, branchHeads+branch+":"+remoteRef)

	result := PushResult{Remote: remote, Branch: branch, RemoteBranch: remoteBranch}
	res, runErr := r.Run(ctx, networkCmd(dir, args...))
	line, found := pushParse(res.Stdout, remoteRef)

	if runErr != nil || (found && line.flag == '!') {
		gerr := &Error{Kind: KindUnknown, Args: redactAll(args), ExitCode: res.ExitCode, Stderr: redact(strings.TrimSpace(string(res.Stderr)))}
		if runErr != nil && !errorsAs(runErr, &gerr) {
			return PushResult{}, runErr
		}
		switch {
		case found && line.flag == '!':
			gerr.Kind = line.rejectionKind()
		case gerr.Kind == KindUnknown && gerr.ExitCode >= 0 && r.hookInstalled(ctx, dir, "pre-push"):
			gerr.Kind = KindHookFailed
		}
		return PushResult{}, gerr
	}
	if !found {
		return PushResult{}, errors.New("git push: no status reported for the pushed branch")
	}

	result.After = after
	switch line.flag {
	case '*':
		result.Status = PushNew
	case '=':
		result.Status, result.Before = PushUpToDate, after
	case '+':
		result.Status, result.Before = PushForced, r.pushOldCommit(ctx, dir, line.summary)
	default:
		result.Status, result.Before = PushFastForward, r.pushOldCommit(ctx, dir, line.summary)
	}
	return result, nil
}

// pushDestination works out where a branch is pushed: the remote (unless one
// was given) and the name of the branch on it.
func (r *Runner) pushDestination(ctx context.Context, dir, branch, remote string) (string, string, error) {
	upstreamRemote, _, err := r.ConfigGet(ctx, dir, "branch."+branch+".remote")
	if err != nil {
		return "", "", err
	}
	if remote == "" {
		for _, key := range []string{"branch." + branch + ".pushRemote", "remote.pushDefault"} {
			if remote, _, err = r.ConfigGet(ctx, dir, key); err != nil {
				return "", "", err
			}
			if remote != "" {
				break
			}
		}
	}
	// "." as the upstream remote means the upstream is a local branch.
	if remote == "" && upstreamRemote != "." {
		remote = upstreamRemote
	}
	if remote == "" {
		remote = "origin"
	}

	remoteBranch := branch
	if upstreamRemote == remote {
		merge, _, err := r.ConfigGet(ctx, dir, "branch."+branch+".merge")
		if err != nil {
			return "", "", err
		}
		if name, ok := strings.CutPrefix(merge, branchHeads); ok && name != "" {
			remoteBranch = name
		}
	}
	return remote, remoteBranch, nil
}

// pushLine is one ref line of `git push --porcelain`:
// "<flag>\t<from>:<to>\t<summary>[ (<reason>)]".
type pushLine struct {
	flag    byte
	summary string
}

// pushParse finds the line for the pushed remote ref.
func pushParse(stdout []byte, remoteRef string) (pushLine, bool) {
	for raw := range strings.SplitSeq(string(stdout), "\n") {
		parts := strings.SplitN(strings.TrimSuffix(raw, "\r"), "\t", 3)
		if len(parts) != 3 || len(parts[0]) != 1 {
			continue
		}
		if _, to, ok := strings.Cut(parts[1], ":"); !ok || to != remoteRef {
			continue
		}
		return pushLine{flag: parts[0][0], summary: parts[2]}, true
	}
	return pushLine{}, false
}

// rejectionKind explains a "!" line.
func (l pushLine) rejectionKind() ErrorKind {
	switch {
	case strings.Contains(l.summary, "(stale info)"),
		strings.Contains(l.summary, "(remote ref updated since checkout)"):
		return KindStaleLease
	case strings.HasPrefix(l.summary, "[remote rejected]"):
		return KindRemoteDeclined
	case strings.Contains(l.summary, "(fetch first)"),
		strings.Contains(l.summary, "(non-fast-forward)"),
		strings.Contains(l.summary, "(already exists)"),
		strings.Contains(l.summary, "(needs force)"):
		return KindRejected
	}
	return KindUnknown
}

// pushOldCommit reads the commit the remote branch was at from a summary such
// as "1a2b3c4..5d6e7f8" or "1a2b3c4...5d6e7f8 (forced update)", as a full
// hash when that commit exists locally.
func (r *Runner) pushOldCommit(ctx context.Context, dir, summary string) string {
	oldAbbrev, _, ok := strings.Cut(summary, "..")
	if !ok || oldAbbrev == "" {
		return ""
	}
	if hash, err := r.resolveCommit(ctx, dir, oldAbbrev); err == nil {
		return hash
	}
	return oldAbbrev
}

// Upstream returns the branch that a local branch tracks, e.g. "origin/main".
// ok is false when it tracks nothing. An empty branch means the current one;
// on a detached HEAD there is none.
func (r *Runner) Upstream(ctx context.Context, dir, branch string) (name string, ok bool, err error) {
	if branch == "" {
		if branch, err = r.CurrentBranch(ctx, dir); err != nil || branch == "" {
			return "", false, err
		}
	} else if err := checkName(branch, ArgInvalidBranch); err != nil {
		return "", false, err
	}

	// for-each-ref knows the remote-tracking ref whatever the remote's fetch
	// refspec maps it to.
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{"for-each-ref", "--format=%(refname) %(upstream)", branchHeads + branch}})
	if err != nil {
		return "", false, err
	}
	for line := range strings.SplitSeq(string(res.Stdout), "\n") {
		ref, upstream, _ := strings.Cut(strings.TrimSuffix(line, "\r"), " ")
		if ref != branchHeads+branch {
			continue
		}
		upstream = strings.TrimPrefix(strings.TrimPrefix(upstream, branchRemotes), branchHeads)
		return upstream, upstream != "", nil
	}

	// A branch without commits is not a ref yet, but it may already be
	// configured to track something (a clone of an empty repository).
	remote, _, err := r.ConfigGet(ctx, dir, "branch."+branch+".remote")
	if err != nil {
		return "", false, err
	}
	merge, _, err := r.ConfigGet(ctx, dir, "branch."+branch+".merge")
	if err != nil {
		return "", false, err
	}
	short, isBranch := strings.CutPrefix(merge, branchHeads)
	switch {
	case remote == "" || !isBranch || short == "":
		return "", false, nil
	case remote == ".":
		return short, true, nil
	}
	return remote + "/" + short, true, nil
}

// AheadBehind counts the commits local has that upstream lacks (ahead) and
// the other way round (behind).
func (r *Runner) AheadBehind(ctx context.Context, dir, local, upstream string) (ahead, behind int, err error) {
	if err := checkName(local, ArgInvalidRev); err != nil {
		return 0, 0, err
	}
	if err := checkName(upstream, ArgInvalidRev); err != nil {
		return 0, 0, err
	}
	res, err := r.Run(ctx, Cmd{Dir: dir, Args: []string{
		"rev-list", "--left-right", "--count", endOfOptions, local + "..." + upstream, "--",
	}})
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(string(res.Stdout))
	if len(fields) != 2 {
		return 0, 0, errors.New("git rev-list: unexpected output")
	}
	if ahead, err = strconv.Atoi(fields[0]); err != nil {
		return 0, 0, errors.New("git rev-list: unexpected output")
	}
	if behind, err = strconv.Atoi(fields[1]); err != nil {
		return 0, 0, errors.New("git rev-list: unexpected output")
	}
	return ahead, behind, nil
}
