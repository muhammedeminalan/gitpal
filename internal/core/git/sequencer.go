package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// stoppedOnConflict reports whether an operation is waiting for conflicts to
// be resolved. It answers false when it cannot tell: the caller is already
// reporting another error.
func (r *Runner) stoppedOnConflict(ctx context.Context, dir string) bool {
	st, err := r.Status(ctx, dir, StatusOptions{Untracked: UntrackedNone})
	return err == nil && len(st.Conflicted()) > 0
}

// conflictKind turns a failure that left conflicts behind into KindConflict.
// The repository decides, not git's wording.
func (r *Runner) conflictKind(ctx context.Context, dir string, err error) error {
	var gerr *Error
	if errorsAs(err, &gerr) && gerr.ExitCode >= 0 && r.stoppedOnConflict(ctx, dir) {
		gerr.Kind = KindConflict
	}
	return err
}

// MergeMode chooses how a merge records its result.
type MergeMode string

// Merge modes.
const (
	// MergeAuto fast-forwards when it can and creates a merge commit
	// otherwise.
	MergeAuto MergeMode = ""
	// MergeNoFF always creates a merge commit.
	MergeNoFF MergeMode = "no-ff"
	// MergeFFOnly only fast-forwards and refuses otherwise.
	MergeFFOnly MergeMode = "ff-only"
	// MergeSquash stages the combined changes without committing them.
	MergeSquash MergeMode = "squash"
)

// MergeOptions describes a merge.
type MergeOptions struct {
	// Rev is the branch or commit to merge into the current branch.
	Rev  string
	Mode MergeMode
	// Message replaces git's default merge commit message.
	Message string
}

// MergeResult says what a merge did.
type MergeResult struct {
	Before, After string
	UpToDate      bool
	FastForward   bool
	// Squashed is set for MergeSquash: the changes are staged, not
	// committed.
	Squashed bool
}

// mergeModeFlags spell every mode out, so merge.ff and branch.<name>.
// mergeOptions cannot turn one into another.
var mergeModeFlags = map[MergeMode][]string{
	MergeAuto:   {"--ff", "--no-squash"},
	MergeNoFF:   {"--no-ff", "--no-squash"},
	MergeFFOnly: {"--ff-only", "--no-squash"},
	MergeSquash: {"--squash"},
}

// Merge merges a branch into the current one. It stops with kind
// KindConflict when there are conflicts to resolve, and never stashes
// uncommitted changes behind the user's back.
func (r *Runner) Merge(ctx context.Context, dir string, opts MergeOptions) (MergeResult, error) {
	if err := checkName(opts.Rev, ArgInvalidRev); err != nil {
		return MergeResult{}, err
	}
	flags, ok := mergeModeFlags[opts.Mode]
	if !ok {
		return MergeResult{}, &ArgError{Code: ArgInvalidStrategy, Value: string(opts.Mode)}
	}
	before, _, err := r.Head(ctx, dir)
	if err != nil {
		return MergeResult{}, err
	}
	args := append([]string{"merge", "--quiet", "--no-edit", "--no-autostash", "--no-log", "--no-stat"}, flags...)
	if opts.Message != "" {
		args = append(args, "-m", opts.Message)
	}
	args = append(args, endOfOptions, opts.Rev)
	if _, err := r.Run(ctx, Cmd{Dir: dir, Args: args}); err != nil {
		return MergeResult{}, r.conflictKind(ctx, dir, err)
	}
	after, _, err := r.Head(ctx, dir)
	if err != nil {
		return MergeResult{}, err
	}
	res := MergeResult{Before: before, After: after, Squashed: opts.Mode == MergeSquash}
	switch {
	case res.Squashed:
	case before == after:
		res.UpToDate = true
	default:
		c, err := r.CommitInfo(ctx, dir, after)
		if err != nil {
			return MergeResult{}, err
		}
		res.FastForward = !c.IsMerge()
	}
	return res, nil
}

// CherryPick applies the changes of the given commits (or ranges such as
// "a..b") on top of the current branch. As in git, several revisions form a
// single walk: "a..b c" also takes everything reachable from c.
func (r *Runner) CherryPick(ctx context.Context, dir string, revs []string) error {
	return r.sequence(ctx, dir, "cherry-pick", []string{"--no-edit"}, revs)
}

// Revert adds commits that undo the given ones. mainline picks the parent
// to undo against when reverting a merge (1 is the branch merged into).
func (r *Runner) Revert(ctx context.Context, dir string, revs []string, mainline int) error {
	flags := []string{"--no-edit"}
	if mainline > 0 {
		flags = append(flags, "--mainline="+strconv.Itoa(mainline))
	}
	return r.sequence(ctx, dir, "revert", flags, revs)
}

func (r *Runner) sequence(ctx context.Context, dir, command string, flags, revs []string) error {
	if len(revs) == 0 {
		return &ArgError{Code: ArgInvalidRev}
	}
	args := append([]string{command}, flags...)
	args = append(args, endOfOptions)
	for _, rev := range revs {
		if err := checkName(rev, ArgInvalidRev); err != nil {
			return err
		}
		args = append(args, rev)
	}
	_, err := r.Run(ctx, Cmd{Dir: dir, Args: args})
	if err != nil {
		return r.conflictKind(ctx, dir, err)
	}
	return nil
}

// operationCommand is the git command that drives each operation.
var operationCommand = map[Operation]string{
	OpMerge: "merge", OpRebase: "rebase", OpAm: "am", OpCherryPick: "cherry-pick", OpRevert: "revert",
}

// errNoOperation is returned when there is nothing to continue or abort.
var errNoOperation = &Error{Kind: KindUnknown, Args: []string{"status"}, ExitCode: -1, Err: errors.New("no operation in progress")}

// ErrNoOperation reports that no merge, rebase, cherry-pick or revert is
// under way.
func ErrNoOperation(err error) bool { return errors.Is(err, errNoOperation.Err) }

func (r *Runner) operationStep(ctx context.Context, dir, step string) (Operation, error) {
	op, err := r.InProgress(ctx, dir)
	if err != nil {
		return op, err
	}
	command := operationCommand[op]
	if command == "" || (step == "--skip" && op == OpMerge) {
		return op, errNoOperation
	}
	_, err = r.Run(ctx, Cmd{Dir: dir, Args: []string{command, step}})
	if err != nil {
		return op, r.conflictKind(ctx, dir, err)
	}
	return op, nil
}

// AbortOperation gives up the merge, rebase, cherry-pick, revert or am under
// way and returns to the state before it started. Conflict resolutions made
// so far are lost.
func (r *Runner) AbortOperation(ctx context.Context, dir string) (Operation, error) {
	return r.operationStep(ctx, dir, "--abort")
}

// ContinueOperation carries on with the operation under way once its
// conflicts are resolved and staged. It stops again with KindConflict at the
// next conflict.
func (r *Runner) ContinueOperation(ctx context.Context, dir string) (Operation, error) {
	return r.operationStep(ctx, dir, "--continue")
}

// SkipOperation drops the commit the rebase, cherry-pick or revert stopped
// at and carries on.
func (r *Runner) SkipOperation(ctx context.Context, dir string) (Operation, error) {
	return r.operationStep(ctx, dir, "--skip")
}

// RebaseAction is what an interactive rebase does with one commit.
type RebaseAction string

// Rebase actions.
const (
	RebasePick   RebaseAction = "pick"
	RebaseReword RebaseAction = "reword"
	RebaseSquash RebaseAction = "squash"
	RebaseFixup  RebaseAction = "fixup"
	RebaseDrop   RebaseAction = "drop"
)

// RebaseStep is one line of an interactive rebase plan.
type RebaseStep struct {
	Action RebaseAction
	Hash   string
	// Subject is shown to the user; it is not sent to git.
	Subject string
	// Message is the new message for reword, and for a squash the message of
	// the combined commit; empty keeps git's.
	Message string
}

// RebaseOptions describes a rebase.
type RebaseOptions struct {
	// Upstream is what the current branch is replayed onto.
	Upstream string
	// Steps, when set, make the rebase interactive: the commits are replayed
	// in this order with these actions. Start from RebasePlan.
	Steps []RebaseStep
}

// RebaseResult says what a rebase did.
type RebaseResult struct{ Before, After string }

// RebasePlan lists the commits a rebase onto upstream would replay, oldest
// first, each to be picked. Merge commits are left out, as git does.
func (r *Runner) RebasePlan(ctx context.Context, dir, upstream string) ([]RebaseStep, error) {
	if err := checkName(upstream, ArgInvalidRev); err != nil {
		return nil, err
	}
	base, err := r.resolveCommit(ctx, dir, upstream)
	if err != nil {
		return nil, err
	}
	commits, err := r.Log(ctx, dir, LogOptions{Revs: []string{base + "..HEAD"}, NoMerges: true})
	if err != nil {
		return nil, err
	}
	steps := make([]RebaseStep, 0, len(commits))
	for i := len(commits) - 1; i >= 0; i-- {
		steps = append(steps, RebaseStep{Action: RebasePick, Hash: commits[i].Hash, Subject: commits[i].Subject})
	}
	return steps, nil
}

var rebaseActions = map[RebaseAction]bool{
	RebasePick: true, RebaseReword: true, RebaseSquash: true, RebaseFixup: true, RebaseDrop: true,
}

// Rebase replays the current branch onto upstream. With Steps it is an
// interactive rebase that follows them exactly; gitpal answers git's editor
// requests itself, so no editor opens. It stops with KindConflict when a
// commit does not apply cleanly.
func (r *Runner) Rebase(ctx context.Context, dir string, opts RebaseOptions) (RebaseResult, error) {
	if err := checkName(opts.Upstream, ArgInvalidRev); err != nil {
		return RebaseResult{}, err
	}
	before, _, err := r.Head(ctx, dir)
	if err != nil {
		return RebaseResult{}, err
	}
	args := []string{
		"-c", "rebase.abbreviateCommands=false", "-c", "commit.cleanup=strip", "rebase", "--quiet",
		"--no-autostash", "--no-autosquash",
	}
	cmd := Cmd{Dir: dir}
	if opts.Steps != nil {
		tmp, err := os.MkdirTemp("", "gitpal-rebase-")
		if err != nil {
			return RebaseResult{}, err
		}
		defer os.RemoveAll(tmp)
		env, err := r.rebaseEditors(tmp, opts.Steps)
		if err != nil {
			return RebaseResult{}, err
		}
		args = append(args, "--interactive")
		cmd.Env = env
	}
	args = append(args, endOfOptions, opts.Upstream)
	cmd.Args = args
	if _, err := r.Run(ctx, cmd); err != nil {
		return RebaseResult{}, r.conflictKind(ctx, dir, err)
	}
	after, _, err := r.Head(ctx, dir)
	if err != nil {
		return RebaseResult{}, err
	}
	return RebaseResult{Before: before, After: after}, nil
}

// rebaseEditors writes the todo list and the message queue into dir and
// returns the environment that makes git ask gitpal for them.
func (r *Runner) rebaseEditors(dir string, steps []RebaseStep) ([]string, error) {
	var todo strings.Builder
	var messages []string
	chainHasSquash, chainMessage := false, ""
	endChain := func() {
		if chainHasSquash {
			messages = append(messages, chainMessage)
		}
		chainHasSquash, chainMessage = false, ""
	}
	for i, s := range steps {
		if !rebaseActions[s.Action] {
			return nil, &ArgError{Code: ArgInvalidRev, Value: string(s.Action)}
		}
		if err := checkName(s.Hash, ArgInvalidRev); err != nil {
			return nil, err
		}
		folds := s.Action == RebaseSquash || s.Action == RebaseFixup
		if folds && i == 0 {
			return nil, &ArgError{Code: ArgInvalidRev, Value: string(s.Action)}
		}
		if !folds && s.Action != RebaseDrop {
			endChain()
		}
		// git asks for messages in this order: a reword when it is replayed,
		// a squash chain when it ends.
		switch s.Action {
		case RebaseReword:
			messages = append(messages, s.Message)
		case RebaseSquash:
			chainHasSquash = true
			if s.Message != "" {
				chainMessage = s.Message
			}
		case RebasePick, RebaseFixup, RebaseDrop:
		}
		todo.WriteString(string(s.Action) + " " + s.Hash + "\n")
	}
	endChain()

	todoFile := filepath.Join(dir, "todo")
	if err := os.WriteFile(todoFile, []byte(todo.String()), 0o600); err != nil {
		return nil, err
	}
	msgDir := filepath.Join(dir, "messages")
	if err := os.Mkdir(msgDir, 0o700); err != nil {
		return nil, err
	}
	for i, m := range messages {
		if err := os.WriteFile(filepath.Join(msgDir, strconv.Itoa(i+1)), []byte(m), 0o600); err != nil {
			return nil, err
		}
	}
	seqEditor, err := r.editorCommand(editorTodoArg)
	if err != nil {
		return nil, err
	}
	msgEditor, err := r.editorCommand(editorMessageArg)
	if err != nil {
		return nil, err
	}
	return []string{
		"GIT_SEQUENCE_EDITOR=" + seqEditor, "GIT_EDITOR=" + msgEditor,
		envTodoFile + "=" + todoFile, envMessageDir + "=" + msgDir,
	}, nil
}
