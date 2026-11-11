package cli

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// target resolves a directory given on the command line against the working
// directory.
func (a *app) target(dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(a.session.dir, dir)
}

func (a *app) newInitCmd() *cobra.Command {
	var branch string
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: i18n.T(a.lang, i18n.MsgInitShort),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := a.cfg(); err != nil {
				return err
			}
			dir := a.session.dir
			if len(args) == 1 {
				dir = a.target(args[0])
			}
			if err := a.session.runner.Init(cmd.Context(), dir, branch); err != nil {
				return err
			}
			fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgInitDone, dir))
			return nil
		},
	}
	cmd.Flags().StringVarP(&branch, "branch", "b", "main", i18n.T(a.lang, i18n.MsgInitBranch))
	return cmd
}

// cloneDir is the directory git would pick for a URL: its last path part,
// without ".git".
func cloneDir(url string) string {
	url = strings.TrimRight(strings.ReplaceAll(url, `\`, "/"), "/")
	if i := strings.LastIndexAny(url, ":/"); i >= 0 {
		url = url[i+1:]
	}
	return strings.TrimSuffix(path.Base(url), ".git")
}

func (a *app) newCloneCmd() *cobra.Command {
	var opts git.CloneOptions
	cmd := &cobra.Command{
		Use:   "clone <url> [dir]",
		Short: i18n.T(a.lang, i18n.MsgCloneShort),
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := a.cfg(); err != nil {
				return err
			}
			dir := cloneDir(args[0])
			if len(args) == 2 {
				dir = args[1]
			}
			dir = a.target(dir)
			if !a.flags.json {
				opts.Progress = a.env.Stderr
			}
			if err := a.session.runner.Clone(cmd.Context(), args[0], dir, opts); err != nil {
				return err
			}
			fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgCloneDone, dir))
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.Branch, "branch", "b", "", i18n.T(a.lang, i18n.MsgCloneBranch))
	cmd.Flags().IntVar(&opts.Depth, "depth", 0, i18n.T(a.lang, i18n.MsgCloneDepth))
	return cmd
}
