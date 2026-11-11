package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
	"github.com/muhammedeminalan/gitpal/internal/tui"
)

// say prints a result line unless the output is JSON.
func (a *app) say(key i18n.Key, args ...any) {
	if !a.flags.json {
		fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, key, args...))
	}
}

// stopped explains what to do when an operation stopped on conflicts.
func (a *app) stopped(err error) error {
	if git.KindOf(err) == git.KindConflict {
		fmt.Fprintln(a.env.Stderr, a.styleFor(a.env.Stderr).yellow(i18n.T(a.lang, i18n.MsgConflictsNext)))
	}
	return err
}

func (a *app) newMergeCmd() *cobra.Command {
	var noFF, ffOnly, squash bool
	var message string
	cmd := &cobra.Command{
		Use:   "merge <branch>",
		Short: i18n.T(a.lang, i18n.MsgMergeShort),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := git.MergeOptions{Rev: args[0], Message: message}
			switch {
			case noFF:
				opts.Mode = git.MergeNoFF
			case ffOnly:
				opts.Mode = git.MergeFFOnly
			case squash:
				opts.Mode = git.MergeSquash
			}
			res, err := a.runOp(cmd.Context(), safety.Merge(opts))
			if err != nil {
				return a.stopped(err)
			}
			switch m := res.Merge; {
			case m.Squashed:
				a.say(i18n.MsgMergeSquashed)
			case m.UpToDate:
				a.say(i18n.MsgMergeUpToDate)
			case m.FastForward:
				a.say(i18n.MsgMergeFF, short(m.After))
			default:
				a.say(i18n.MsgMergeDone, short(m.After))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noFF, "no-ff", false, i18n.T(a.lang, i18n.MsgMergeNoFF))
	cmd.Flags().BoolVar(&ffOnly, "ff-only", false, i18n.T(a.lang, i18n.MsgMergeFFOnly))
	cmd.Flags().BoolVar(&squash, "squash", false, i18n.T(a.lang, i18n.MsgMergeSquash))
	cmd.Flags().StringVarP(&message, "message", "m", "", i18n.T(a.lang, i18n.MsgMergeMessage))
	cmd.MarkFlagsMutuallyExclusive("no-ff", "ff-only", "squash")
	return cmd
}

func (a *app) newRebaseCmd() *cobra.Command {
	var interactive bool
	cmd := &cobra.Command{
		Use:   "rebase <upstream>",
		Short: i18n.T(a.lang, i18n.MsgRebaseShort),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			opts := git.RebaseOptions{Upstream: args[0]}
			if interactive {
				steps, err := a.editRebase(ctx, args[0])
				if err != nil || steps == nil {
					return err
				}
				opts.Steps = steps
			}
			res, err := a.runOp(ctx, safety.Rebase(opts))
			if err != nil {
				return a.stopped(err)
			}
			a.say(i18n.MsgRebaseDone, short(res.Rebase.Before), short(res.Rebase.After))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, i18n.T(a.lang, i18n.MsgRebaseInteract))
	return cmd
}

// editRebase shows the plan of an interactive rebase for editing. nil steps
// with no error means there is nothing to do.
func (a *app) editRebase(ctx context.Context, upstream string) ([]git.RebaseStep, error) {
	runner, root, err := a.git()
	if err != nil {
		return nil, err
	}
	plan, err := runner.RebasePlan(ctx, root, upstream)
	if err != nil {
		return nil, err
	}
	if len(plan) == 0 {
		a.say(i18n.MsgRebaseNothing)
		return nil, nil
	}
	if !a.interactive() {
		return nil, fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgNeedsTerminal, "gitpal rebase "+upstream))
	}
	steps, err := a.env.RebaseEditor(ctx, tui.RebaseOptions{
		Lang: a.lang, Title: i18n.T(a.lang, i18n.MsgRebaseInteract), Steps: plan, In: a.env.Stdin, Out: a.env.Stderr,
	})
	if err == tui.ErrCanceled { //nolint:errorlint // a sentinel returned as is
		return nil, errCanceled
	}
	return steps, err
}

func (a *app) newCherryPickCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cherry-pick <commits...>",
		Short: i18n.T(a.lang, i18n.MsgCherryPickShort),
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := a.runOp(cmd.Context(), safety.CherryPick(args...)); err != nil {
				return a.stopped(err)
			}
			a.say(i18n.MsgOperationDone, "cherry-pick")
			return nil
		},
	}
}

func (a *app) newRevertCmd() *cobra.Command {
	var mainline int
	cmd := &cobra.Command{
		Use:   "revert <commits...>",
		Short: i18n.T(a.lang, i18n.MsgRevertShort),
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := a.runOp(cmd.Context(), safety.Revert(mainline, args...)); err != nil {
				return a.stopped(err)
			}
			a.say(i18n.MsgOperationDone, "revert")
			return nil
		},
	}
	cmd.Flags().IntVar(&mainline, "mainline", 0, i18n.T(a.lang, i18n.MsgRevertMainline))
	return cmd
}

func (a *app) operationStepCmd(use string, short i18n.Key, skip bool) *cobra.Command {
	return &cobra.Command{
		Use: use, Short: i18n.T(a.lang, short), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			g, err := a.guard(cmd.Context())
			if err != nil {
				return err
			}
			step := g.Continue
			if skip {
				step = g.Skip
			}
			op, _, err := step(cmd.Context())
			if git.ErrNoOperation(err) {
				return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgSafetyNothingToAbort))
			}
			if err != nil {
				return a.stopped(err)
			}
			a.say(i18n.MsgOperationDone, op)
			return nil
		},
	}
}

func (a *app) newContinueCmd() *cobra.Command {
	return a.operationStepCmd("continue", i18n.MsgContinueShort, false)
}

func (a *app) newSkipCmd() *cobra.Command {
	return a.operationStepCmd("skip", i18n.MsgSkipShort, true)
}

func (a *app) newAbortCmd() *cobra.Command {
	return &cobra.Command{
		Use: "abort", Short: i18n.T(a.lang, i18n.MsgAbortShort), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := a.runOp(cmd.Context(), safety.Abort()); err != nil {
				return err
			}
			a.say(i18n.MsgOperationDone, "abort")
			return nil
		},
	}
}

func (a *app) newConflictsCmd() *cobra.Command {
	return &cobra.Command{
		Use: "conflicts", Short: i18n.T(a.lang, i18n.MsgConflictsShort), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			st, err := runner.Status(cmd.Context(), root, git.StatusOptions{Untracked: git.UntrackedNone})
			if err != nil {
				return err
			}
			files := st.Conflicted()
			if a.flags.json {
				paths := []string{}
				for _, e := range files {
					paths = append(paths, e.Path)
				}
				return a.writeJSON(map[string]any{"operation": st.Operation, "files": paths})
			}
			if len(files) == 0 {
				a.say(i18n.MsgConflictsNone)
				return nil
			}
			if st.Operation != git.OpNone {
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgStatusOperation, st.Operation))
			}
			for _, e := range files {
				fmt.Fprintf(a.env.Stdout, "  %s%s  %s\n", string(e.Index), string(e.Worktree), e.Path)
			}
			fmt.Fprintln(a.env.Stderr, a.styleFor(a.env.Stderr).dim(i18n.T(a.lang, i18n.MsgConflictsNext)))
			return nil
		},
	}
}

func (a *app) newResolveCmd() *cobra.Command {
	var ours, theirs bool
	cmd := &cobra.Command{
		Use: "resolve <paths...>", Short: i18n.T(a.lang, i18n.MsgResolveShort), Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, paths []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			switch {
			case ours:
				err = runner.ResolveConflict(cmd.Context(), root, paths, git.SideOurs)
			case theirs:
				err = runner.ResolveConflict(cmd.Context(), root, paths, git.SideTheirs)
			default:
				err = runner.Stage(cmd.Context(), root, paths)
			}
			if err != nil {
				return err
			}
			for _, p := range paths {
				a.say(i18n.MsgResolveDone, p)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&ours, "ours", false, i18n.T(a.lang, i18n.MsgResolveOurs))
	cmd.Flags().BoolVar(&theirs, "theirs", false, i18n.T(a.lang, i18n.MsgResolveTheirs))
	cmd.MarkFlagsMutuallyExclusive("ours", "theirs")
	return cmd
}

// stashIndex reads the optional stash number argument.
func stashIndex(args []string) (int, error) {
	if len(args) == 0 {
		return 0, nil
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 0 {
		return 0, &git.ArgError{Code: git.ArgInvalidRev, Value: args[0]}
	}
	return n, nil
}

func (a *app) newStashCmd() *cobra.Command {
	list := func(cmd *cobra.Command, _ []string) error {
		runner, root, err := a.git()
		if err != nil {
			return err
		}
		stashes, err := runner.Stashes(cmd.Context(), root)
		if err != nil {
			return err
		}
		if a.flags.json {
			if stashes == nil {
				stashes = []git.Stash{}
			}
			return a.writeJSON(stashes)
		}
		if len(stashes) == 0 {
			a.say(i18n.MsgStashNone)
		}
		for _, s := range stashes {
			fmt.Fprintf(a.env.Stdout, "%s  %s  %s\n", s.Ref, a.when(s.Time), s.Message)
		}
		return nil
	}
	cmd := &cobra.Command{Use: "stash", Short: i18n.T(a.lang, i18n.MsgStashShort), Args: cobra.NoArgs, RunE: list}

	var opts git.StashOptions
	save := &cobra.Command{
		Use: "save [paths...]", Aliases: []string{"push"}, Short: i18n.T(a.lang, i18n.MsgStashSaveShort),
		RunE: func(cmd *cobra.Command, paths []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			opts.Paths = paths
			created, err := runner.StashPush(cmd.Context(), root, opts)
			if err != nil {
				return err
			}
			if created {
				a.say(i18n.MsgStashSaved)
			} else {
				a.say(i18n.MsgStashNothing)
			}
			return nil
		},
	}
	save.Flags().StringVarP(&opts.Message, "message", "m", "", i18n.T(a.lang, i18n.MsgCommitMessage))
	save.Flags().BoolVarP(&opts.IncludeUntracked, "include-untracked", "u", false, i18n.T(a.lang, i18n.MsgStashUntracked))

	apply := func(pop bool) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			n, err := stashIndex(args)
			if err != nil {
				return err
			}
			if err := runner.StashApply(cmd.Context(), root, n, pop); err != nil {
				return a.stopped(err)
			}
			a.say(i18n.MsgStashApplied, git.StashRef(n))
			return nil
		}
	}
	cmd.AddCommand(
		save,
		&cobra.Command{Use: "list", Short: i18n.T(a.lang, i18n.MsgStashListShort), Args: cobra.NoArgs, RunE: list},
		&cobra.Command{Use: "apply [n]", Short: i18n.T(a.lang, i18n.MsgStashApplyShort), Args: cobra.MaximumNArgs(1), RunE: apply(false)},
		&cobra.Command{Use: "pop [n]", Short: i18n.T(a.lang, i18n.MsgStashPopShort), Args: cobra.MaximumNArgs(1), RunE: apply(true)},
		&cobra.Command{
			Use: "drop [n]", Short: i18n.T(a.lang, i18n.MsgStashDropShort), Args: cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				n, err := stashIndex(args)
				if err != nil {
					return err
				}
				if _, err := a.runOp(cmd.Context(), safety.StashDrop(n)); err != nil {
					return err
				}
				a.say(i18n.MsgStashDropped, git.StashRef(n))
				return nil
			},
		},
		&cobra.Command{
			Use: "show [n]", Short: i18n.T(a.lang, i18n.MsgStashShowShort), Args: cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				runner, root, err := a.git()
				if err != nil {
					return err
				}
				n, err := stashIndex(args)
				if err != nil {
					return err
				}
				files, err := runner.Diff(cmd.Context(), root, git.DiffOptions{Commit: git.StashRef(n)})
				if err != nil {
					return err
				}
				if a.flags.json {
					return a.writeJSON(files)
				}
				a.printDiff(files)
				return nil
			},
		},
	)
	return cmd
}

func (a *app) newTagCmd() *cobra.Command {
	list := func(cmd *cobra.Command, _ []string) error {
		runner, root, err := a.git()
		if err != nil {
			return err
		}
		tags, err := runner.Tags(cmd.Context(), root)
		if err != nil {
			return err
		}
		if a.flags.json {
			if tags == nil {
				tags = []git.Tag{}
			}
			return a.writeJSON(tags)
		}
		if len(tags) == 0 {
			a.say(i18n.MsgTagNone)
		}
		for _, t := range tags {
			fmt.Fprintf(a.env.Stdout, "%s  %s  %s\n", t.Name, short(t.Commit), t.Subject)
		}
		return nil
	}
	cmd := &cobra.Command{Use: "tag", Short: i18n.T(a.lang, i18n.MsgTagShort), Args: cobra.NoArgs, RunE: list}
	var message string
	create := &cobra.Command{
		Use: "new <name> [commit]", Short: i18n.T(a.lang, i18n.MsgTagNewShort), Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			rev := ""
			if len(args) == 2 {
				rev = args[1]
			}
			if err := runner.CreateTag(cmd.Context(), root, args[0], rev, message); err != nil {
				return err
			}
			a.say(i18n.MsgTagCreated, args[0])
			return nil
		},
	}
	create.Flags().StringVarP(&message, "message", "m", "", i18n.T(a.lang, i18n.MsgTagMessage))
	cmd.AddCommand(
		&cobra.Command{Use: "list", Short: i18n.T(a.lang, i18n.MsgTagListShort), Args: cobra.NoArgs, RunE: list},
		create,
		&cobra.Command{
			Use: "delete <name>", Short: i18n.T(a.lang, i18n.MsgTagDeleteShort), Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				runner, root, err := a.git()
				if err != nil {
					return err
				}
				if err := runner.DeleteTag(cmd.Context(), root, args[0]); err != nil {
					return err
				}
				a.say(i18n.MsgTagDeleted, args[0])
				return nil
			},
		},
		&cobra.Command{
			Use: "push <name> [remote]", Short: i18n.T(a.lang, i18n.MsgTagPushShort), Args: cobra.RangeArgs(1, 2),
			RunE: func(cmd *cobra.Command, args []string) error {
				runner, root, err := a.git()
				if err != nil {
					return err
				}
				remote := "origin"
				if len(args) == 2 {
					remote = args[1]
				}
				if err := runner.PushTag(cmd.Context(), root, remote, args[0]); err != nil {
					return err
				}
				a.say(i18n.MsgTagPushed, args[0], remote)
				return nil
			},
		},
	)
	return cmd
}

func (a *app) newReflogCmd() *cobra.Command {
	var maxCount int
	cmd := &cobra.Command{
		Use: "reflog [ref]", Short: i18n.T(a.lang, i18n.MsgReflogShort), Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			entries, err := runner.Reflog(cmd.Context(), root, ref, maxCount)
			if err != nil {
				return err
			}
			if a.flags.json {
				if entries == nil {
					entries = []git.ReflogEntry{}
				}
				return a.writeJSON(entries)
			}
			st := a.styleFor(a.env.Stdout)
			for _, e := range entries {
				fmt.Fprintf(a.env.Stdout, "%s %s  %s  %s\n", st.yellow(short(e.Hash)), e.Selector, a.when(e.Time), e.Action)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&maxCount, "max", "n", 30, i18n.T(a.lang, i18n.MsgLogMax))
	return cmd
}

func (a *app) newRecoverCmd() *cobra.Command {
	return &cobra.Command{
		Use: "recover <commit> <new-branch>", Short: i18n.T(a.lang, i18n.MsgRecoverShort), Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.branchName(cmd.Context(), args[1]); err != nil {
				return err
			}
			runner, root, _ := a.git()
			if err := runner.CreateBranch(cmd.Context(), root, args[1], args[0]); err != nil {
				return err
			}
			a.say(i18n.MsgRecoverDone, args[0], args[1])
			return nil
		},
	}
}

func (a *app) newBlameCmd() *cobra.Command {
	return &cobra.Command{
		Use: "blame <path> [revision]", Short: i18n.T(a.lang, i18n.MsgBlameShort), Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			rev := ""
			if len(args) == 2 {
				rev = args[1]
			}
			lines, err := runner.Blame(cmd.Context(), root, args[0], rev)
			if err != nil {
				return err
			}
			if a.flags.json {
				return a.writeJSON(lines)
			}
			st := a.styleFor(a.env.Stdout)
			for _, l := range lines {
				author := []rune(l.Author)
				if len(author) > 16 {
					author = author[:16]
				}
				fmt.Fprintf(a.env.Stdout, "%s %-16s %s %4d  %s\n", st.yellow(short(l.Hash)), string(author),
					st.dim(l.Time.Format("2006-01-02")), l.Line, l.Text)
			}
			return nil
		},
	}
}
