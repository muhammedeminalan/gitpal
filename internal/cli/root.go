package cli

import (
	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/version"
)

func (a *app) newRootCmd() *cobra.Command {
	loc := i18n.For(a.lang)

	root := &cobra.Command{
		Use:     "gitpal",
		Short:   loc.T(i18n.MsgRootShort),
		Long:    loc.T(i18n.MsgRootLong),
		Version: version.String(),
		// Errors and usage are printed by Run, in the user's language.
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return a.startSession(cmd.Context())
		},
	}
	root.SetVersionTemplate("gitpal {{.Version}}\n")

	flags := root.PersistentFlags()
	flags.BoolVar(&a.flags.json, "json", false, loc.T(i18n.MsgFlagJSON))
	flags.BoolVar(&a.flags.noColor, "no-color", false, loc.T(i18n.MsgFlagNoColor))
	flags.BoolVar(&a.flags.explain, "explain", false, loc.T(i18n.MsgFlagExplain))
	flags.StringVar(&a.flags.repo, "repo", "", loc.T(i18n.MsgFlagRepo))
	flags.BoolVar(&a.flags.dryRun, "dry-run", false, loc.T(i18n.MsgFlagDryRun))
	flags.BoolVarP(&a.flags.yes, "yes", "y", false, loc.T(i18n.MsgFlagYes))
	flags.BoolP("help", "h", false, loc.T(i18n.MsgFlagHelp))
	root.Flags().BoolP("version", "v", false, loc.T(i18n.MsgFlagVersion))

	root.AddCommand(
		a.newDoctorCmd(),
		a.newStatusCmd(), a.newDiffCmd(),
		a.newAddCmd(), a.newUnstageCmd(), a.newDiscardCmd(), a.newIgnoreCmd(),
		a.newCommitCmd(), a.newSaveCmd(),
		a.newPushCmd(), a.newPullCmd(), a.newFetchCmd(),
		a.newBranchCmd(), a.newLogCmd(),
		a.newUndoCmd(), a.newBackupsCmd(), a.newHistoryCmd(),
		a.newConfigCmd(), a.newInitCmd(), a.newCloneCmd(), a.newAuthCmd(),
	)
	// Without a subcommand: the interactive screen for a person, help for a
	// script.
	// cobra's default argument check stays: an unknown word is still an
	// unknown command.
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		if !a.env.Interactive() {
			return cmd.Help()
		}
		return a.runDashboard(cmd.Context())
	}
	return root
}
