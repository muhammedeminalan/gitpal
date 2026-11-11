package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

func (a *app) newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: i18n.T(a.lang, i18n.MsgConfigShort)}
	list := func(*cobra.Command, []string) error {
		c, err := a.cfg()
		if err != nil {
			return err
		}
		if a.flags.json {
			return a.writeJSON(c)
		}
		for _, key := range config.Keys() {
			value, _ := config.Get(c, key)
			fmt.Fprintf(a.env.Stdout, "%s = %s\n", key, value)
		}
		return nil
	}
	cmd.RunE = list
	unknown := func(key string, err error) error {
		if errors.Is(err, config.ErrUnknownKey) {
			return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgConfigUnknown, key))
		}
		return err
	}
	cmd.AddCommand(
		&cobra.Command{Use: "list", Short: i18n.T(a.lang, i18n.MsgConfigListShort), Args: cobra.NoArgs, RunE: list},
		&cobra.Command{
			Use: "get <key>", Short: i18n.T(a.lang, i18n.MsgConfigGetShort), Args: cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				c, err := a.cfg()
				if err != nil {
					return err
				}
				value, err := config.Get(c, args[0])
				if err != nil {
					return unknown(args[0], err)
				}
				fmt.Fprintln(a.env.Stdout, value)
				return nil
			},
		},
		&cobra.Command{
			Use: "set <key> <value>", Short: i18n.T(a.lang, i18n.MsgConfigSetShort), Args: cobra.ExactArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				path, err := config.GlobalPath(a.cfgEnv)
				if err != nil {
					return err
				}
				if err := config.SetGlobal(path, args[0], args[1]); err != nil {
					return unknown(args[0], err)
				}
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgConfigSaved, args[0], args[1]))
				return nil
			},
		},
		&cobra.Command{
			Use: "path", Short: i18n.T(a.lang, i18n.MsgConfigPathShort), Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				path, err := config.GlobalPath(a.cfgEnv)
				if err != nil {
					return err
				}
				fmt.Fprintln(a.env.Stdout, path)
				if a.session.loaded.RepoPath != "" {
					fmt.Fprintln(a.env.Stdout, a.session.loaded.RepoPath)
				}
				return nil
			},
		},
		&cobra.Command{
			Use: "edit", Short: i18n.T(a.lang, i18n.MsgConfigEditShort), Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				path, err := config.GlobalPath(a.cfgEnv)
				if err != nil {
					return err
				}
				if err := a.env.Editor(cmd.Context(), path); err != nil {
					if errors.Is(err, errNoEditor) {
						return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgConfigNoEditor, path))
					}
					return err
				}
				return nil
			},
		},
	)
	return cmd
}
