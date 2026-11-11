package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

func (a *app) newRemoteCmd() *cobra.Command {
	list := func(cmd *cobra.Command, _ []string) error {
		runner, root, err := a.git()
		if err != nil {
			return err
		}
		remotes, err := runner.Remotes(cmd.Context(), root)
		if err != nil {
			return err
		}
		if a.flags.json {
			return a.writeJSON(remotes)
		}
		if len(remotes) == 0 {
			a.say(i18n.MsgRemoteNone)
		}
		for _, r := range remotes {
			fmt.Fprintf(a.env.Stdout, "%s\t%s (fetch)\n%s\t%s (push)\n", r.Name, r.FetchURL, r.Name, r.PushURL)
		}
		return nil
	}
	run := func(fn func(ctx context.Context, runner *git.Runner, root string, args []string) error) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			if err := fn(cmd.Context(), runner, root, args); err != nil {
				return err
			}
			a.say(i18n.MsgDone)
			return nil
		}
	}
	var push bool
	setURL := &cobra.Command{
		Use: "set-url <name> <url>", Short: i18n.T(a.lang, i18n.MsgRemoteURLShort), Args: cobra.ExactArgs(2),
		RunE: run(func(ctx context.Context, runner *git.Runner, root string, args []string) error {
			return runner.SetRemoteURL(ctx, root, args[0], args[1], push)
		}),
	}
	setURL.Flags().BoolVar(&push, "push", false, i18n.T(a.lang, i18n.MsgRemotePushFlag))
	cmd := &cobra.Command{Use: "remote", Short: i18n.T(a.lang, i18n.MsgRemoteShort), Args: cobra.NoArgs, RunE: list}
	cmd.AddCommand(
		&cobra.Command{Use: "list", Short: i18n.T(a.lang, i18n.MsgRemoteShort), Args: cobra.NoArgs, RunE: list},
		&cobra.Command{
			Use: "add <name> <url>", Short: i18n.T(a.lang, i18n.MsgRemoteAddShort), Args: cobra.ExactArgs(2),
			RunE: run(func(ctx context.Context, runner *git.Runner, root string, args []string) error {
				return runner.AddRemote(ctx, root, args[0], args[1])
			}),
		},
		&cobra.Command{
			Use: "remove <name>", Short: i18n.T(a.lang, i18n.MsgRemoteRemoveShort), Args: cobra.ExactArgs(1),
			RunE: run(func(ctx context.Context, runner *git.Runner, root string, args []string) error {
				return runner.RemoveRemote(ctx, root, args[0])
			}),
		},
		&cobra.Command{
			Use: "rename <old> <new>", Short: i18n.T(a.lang, i18n.MsgRemoteRenameShort), Args: cobra.ExactArgs(2),
			RunE: run(func(ctx context.Context, runner *git.Runner, root string, args []string) error {
				return runner.RenameRemote(ctx, root, args[0], args[1])
			}),
		},
		setURL,
	)
	return cmd
}

func (a *app) newSubmoduleCmd() *cobra.Command {
	status := func(cmd *cobra.Command, _ []string) error {
		runner, root, err := a.git()
		if err != nil {
			return err
		}
		subs, err := runner.Submodules(cmd.Context(), root)
		if err != nil {
			return err
		}
		if a.flags.json {
			if subs == nil {
				subs = []git.Submodule{}
			}
			return a.writeJSON(subs)
		}
		if len(subs) == 0 {
			a.say(i18n.MsgSubmoduleNone)
		}
		for _, s := range subs {
			note := ""
			switch {
			case !s.Initialized:
				note = " (" + i18n.T(a.lang, i18n.MsgSubmoduleNotInit) + ")"
			case s.Modified:
				note = " (" + i18n.T(a.lang, i18n.MsgSubmoduleModified) + ")"
			}
			fmt.Fprintf(a.env.Stdout, "%s  %s%s\n", short(s.Hash), s.Path, note)
		}
		return nil
	}
	cmd := &cobra.Command{Use: "submodule", Short: i18n.T(a.lang, i18n.MsgSubmoduleShort), Args: cobra.NoArgs, RunE: status}
	cmd.AddCommand(
		&cobra.Command{Use: "status", Short: i18n.T(a.lang, i18n.MsgSubmoduleShort), Args: cobra.NoArgs, RunE: status},
		&cobra.Command{
			Use: "update", Short: i18n.T(a.lang, i18n.MsgSubmoduleUpdate), Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				runner, root, err := a.git()
				if err != nil {
					return err
				}
				if err := runner.UpdateSubmodules(cmd.Context(), root); err != nil {
					return err
				}
				a.say(i18n.MsgDone)
				return nil
			},
		},
	)
	return cmd
}

func (a *app) newIdentityCmd() *cobra.Command {
	var global bool
	var id git.Identity
	cmd := &cobra.Command{
		Use: "identity", Short: i18n.T(a.lang, i18n.MsgIdentityShort), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := a.cfg(); err != nil {
				return err
			}
			runner, dir := a.session.runner, a.session.dir
			if a.session.repoRoot != "" {
				dir = a.session.repoRoot
			} else if id != (git.Identity{}) {
				// Outside a repository only the global identity can be set.
				global = true
			}
			if id != (git.Identity{}) {
				if err := runner.SetIdentity(cmd.Context(), dir, global, id); err != nil {
					return err
				}
			}
			current, err := runner.Identity(cmd.Context(), dir)
			if err != nil {
				return err
			}
			if a.flags.json {
				return a.writeJSON(current)
			}
			a.say(i18n.MsgIdentityShow, current.Name, current.Email)
			return nil
		},
	}
	cmd.Flags().BoolVar(&global, "global", false, i18n.T(a.lang, i18n.MsgIdentityGlobal))
	cmd.Flags().StringVar(&id.Name, "name", "", i18n.T(a.lang, i18n.MsgIdentityName))
	cmd.Flags().StringVar(&id.Email, "email", "", i18n.T(a.lang, i18n.MsgIdentityEmail))
	return cmd
}

func (a *app) newWorkspaceCmd() *cobra.Command {
	repos := func() ([]string, error) {
		c, err := a.cfg()
		return c.Workspace.Repos, err
	}
	save := func(list []string) error {
		path, err := config.GlobalPath(a.cfgEnv)
		if err != nil {
			return err
		}
		return config.SetGlobal(path, "workspace.repos", strings.Join(list, ","))
	}
	list := func(*cobra.Command, []string) error {
		r, err := repos()
		if err != nil {
			return err
		}
		if a.flags.json {
			return a.writeJSON(nonNil(r))
		}
		if len(r) == 0 {
			a.say(i18n.MsgWorkspaceEmpty)
		}
		for _, p := range r {
			fmt.Fprintln(a.env.Stdout, p)
		}
		return nil
	}
	// each runs fn in every repository, reporting failures without stopping.
	each := func(fn func(ctx context.Context, runner *git.Runner, dir string) (string, error)) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, _ []string) error {
			r, err := repos()
			if err != nil {
				return err
			}
			if len(r) == 0 {
				a.say(i18n.MsgWorkspaceEmpty)
				return nil
			}
			st := a.styleFor(a.env.Stdout)
			var failed error
			for _, dir := range r {
				line, err := fn(cmd.Context(), a.session.runner, dir)
				if err != nil {
					failed = errors.Join(failed, err)
					line = st.red(i18n.Describe(a.lang, err))
				}
				fmt.Fprintf(a.env.Stdout, "%s  %s\n", st.bold(filepath.Base(dir)), line)
			}
			if failed != nil {
				return &exitError{code: ExitError, silent: true, err: failed}
			}
			return nil
		}
	}
	cmd := &cobra.Command{Use: "workspace", Short: i18n.T(a.lang, i18n.MsgWorkspaceShort), Args: cobra.NoArgs, RunE: list}
	cmd.AddCommand(
		&cobra.Command{Use: "list", Short: i18n.T(a.lang, i18n.MsgWorkspaceListShort), Args: cobra.NoArgs, RunE: list},
		&cobra.Command{
			Use: "add [path]", Short: i18n.T(a.lang, i18n.MsgWorkspaceAddShort), Args: cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				dir := a.session.dir
				if len(args) == 1 {
					dir = a.target(args[0])
				}
				root, err := a.session.runner.TopLevel(cmd.Context(), dir)
				if err != nil {
					return err
				}
				if strings.Contains(root, ",") {
					return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgWorkspaceComma, root))
				}
				r, err := repos()
				if err != nil {
					return err
				}
				if !slices.Contains(r, root) {
					if err := save(append(r, root)); err != nil {
						return err
					}
				}
				a.say(i18n.MsgWorkspaceAdded, root)
				return nil
			},
		},
		&cobra.Command{
			Use: "remove <path>", Short: i18n.T(a.lang, i18n.MsgWorkspaceRemoveShort), Args: cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				r, err := repos()
				if err != nil {
					return err
				}
				target := a.target(args[0])
				kept := slices.DeleteFunc(slices.Clone(r), func(p string) bool { return p == target || p == args[0] })
				if err := save(kept); err != nil {
					return err
				}
				a.say(i18n.MsgWorkspaceRemoved, target)
				return nil
			},
		},
		&cobra.Command{
			Use: "status", Short: i18n.T(a.lang, i18n.MsgWorkspaceStatusShort), Args: cobra.NoArgs,
			RunE: each(func(ctx context.Context, runner *git.Runner, dir string) (string, error) {
				st, err := runner.Status(ctx, dir, git.StatusOptions{Untracked: git.UntrackedNormal})
				if err != nil {
					return "", err
				}
				b := st.Branch
				line := b.Head
				if b.Detached {
					line = short(b.OID)
				}
				if b.Upstream != "" {
					line += " " + i18n.T(a.lang, i18n.MsgBranchAheadBehind, b.Ahead, b.Behind)
				}
				if st.Clean() {
					return line + " · " + i18n.T(a.lang, i18n.MsgWorkspaceClean), nil
				}
				return line + " · " + i18n.T(a.lang, i18n.MsgWorkspaceDirty, len(st.Entries)), nil
			}),
		},
		&cobra.Command{
			Use: "fetch", Short: i18n.T(a.lang, i18n.MsgWorkspaceFetchShort), Args: cobra.NoArgs,
			RunE: each(func(ctx context.Context, runner *git.Runner, dir string) (string, error) {
				return i18n.T(a.lang, i18n.MsgFetchDone), runner.Fetch(ctx, dir, git.FetchOptions{All: true, Prune: true})
			}),
		},
		&cobra.Command{
			Use: "pull", Short: i18n.T(a.lang, i18n.MsgWorkspacePullShort), Args: cobra.NoArgs,
			// Pulling many repositories at once is only done the way that can
			// never rewrite or merge anything: fast-forward.
			RunE: each(func(ctx context.Context, runner *git.Runner, dir string) (string, error) {
				res, err := runner.Pull(ctx, dir, git.PullOptions{Strategy: git.PullFFOnly})
				if err != nil {
					return "", err
				}
				if !res.Updated {
					return i18n.T(a.lang, i18n.MsgPullUpToDate), nil
				}
				return i18n.T(a.lang, i18n.MsgPullDone, short(res.Before), short(res.After)), nil
			}),
		},
	)
	return cmd
}
