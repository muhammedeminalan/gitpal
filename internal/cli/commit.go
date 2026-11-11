package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
)

// commitFlags are shared by commit and save.
type commitFlags struct {
	messages   []string
	amend      bool
	allowEmpty bool
	ai         bool
}

func (f *commitFlags) register(cmd *cobra.Command, a *app, withAmend bool) {
	cmd.Flags().StringArrayVarP(&f.messages, "message", "m", nil, i18n.T(a.lang, i18n.MsgCommitMessage))
	cmd.Flags().BoolVar(&f.ai, "ai", false, i18n.T(a.lang, i18n.MsgCommitAI))
	if withAmend {
		cmd.Flags().BoolVar(&f.amend, "amend", false, i18n.T(a.lang, i18n.MsgCommitAmend))
		cmd.Flags().BoolVar(&f.allowEmpty, "allow-empty", false, i18n.T(a.lang, i18n.MsgCommitAllowEmpty))
	}
}

// message returns the commit message: the -m paragraphs, a suggestion from
// the AI, or what the user types. Amending without one keeps the old message.
func (a *app) message(ctx context.Context, f commitFlags) (string, error) {
	if len(f.messages) > 0 {
		return strings.Join(f.messages, "\n\n"), nil
	}
	if f.amend {
		return "", nil
	}
	if f.ai || a.aiByDefault() {
		msg, err := a.aiMessage(ctx, f.ai)
		if err != nil || msg != "" {
			return msg, err
		}
	}
	if !a.interactive() {
		return "", fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgCommitNoMessage))
	}
	for {
		line, err := a.readLine(i18n.T(a.lang, i18n.MsgCommitPrompt))
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(line) != "" {
			return line, nil
		}
	}
}

func (a *app) commit(ctx context.Context, f commitFlags) (safety.Result, error) {
	msg, err := a.message(ctx, f)
	if err != nil {
		return safety.Result{}, err
	}
	if msg != "" {
		a.fixesHint(ctx, msg)
	}
	res, err := a.runOp(ctx, safety.Commit(git.CommitOptions{Message: msg, Amend: f.amend, AllowEmpty: f.allowEmpty}))
	if err != nil {
		return res, err
	}
	if a.flags.json {
		return res, a.writeJSON(map[string]string{"commit": res.Commit.Hash})
	}
	shown := firstLine(msg)
	if shown == "" {
		if m, err := a.session.runner.LastCommitMessage(ctx, a.session.repoRoot); err == nil {
			shown = firstLine(m)
		}
	}
	fmt.Fprintln(a.env.Stdout, a.styleFor(a.env.Stdout).green(i18n.T(a.lang, i18n.MsgCommitDone, short(res.Commit.Hash), shown)))
	return res, nil
}

func (a *app) newCommitCmd() *cobra.Command {
	var f commitFlags
	cmd := &cobra.Command{
		Use:   "commit",
		Short: i18n.T(a.lang, i18n.MsgCommitShort),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := a.commit(cmd.Context(), f)
			return err
		},
	}
	f.register(cmd, a, true)
	return cmd
}

func (a *app) newSaveCmd() *cobra.Command {
	var (
		f      commitFlags
		all    bool
		noPush bool
	)
	cmd := &cobra.Command{
		Use:   "save [paths...]",
		Short: i18n.T(a.lang, i18n.MsgSaveShort),
		RunE: func(cmd *cobra.Command, paths []string) error {
			return a.save(cmd.Context(), f, paths, all, noPush)
		},
	}
	f.register(cmd, a, false)
	cmd.Flags().BoolVarP(&all, "all", "A", false, i18n.T(a.lang, i18n.MsgAddAllFlag))
	cmd.Flags().BoolVar(&noPush, "no-push", false, i18n.T(a.lang, i18n.MsgSaveNoPush))
	return cmd
}

// save is the main flow (spec §8): summary, pick files, check, commit, push.
func (a *app) save(ctx context.Context, f commitFlags, paths []string, all, noPush bool) error {
	runner, root, err := a.git()
	if err != nil {
		return err
	}
	out := a.env.Stdout
	style := a.styleFor(out)
	loc := i18n.For(a.lang)

	st, err := runner.Status(ctx, root, git.StatusOptions{})
	if err != nil {
		return err
	}
	if !a.flags.json {
		a.printBranchLine(out, st)
		fmt.Fprintln(out)
	}
	if st.Clean() {
		fmt.Fprintln(out, loc.T(i18n.MsgSaveNothing))
		return nil
	}

	// 1. Which files.
	step := func(n int, key i18n.Key) {
		if !a.flags.json {
			fmt.Fprintln(out, style.bold(loc.T(i18n.MsgSaveStep, n, loc.T(key))))
		}
	}
	step(1, i18n.MsgSaveStepStage)
	if len(paths) == 0 {
		items := changeItems(st, true)
		if a.interactive() && !all {
			if paths, err = a.pick(ctx, loc.T(i18n.MsgAddPickTitle), items); err != nil {
				return err
			}
		} else {
			for _, item := range items {
				paths = append(paths, item.Value)
			}
		}
	}
	if len(paths) == 0 {
		fmt.Fprintln(a.env.Stderr, loc.T(i18n.MsgNothingSelected))
		return errCanceled
	}
	// What is staged but not chosen leaves the index; the choice is what
	// goes into the commit.
	var drop []string
	for _, e := range st.Staged() {
		if !contains(paths, e.Path) {
			drop = append(drop, e.Path)
			if e.OrigPath != "" {
				drop = append(drop, e.OrigPath)
			}
		}
	}
	if len(drop) > 0 && !a.flags.dryRun {
		if err := runner.Unstage(ctx, root, drop); err != nil {
			return err
		}
	}
	if !a.flags.dryRun {
		if err := runner.Stage(ctx, root, stagePaths(st, paths)); err != nil {
			return err
		}
	}
	for _, p := range paths {
		if !a.flags.json {
			fmt.Fprintf(out, "  %s\n", p)
		}
	}

	// 2. Commit (the safety layer checks for secrets and large files).
	step(2, i18n.MsgSaveStepCommit)
	res, err := a.commit(ctx, f)
	if err != nil {
		return err
	}
	if noPush {
		return nil
	}

	// 3. Push, setting the upstream when there is none.
	step(3, i18n.MsgSaveStepPush)
	pushed, err := a.push(ctx, git.PushOptions{SetUpstream: st.Branch.Upstream == "" || st.Branch.UpstreamGone})
	if err != nil {
		return err
	}
	if !a.flags.json {
		where := pushed.Remote + "/" + pushed.RemoteBranch
		fmt.Fprintln(out, style.green(loc.T(i18n.MsgSaveSummary, len(paths), short(res.Commit.Hash), where)))
	}
	return nil
}
