package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	lanes "github.com/muhammedeminalan/gitpal/internal/core/graph"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

func (a *app) newLogCmd() *cobra.Command {
	var (
		opts  git.LogOptions
		graph bool
	)
	cmd := &cobra.Command{
		Use:   "log [revision] [-- paths...]",
		Short: i18n.T(a.lang, i18n.MsgLogShort),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			if dash := cmd.ArgsLenAtDash(); dash >= 0 {
				opts.Revs, opts.Paths = args[:dash], args[dash:]
			} else {
				opts.Revs = args
			}
			opts.IgnoreCase = true
			commits, err := runner.Log(cmd.Context(), root, opts)
			if err != nil {
				return err
			}
			if a.flags.json {
				if commits == nil {
					commits = []git.Commit{}
				}
				return a.writeJSON(commits)
			}
			a.printLog(commits, graph)
			return nil
		},
	}
	cmd.Flags().IntVarP(&opts.Max, "max", "n", 30, i18n.T(a.lang, i18n.MsgLogMax))
	cmd.Flags().StringVar(&opts.Author, "author", "", i18n.T(a.lang, i18n.MsgLogAuthor))
	cmd.Flags().StringVar(&opts.Grep, "grep", "", i18n.T(a.lang, i18n.MsgLogGrep))
	cmd.Flags().BoolVar(&graph, "graph", false, i18n.T(a.lang, i18n.MsgLogGraph))
	cmd.Flags().BoolVar(&opts.All, "all", false, i18n.T(a.lang, i18n.MsgLogAll))
	return cmd
}

func (a *app) printLog(commits []git.Commit, graph bool) {
	w := a.env.Stdout
	st := a.styleFor(w)
	if len(commits) == 0 {
		fmt.Fprintln(w, i18n.T(a.lang, i18n.MsgLogEmpty))
		return
	}
	var rows []string
	if graph {
		rows = lanes.Rows(commits)
	}
	for i, c := range commits {
		prefix := ""
		if graph {
			prefix = st.yellow(rows[i]) + " "
		}
		refs := ""
		if len(c.Refs) > 0 {
			refs = " " + st.green("("+strings.Join(c.Refs, ", ")+")")
		}
		fmt.Fprintf(w, "%s%s%s %s %s\n", prefix, st.yellow(c.ShortHash), refs, c.Subject,
			st.dim("— "+c.AuthorName+", "+c.AuthorDate.Format("2006-01-02 15:04")))
	}
}
