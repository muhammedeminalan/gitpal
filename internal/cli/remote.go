package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
)

func (a *app) push(ctx context.Context, opts git.PushOptions) (git.PushResult, error) {
	if !opts.SetUpstream && opts.Branch == "" && opts.Remote == "" {
		// The first push of a branch sets its upstream, so the next one needs
		// no arguments.
		runner, root, err := a.git()
		if err != nil {
			return git.PushResult{}, err
		}
		if _, ok, err := runner.Upstream(ctx, root, ""); err == nil && !ok {
			opts.SetUpstream = true
		}
	}
	if opts.Branch == "" && !opts.ForceWithLease {
		if runner, root, err := a.git(); err == nil {
			if branch, err := runner.CurrentBranch(ctx, root); err == nil && branch != "" {
				a.warnIfProtected(ctx, branch)
			}
		}
	}
	res, err := a.runOp(ctx, safety.Push(opts))
	if err != nil {
		return res.Push, err
	}
	switch {
	case a.flags.json:
		return res.Push, a.writeJSON(res.Push)
	case res.Push.Status == git.PushUpToDate:
		fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgPushUpToDate))
	default:
		fmt.Fprintln(a.env.Stdout, a.styleFor(a.env.Stdout).green(
			i18n.T(a.lang, i18n.MsgPushDone, res.Push.Branch, res.Push.Remote, res.Push.RemoteBranch)))
	}
	return res.Push, nil
}

func (a *app) newPushCmd() *cobra.Command {
	var opts git.PushOptions
	cmd := &cobra.Command{
		Use:   "push [remote] [branch]",
		Short: i18n.T(a.lang, i18n.MsgPushShort),
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Remote = args[0]
			}
			if len(args) > 1 {
				opts.Branch = args[1]
			}
			_, err := a.push(cmd.Context(), opts)
			return err
		},
	}
	// There is no plain --force: asking for it gets --force-with-lease (spec 7.2).
	cmd.Flags().BoolVarP(&opts.ForceWithLease, "force", "f", false, i18n.T(a.lang, i18n.MsgPushForce))
	cmd.Flags().BoolVar(&opts.ForceWithLease, "force-with-lease", false, i18n.T(a.lang, i18n.MsgPushForce))
	cmd.Flags().BoolVarP(&opts.SetUpstream, "set-upstream", "u", false, i18n.T(a.lang, i18n.MsgPushSetUpstream))
	return cmd
}

func (a *app) newPullCmd() *cobra.Command {
	var rebase, merge, ffOnly bool
	cmd := &cobra.Command{
		Use:   "pull [remote branch]",
		Short: i18n.T(a.lang, i18n.MsgPullShort),
		Args:  cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := a.cfg()
			if err != nil {
				return err
			}
			// gitpal's pull.strategy values are git's own.
			strategy := git.PullStrategy(c.Pull.Strategy)
			switch {
			case rebase:
				strategy = git.PullRebase
			case merge:
				strategy = git.PullMerge
			case ffOnly:
				strategy = git.PullFFOnly
			}
			opts := git.PullOptions{Strategy: strategy}
			if len(args) == 2 {
				opts.Remote, opts.Branch = args[0], args[1]
			} else if len(args) == 1 {
				return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgGitArgInvalidBranch, ""))
			}
			res, err := a.runOp(cmd.Context(), safety.Pull(opts))
			if err != nil {
				return err
			}
			switch {
			case a.flags.json:
				return a.writeJSON(res.Pull)
			case !res.Pull.Updated:
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgPullUpToDate))
			default:
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgPullDone, short(res.Pull.Before), short(res.Pull.After)))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&rebase, "rebase", false, i18n.T(a.lang, i18n.MsgPullRebase))
	cmd.Flags().BoolVar(&merge, "merge", false, i18n.T(a.lang, i18n.MsgPullMerge))
	cmd.Flags().BoolVar(&ffOnly, "ff-only", false, i18n.T(a.lang, i18n.MsgPullFFOnly))
	cmd.MarkFlagsMutuallyExclusive("rebase", "merge", "ff-only")
	return cmd
}

func (a *app) newFetchCmd() *cobra.Command {
	var opts git.FetchOptions
	cmd := &cobra.Command{
		Use:   "fetch [remote]",
		Short: i18n.T(a.lang, i18n.MsgFetchShort),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				opts.Remote = args[0]
			}
			if err := runner.Fetch(cmd.Context(), root, opts); err != nil {
				return err
			}
			if !a.flags.json {
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgFetchDone))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.All, "all", false, i18n.T(a.lang, i18n.MsgFetchAll))
	cmd.Flags().BoolVarP(&opts.Prune, "prune", "p", false, i18n.T(a.lang, i18n.MsgFetchPrune))
	return cmd
}
