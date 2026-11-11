package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/history"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
)

func (a *app) newUndoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo",
		Short: i18n.T(a.lang, i18n.MsgUndoShort),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			g, err := a.guard(ctx)
			if err != nil {
				return err
			}
			plan, err := g.PlanUndo(ctx)
			if err != nil {
				return err
			}
			if plan.Err != nil {
				return plan.Err
			}
			fmt.Fprintln(a.env.Stderr, i18n.T(a.lang, i18n.MsgUndoWouldUndo, plan.Entry.Op, a.when(plan.Entry.Time)))
			if a.flags.dryRun {
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgDryRunNothingDone))
				return nil
			}
			var approve safety.Approver
			if a.interactive() {
				approve = func(safety.Assessment) (safety.Answer, error) {
					line, err := a.readLine(i18n.T(a.lang, i18n.MsgConfirmQuestion))
					return safety.Answer{Approved: a.yes(line, false)}, err
				}
			}
			if _, err := g.Undo(ctx, approve, safety.DoOptions{AssumeYes: a.flags.yes}); err != nil {
				return err
			}
			fmt.Fprintln(a.env.Stdout, a.styleFor(a.env.Stdout).green(i18n.T(a.lang, i18n.MsgUndoDone, plan.Entry.Op)))
			return nil
		},
	}
}

func (a *app) when(t time.Time) string { return t.Local().Format("2006-01-02 15:04:05") }

func (a *app) newBackupsCmd() *cobra.Command {
	list := func(cmd *cobra.Command, _ []string) error {
		g, err := a.guard(cmd.Context())
		if err != nil {
			return err
		}
		backups, err := g.Backups(cmd.Context())
		if err != nil {
			return err
		}
		if a.flags.json {
			if backups == nil {
				backups = []safety.Backup{}
			}
			return a.writeJSON(backups)
		}
		if len(backups) == 0 {
			fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgBackupsNone))
			return nil
		}
		st := a.styleFor(a.env.Stdout)
		for _, b := range backups {
			kind := ""
			if b.Worktree {
				kind = " (" + i18n.T(a.lang, i18n.MsgBackupsWorktreeTag) + ")"
			}
			fmt.Fprintf(a.env.Stdout, "%s  %s  %s%s\n    %s\n", a.when(b.Time), st.yellow(short(b.Hash)), b.Op, kind, st.dim(b.Ref))
		}
		return nil
	}
	cmd := &cobra.Command{Use: "backups", Short: i18n.T(a.lang, i18n.MsgBackupsShort), Args: cobra.NoArgs, RunE: list}
	cmd.AddCommand(
		&cobra.Command{Use: "list", Short: i18n.T(a.lang, i18n.MsgBackupsListShort), Args: cobra.NoArgs, RunE: list},
		&cobra.Command{
			Use: "restore <ref>", Short: i18n.T(a.lang, i18n.MsgBackupsRestore), Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				g, err := a.guard(cmd.Context())
				if err != nil {
					return err
				}
				shown := false
				if _, err := g.RestoreBackup(cmd.Context(), args[0], a.approver(&shown), safety.DoOptions{AssumeYes: a.flags.yes}); err != nil {
					return err
				}
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgBackupsRestored, args[0]))
				return nil
			},
		},
		&cobra.Command{
			Use: "prune", Short: i18n.T(a.lang, i18n.MsgBackupsPruneShort), Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				g, err := a.guard(cmd.Context())
				if err != nil {
					return err
				}
				removed, err := g.PruneBackups(cmd.Context())
				if err != nil {
					return err
				}
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgBackupsPruned, len(removed)))
				return nil
			},
		},
	)
	return cmd
}

func (a *app) newHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: i18n.T(a.lang, i18n.MsgHistoryShort),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			g, err := a.guard(cmd.Context())
			if err != nil {
				return err
			}
			entries, err := g.History().Entries()
			if err != nil {
				return err
			}
			if a.flags.json {
				if entries == nil {
					entries = []history.Entry{}
				}
				return a.writeJSON(entries)
			}
			if len(entries) == 0 {
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgHistoryEmpty))
				return nil
			}
			undone := map[string]bool{}
			for _, e := range entries {
				if e.Op == history.OpUndo && e.Failed == "" {
					undone[e.Undoes] = true
				}
			}
			st := a.styleFor(a.env.Stdout)
			for _, e := range backward(entries) {
				line := fmt.Sprintf("%s  %-14s %s → %s", a.when(e.Time), e.Op, short(e.Before), short(e.After))
				switch {
				case e.Failed != "":
					line += " " + st.red(i18n.T(a.lang, i18n.MsgHistoryFailed, e.Failed))
				case undone[e.ID]:
					line += " " + st.dim(i18n.T(a.lang, i18n.MsgHistoryUndone))
				}
				fmt.Fprintln(a.env.Stdout, line)
			}
			return nil
		},
	}
}

// backward returns the entries newest first.
func backward(entries []history.Entry) []history.Entry {
	out := make([]history.Entry, len(entries))
	for i, e := range entries {
		out[len(entries)-1-i] = e
	}
	return out
}
