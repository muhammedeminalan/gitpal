package git

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eminalan/gitpal/internal/core/i18n"
)

// Real git error output (git 2.30–2.54) mapped to the kind gitpal must
// recognize. Order matters where messages overlap: a stale lease is also
// "[rejected]", and a publickey failure also says it "could not read from
// remote repository".
func TestClassifyRecognizesCommonFailures(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   ErrorKind
	}{
		{"not a repo", "fatal: not a git repository (or any of the parent directories): .git", KindNotARepo},
		{"nothing staged", "nothing to commit, working tree clean", KindNothingToCommit},
		{"nothing added", "no changes added to commit (use \"git add\" and/or \"git commit -a\")", KindNothingToCommit},
		{"no identity", "Author identity unknown\n\n*** Please tell me who you are.\n\nRun\n\n  git config --global user.email \"you@example.com\"", KindIdentityMissing},
		{"no name with useConfigOnly", "fatal: no name was given and auto-detection is disabled", KindIdentityMissing},
		{"no email with useConfigOnly", "fatal: no email was given and auto-detection is disabled", KindIdentityMissing},
		{"https auth", "remote: Invalid username or password.\nfatal: Authentication failed for 'https://github.com/a/b.git/'", KindAuthFailed},
		{"prompts disabled", "fatal: could not read Username for 'https://github.com': terminal prompts disabled", KindAuthFailed},
		{"ssh publickey", "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights", KindAuthFailed},
		{"http 403", "fatal: unable to access 'https://github.com/a/b.git/': The requested URL returned error: 403", KindAuthFailed},
		{"dns", "fatal: unable to access 'https://nohost.invalid/a.git/': Could not resolve host: nohost.invalid", KindRemoteUnreachable},
		{"ssh resolve", "ssh: Could not resolve hostname nohost.invalid: nodename nor servname provided, or not known\nfatal: Could not read from remote repository.", KindRemoteUnreachable},
		{"missing path remote", "fatal: '/tmp/nope' does not appear to be a git repository\nfatal: Could not read from remote repository.", KindRemoteUnreachable},
		{"no push destination", "fatal: No configured push destination.\nEither specify the URL from the command-line or configure a remote repository using", KindNoRemote},
		{"no such remote", "error: No such remote 'upstream'", KindNoRemote},
		{"non fast forward", " ! [rejected]        main -> main (non-fast-forward)\nerror: failed to push some refs to '/tmp/r'\nhint: Updates were rejected because the tip of your current branch is behind", KindRejected},
		{"fetch first", " ! [rejected]        main -> main (fetch first)\nerror: failed to push some refs to '/tmp/r'\nhint: Updates were rejected because the remote contains work that you do not", KindRejected},
		{"stale lease", " ! [rejected]        main -> main (stale info)\nerror: failed to push some refs to '/tmp/r'", KindStaleLease},
		{"remote hook declined", " ! [remote rejected] main -> main (pre-receive hook declined)\nerror: failed to push some refs to '/tmp/r'", KindRemoteDeclined},
		{"protected branch", "remote: error: GH006: Protected branch update failed for refs/heads/main.\n ! [remote rejected] main -> main (protected branch hook declined)", KindRemoteDeclined},
		{"push without upstream", "fatal: The current branch feat has no upstream branch.\nTo push the current branch and set the remote as upstream, use", KindNoUpstream},
		{"pull without upstream", "There is no tracking information for the current branch.\nPlease specify which branch you want to merge with.", KindNoUpstream},
		{"ff-only diverged", "hint: Diverging branches can't be fast-forwarded, you need to either:\nfatal: Not possible to fast-forward, aborting.", KindDiverged},
		{"must reconcile", "hint: You have divergent branches and need to specify how to reconcile them.\nfatal: Need to specify how to reconcile divergent branches.", KindDiverged},
		{"local changes overwritten", "error: Your local changes to the following files would be overwritten by checkout:\n\ta.txt\nPlease commit your changes or stash them before you switch branches.", KindDirtyWorktree},
		{"untracked overwritten", "error: The following untracked working tree files would be overwritten by merge:\n\tb.txt", KindDirtyWorktree},
		{"rebase with unstaged", "error: cannot pull with rebase: You have unstaged changes.\nerror: please commit or stash them.", KindDirtyWorktree},
		{"unmerged files", "error: Pulling is not possible because you have unmerged files.\nhint: Fix them up in the work tree", KindConflict},
		{"resolve index", "error: you need to resolve your current index first", KindConflict},
		{"merge in progress", "fatal: You have not concluded your merge (MERGE_HEAD exists).\nPlease, commit your changes before you merge.", KindOperationInProgress},
		{"rebase in progress", "fatal: It seems that there is already a rebase-merge directory, and\nI wonder if you are in the middle of another rebase.", KindOperationInProgress},
		{"cherry-pick in progress", "error: cherry-pick is already in progress", KindOperationInProgress},
		{"detached push", "fatal: You are not currently on a branch.\nTo push the history leading to the current (detached HEAD)", KindDetachedHead},
		{"unborn", "fatal: your current branch 'main' does not have any commits yet", KindUnbornBranch},
		{"branch exists", "fatal: a branch named 'feat' already exists", KindBranchExists},
		{"branch not merged", "error: the branch 'feat' is not fully merged.\nIf you are sure you want to delete it, run 'git branch -D feat'.", KindBranchNotMerged},
		{"invalid branch name", "fatal: 'bad..name' is not a valid branch name", KindInvalidName},
		{"pathspec add", "fatal: pathspec 'nope.txt' did not match any files", KindPathspecNoMatch},
		{"pathspec restore", "error: pathspec 'nope.txt' did not match any file(s) known to git", KindPathspecNoMatch},
		{"invalid reference", "fatal: invalid reference: nope", KindRefNotFound},
		{"branch not found", "error: branch 'nope' not found.", KindRefNotFound},
		{"unknown revision", "fatal: ambiguous argument 'nope': unknown revision or path not in the working tree.", KindRefNotFound},
		{"remote ref", "fatal: couldn't find remote ref nope", KindRefNotFound},
		{"index lock", "fatal: Unable to create '/r/.git/index.lock': File exists.\n\nAnother git process seems to be running in this repository", KindIndexLocked},
		{"unrecognized", "fatal: something nobody has seen before", KindUnknown},
		{"empty", "", KindUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, classify(tt.stderr))
		})
	}
}

// Every kind gitpal can produce must explain itself to the user in both
// languages; a missing entry would show a raw catalog key.
func TestEveryKindHasAnExplanation(t *testing.T) {
	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			key, ok := kindMessages[kind]
			require.True(t, ok, "no message for kind %q", kind)
			for _, lang := range []i18n.Lang{i18n.TR, i18n.EN} {
				text := i18n.T(lang, key)
				assert.NotEqual(t, string(key), text, "%s: key %q is not in the catalog", lang, key)
			}
		})
	}
}

func TestAllKindsListsEveryClassifiedKind(t *testing.T) {
	for _, entry := range stderrKinds {
		assert.Contains(t, allKinds, entry.kind)
	}
}

func TestErrorLocalizeExplainsAndKeepsGitOutput(t *testing.T) {
	err := &Error{
		Kind:     KindRejected,
		Args:     []string{"push", "origin", "main"},
		ExitCode: 1,
		Stderr:   " ! [rejected] main -> main (fetch first)",
	}

	tr := i18n.Describe(i18n.TR, fmt.Errorf("push: %w", err))
	en := i18n.Describe(i18n.EN, err)

	assert.Contains(t, tr, i18n.T(i18n.TR, kindMessages[KindRejected]))
	assert.Contains(t, en, i18n.T(i18n.EN, kindMessages[KindRejected]))
	assert.Contains(t, tr, "(fetch first)", "git's own words stay visible: hook output and details matter")
	assert.NotEqual(t, tr, en)
}

func TestErrorLocalizeForUnknownKindNamesTheCommand(t *testing.T) {
	err := &Error{Kind: KindUnknown, Args: []string{"merge", "topic"}, ExitCode: 3, Stderr: "fatal: odd"}

	text := err.Localize(i18n.TR)

	assert.Contains(t, text, "git merge topic")
	assert.Contains(t, text, "3")
	assert.Contains(t, text, "fatal: odd")
}

func TestErrorLocalizeWithoutOutputIsJustTheExplanation(t *testing.T) {
	err := &Error{Kind: KindTimeout, Args: []string{"fetch"}, ExitCode: -1}

	assert.Equal(t, i18n.T(i18n.EN, kindMessages[KindTimeout]), err.Localize(i18n.EN))
}

func TestErrorLocalizeTruncatesLongOutput(t *testing.T) {
	err := &Error{Kind: KindHookFailed, Args: []string{"commit"}, ExitCode: 1, Stderr: strings.Repeat("lint error\n", 1000)}

	assert.Less(t, len(err.Localize(i18n.EN)), maxErrorText+500)
}

func TestArgErrorLocalizes(t *testing.T) {
	for _, code := range allArgCodes {
		t.Run(string(code), func(t *testing.T) {
			err := &ArgError{Code: code, Value: "x"}
			for _, lang := range []i18n.Lang{i18n.TR, i18n.EN} {
				text := err.Localize(lang)
				assert.NotContains(t, text, "git.arg.", "%s: missing catalog entry", lang)
			}
			assert.Equal(t, err.Localize(i18n.EN), err.Error())
		})
	}
}

func TestArgErrorIsMatchable(t *testing.T) {
	err := fmt.Errorf("stage: %w", &ArgError{Code: ArgNoPaths})

	var argErr *ArgError
	require.True(t, errors.As(err, &argErr))
	assert.Equal(t, ArgNoPaths, argErr.Code)
	assert.Equal(t, KindUnknown, KindOf(err), "argument errors are not git failures")
}
