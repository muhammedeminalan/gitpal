package cli

import (
	"strings"

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
		a.newMergeCmd(), a.newRebaseCmd(), a.newCherryPickCmd(), a.newRevertCmd(),
		a.newContinueCmd(), a.newAbortCmd(), a.newSkipCmd(), a.newConflictsCmd(), a.newResolveCmd(),
		a.newStashCmd(), a.newTagCmd(), a.newReflogCmd(), a.newRecoverCmd(), a.newBlameCmd(),
		a.newRemoteCmd(), a.newSubmoduleCmd(), a.newIdentityCmd(), a.newWorkspaceCmd(),
		a.newRepoCmd(), a.newPRCmd(), a.newIssueCmd(), a.newReleaseCmd(), a.newActionsCmd(),
		a.newGUICmd(),
	)
	// After the commands: cobra adds help and completion only to a command
	// that has subcommands.
	a.localizeHelp(root, loc)
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

// localizeHelp replaces cobra's built-in English help texts. The template
// is cobra's default without command groups (gitpal has none), with the
// headings from the catalog.
func (a *app) localizeHelp(root *cobra.Command, loc i18n.Localizer) {
	more := `"{{.CommandPath}} [` + loc.T(i18n.MsgHelpCommandWord) + `] --help"`
	r := strings.NewReplacer(
		"@USAGE@", loc.T(i18n.MsgHelpUsage),
		"@ALIASES@", loc.T(i18n.MsgHelpAliases),
		"@EXAMPLES@", loc.T(i18n.MsgHelpExamples),
		"@COMMANDS@", loc.T(i18n.MsgHelpCommands),
		"@FLAGS@", loc.T(i18n.MsgHelpFlags),
		"@GLOBAL@", loc.T(i18n.MsgHelpGlobalFlags),
		"@FLAGSWORD@", loc.T(i18n.MsgHelpFlagsWord),
		"@COMMANDWORD@", loc.T(i18n.MsgHelpCommandWord),
		"@MORE@", loc.T(i18n.MsgHelpMore, more),
	)
	root.SetUsageTemplate(r.Replace(`@USAGE@{{if .Runnable}}
  {{.CommandPath}}{{if .HasAvailableFlags}} [@FLAGSWORD@]{{end}}{{if ne .Use .Name}} {{slice .Use (len .Name)}}{{end}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [@COMMANDWORD@]{{end}}{{if gt (len .Aliases) 0}}

@ALIASES@
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

@EXAMPLES@
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

@COMMANDS@{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

@FLAGS@
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

@GLOBAL@
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

@MORE@{{end}}
`))
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	for _, c := range root.Commands() {
		switch c.Name() {
		case "help":
			c.Short = loc.T(i18n.MsgHelpShort)
		case "completion":
			c.Short = loc.T(i18n.MsgCompletionShort)
		}
	}
}
