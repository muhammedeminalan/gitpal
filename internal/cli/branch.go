package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
	"github.com/muhammedeminalan/gitpal/internal/tui"
)

func (a *app) newBranchCmd() *cobra.Command {
	var remotes bool
	list := func(cmd *cobra.Command, _ []string) error {
		runner, root, err := a.git()
		if err != nil {
			return err
		}
		branches, err := runner.Branches(cmd.Context(), root, git.BranchListOptions{Remotes: remotes})
		if err != nil {
			return err
		}
		if a.flags.json {
			if branches == nil {
				branches = []git.Branch{}
			}
			return a.writeJSON(branches)
		}
		a.printBranches(branches)
		return nil
	}
	cmd := &cobra.Command{
		Use:   "branch",
		Short: i18n.T(a.lang, i18n.MsgBranchShort),
		Args:  cobra.NoArgs,
		RunE:  list,
	}
	cmd.Flags().BoolVarP(&remotes, "remotes", "r", false, i18n.T(a.lang, i18n.MsgBranchRemotes))

	listCmd := &cobra.Command{Use: "list", Short: i18n.T(a.lang, i18n.MsgBranchListShort), Args: cobra.NoArgs, RunE: list}
	listCmd.Flags().BoolVarP(&remotes, "remotes", "r", false, i18n.T(a.lang, i18n.MsgBranchRemotes))

	cmd.AddCommand(listCmd, a.newBranchNewCmd(), a.newBranchSwitchCmd(), a.newBranchDeleteCmd(),
		a.newBranchRenameCmd(), a.newBranchCleanCmd())
	return cmd
}

func (a *app) printBranches(branches []git.Branch) {
	w := a.env.Stdout
	st := a.styleFor(w)
	loc := i18n.For(a.lang)
	for _, b := range branches {
		mark := "  "
		name := b.Name
		switch {
		case b.Current:
			mark, name = st.green("* "), st.green(st.bold(name))
		case b.Remote:
			name = st.red(name)
		}
		var extra []string
		if b.Upstream != "" {
			track := b.Upstream
			switch {
			case b.UpstreamGone:
				track += ": " + loc.T(i18n.MsgBranchUpstreamGone)
			case b.Ahead > 0 || b.Behind > 0:
				track += ": " + loc.T(i18n.MsgBranchAheadBehind, b.Ahead, b.Behind)
			}
			extra = append(extra, "["+track+"]")
		}
		fmt.Fprintf(w, "%s%s %s %s %s\n", mark, name, st.dim(short(b.Hash)), strings.Join(extra, " "), b.Subject)
	}
}

// branchName checks a name for a new branch and, when it is not valid,
// offers a usable one.
func (a *app) branchName(ctx context.Context, name string) error {
	runner, root, err := a.git()
	if err != nil {
		return err
	}
	if err := runner.ValidateBranchName(ctx, root, name); err != nil {
		if suggestion := git.SuggestBranchName(name); suggestion != "" && suggestion != name {
			return fmt.Errorf("%s\n%s", i18n.Describe(a.lang, err), i18n.T(a.lang, i18n.MsgBranchSuggest, suggestion))
		}
		return err
	}
	return nil
}

func (a *app) newBranchNewCmd() *cobra.Command {
	var noSwitch bool
	cmd := &cobra.Command{
		Use:   "new <name> [start]",
		Short: i18n.T(a.lang, i18n.MsgBranchNewShort),
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, start := args[0], ""
			if len(args) == 2 {
				start = args[1]
			}
			if err := a.branchName(ctx, name); err != nil {
				return err
			}
			runner, root, _ := a.git()
			if noSwitch {
				if err := runner.CreateBranch(ctx, root, name, start); err != nil {
					return err
				}
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgBranchCreated, name))
				return nil
			}
			if err := runner.CreateAndSwitch(ctx, root, name, start); err != nil {
				return err
			}
			fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgBranchSwitched, name))
			return nil
		},
	}
	cmd.Flags().BoolVar(&noSwitch, "no-switch", false, i18n.T(a.lang, i18n.MsgBranchNoSwitch))
	return cmd
}

func (a *app) newBranchSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "switch <name>",
		Aliases: []string{"checkout"},
		Short:   i18n.T(a.lang, i18n.MsgBranchSwitchShort),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := a.runOp(cmd.Context(), safety.Switch(args[0])); err != nil {
				return err
			}
			fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgBranchSwitched, args[0]))
			return nil
		},
	}
}

func (a *app) newBranchDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: i18n.T(a.lang, i18n.MsgBranchDeleteShort),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := a.runOp(cmd.Context(), safety.DeleteBranch(args[0], force)); err != nil {
				return err
			}
			fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgBranchDeleted, args[0]))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, i18n.T(a.lang, i18n.MsgBranchForce))
	return cmd
}

func (a *app) newBranchRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: i18n.T(a.lang, i18n.MsgBranchRenameShort),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.branchName(cmd.Context(), args[1]); err != nil {
				return err
			}
			runner, root, _ := a.git()
			if err := runner.RenameBranch(cmd.Context(), root, args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgBranchRenamed, args[0], args[1]))
			return nil
		},
	}
}

func (a *app) newBranchCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean [into]",
		Short: i18n.T(a.lang, i18n.MsgBranchCleanShort),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			into := ""
			if len(args) == 1 {
				into = args[0]
			}
			merged, err := runner.MergedBranches(ctx, root, into)
			if err != nil {
				return err
			}
			c, _ := a.cfg()
			// Protected branches are never cleaned up in bulk.
			p := policy(c.Safety)
			var candidates []string
			for _, name := range merged {
				if !p.Protected(name) {
					candidates = append(candidates, name)
				}
			}
			if len(candidates) == 0 {
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgBranchCleanNone))
				return nil
			}
			if a.interactive() && !a.flags.dryRun {
				items := make([]tui.Item, 0, len(candidates))
				for _, name := range candidates {
					items = append(items, tui.Item{Label: name, Value: name, Selected: true})
				}
				if candidates, err = a.pick(ctx, i18n.T(a.lang, i18n.MsgBranchCleanShort), items); err != nil {
					return err
				}
			}
			for _, name := range candidates {
				_, err := a.runOp(ctx, safety.DeleteBranch(name, false))
				if errors.Is(err, errDryRun) {
					continue
				}
				if err != nil {
					return err
				}
				if !a.flags.dryRun {
					fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgBranchDeleted, name))
				}
			}
			return nil
		},
	}
}
