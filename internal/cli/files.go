package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/gitignore"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
	"github.com/muhammedeminalan/gitpal/internal/tui"
)

// changeItems lists the files a commit could take, for the picker: every
// change, with what is already staged chosen.
func changeItems(st git.Status, chooseAll bool) []tui.Item {
	var items []tui.Item
	for _, e := range st.Entries {
		if e.Kind == git.EntryIgnored {
			continue
		}
		mark := "?"
		switch {
		case e.Conflicted():
			mark = "U"
		case e.Staged():
			mark = string(e.Index)
		case e.Kind != git.EntryUntracked:
			mark = string(e.Worktree)
		}
		label := mark + "  " + e.Path
		if e.OrigPath != "" {
			label = mark + "  " + e.OrigPath + " → " + e.Path
		}
		items = append(items, tui.Item{Label: label, Value: e.Path, Selected: chooseAll || e.Staged()})
	}
	return items
}

// stagePaths adds paths to the index, and for a rename also its source, so
// the rename is staged whole.
func stagePaths(st git.Status, paths []string) []string {
	out := append([]string(nil), paths...)
	for _, e := range st.Entries {
		if e.OrigPath != "" && contains(paths, e.Path) {
			out = append(out, e.OrigPath)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// pickHunks lets the user choose hunks of the files' diff and applies them:
// staging them, or with unstage, taking them out of the index.
func (a *app) pickHunks(ctx context.Context, paths []string, unstage bool) error {
	runner, root, err := a.git()
	if err != nil {
		return err
	}
	if !a.interactive() {
		return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgNeedsTerminal, "gitpal add <yol>"))
	}
	files, err := runner.Diff(ctx, root, git.DiffOptions{Staged: unstage, Paths: paths})
	if err != nil {
		return err
	}
	var items []tui.Item
	for fi, f := range files {
		if f.Binary || f.Change != git.ChangeModified || f.OldPath != "" {
			continue
		}
		for hi, h := range f.Hunks {
			label := fmt.Sprintf("%s @@ -%d +%d @@ %s", f.Path, h.OldStart, h.NewStart, h.Section)
			for _, l := range h.Lines {
				if l.Kind == git.LineAdded || l.Kind == git.LineDeleted {
					label += "\n      " + string(l.Kind) + strings.TrimSuffix(l.Text, "\r")
				}
			}
			items = append(items, tui.Item{Label: label, Value: fmt.Sprintf("%d:%d", fi, hi)})
		}
	}
	if len(items) == 0 {
		a.say(i18n.MsgDiffNoChanges)
		return nil
	}
	chosen, err := a.pick(ctx, i18n.T(a.lang, i18n.MsgHunkPickTitle), items)
	if err != nil {
		return err
	}
	byFile := map[int][]int{}
	for _, v := range chosen {
		var fi, hi int
		if _, err := fmt.Sscanf(v, "%d:%d", &fi, &hi); err == nil {
			byFile[fi] = append(byFile[fi], hi)
		}
	}
	for fi, hunks := range byFile {
		if unstage {
			err = runner.UnstageHunks(ctx, root, files[fi], hunks)
		} else {
			err = runner.StageHunks(ctx, root, files[fi], hunks)
		}
		if err != nil {
			return err
		}
	}
	a.say(i18n.MsgHunksStaged, len(chosen))
	return nil
}

func (a *app) newAddCmd() *cobra.Command {
	var all, patch bool
	cmd := &cobra.Command{
		Use:   "add [paths...]",
		Short: i18n.T(a.lang, i18n.MsgAddShort),
		RunE: func(cmd *cobra.Command, paths []string) error {
			ctx := cmd.Context()
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			if all {
				return runner.StageAll(ctx, root)
			}
			if patch {
				return a.pickHunks(ctx, paths, false)
			}
			st, err := runner.Status(ctx, root, git.StatusOptions{})
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				if !a.interactive() {
					return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgNeedsTerminal, "gitpal add <yol> | gitpal add --all"))
				}
				items := changeItems(st, false)
				if len(items) == 0 {
					fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgStatusClean))
					return nil
				}
				if paths, err = a.pick(ctx, i18n.T(a.lang, i18n.MsgAddPickTitle), items); err != nil {
					return err
				}
				if len(paths) == 0 {
					fmt.Fprintln(a.env.Stderr, i18n.T(a.lang, i18n.MsgNothingSelected))
					return nil
				}
			}
			if err := runner.Stage(ctx, root, stagePaths(st, paths)); err != nil {
				return err
			}
			if !a.flags.json {
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgAddDone, len(paths)))
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "A", false, i18n.T(a.lang, i18n.MsgAddAllFlag))
	cmd.Flags().BoolVarP(&patch, "patch", "p", false, i18n.T(a.lang, i18n.MsgPatchFlag))
	return cmd
}

func (a *app) newUnstageCmd() *cobra.Command {
	var patch bool
	cmd := &cobra.Command{
		Use:   "unstage [paths...]",
		Short: i18n.T(a.lang, i18n.MsgUnstageShort),
		RunE: func(cmd *cobra.Command, paths []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			if patch {
				return a.pickHunks(cmd.Context(), paths, true)
			}
			if len(paths) == 0 {
				err = runner.UnstageAll(cmd.Context(), root)
			} else {
				err = runner.Unstage(cmd.Context(), root, paths)
			}
			if err == nil && !a.flags.json {
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgUnstageDone))
			}
			return err
		},
	}
	cmd.Flags().BoolVarP(&patch, "patch", "p", false, i18n.T(a.lang, i18n.MsgPatchFlag))
	return cmd
}

func (a *app) newDiscardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discard <paths...>",
		Short: i18n.T(a.lang, i18n.MsgDiscardShort),
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, paths []string) error {
			if _, err := a.runOp(cmd.Context(), safety.Discard(paths...)); err != nil {
				return err
			}
			if !a.flags.json {
				fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgDiscardDone))
			}
			return nil
		},
	}
}

func (a *app) newIgnoreCmd() *cobra.Command {
	var templates []string
	var listTemplates bool
	cmd := &cobra.Command{
		Use:   "ignore [paths...]",
		Short: i18n.T(a.lang, i18n.MsgIgnoreShort),
		RunE: func(cmd *cobra.Command, paths []string) error {
			if listTemplates {
				for _, name := range gitignore.Names() {
					fmt.Fprintln(a.env.Stdout, name)
				}
				return nil
			}
			if len(paths) == 0 && len(templates) == 0 {
				return cmd.Help()
			}
			root, err := a.repo()
			if err != nil {
				return err
			}
			if len(templates) > 0 {
				if err := gitignore.Apply(root, templates); err != nil {
					if errors.Is(err, gitignore.ErrUnknownTemplate) {
						return fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgTemplateUnknown, strings.Join(templates, ",")))
					}
					return err
				}
				a.say(i18n.MsgTemplateAdded, strings.Join(templates, ", "))
			}
			if len(paths) > 0 {
				if err := git.IgnorePaths(root, paths); err != nil {
					return err
				}
				a.say(i18n.MsgIgnoreDone)
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&templates, "template", "t", nil, i18n.T(a.lang, i18n.MsgTemplateFlag))
	cmd.Flags().BoolVar(&listTemplates, "templates", false, i18n.T(a.lang, i18n.MsgTemplatesFlag))
	return cmd
}

func (a *app) newDiffCmd() *cobra.Command {
	var staged bool
	cmd := &cobra.Command{
		Use:   "diff [paths...]",
		Short: i18n.T(a.lang, i18n.MsgDiffShort),
		RunE: func(cmd *cobra.Command, paths []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			opts := git.DiffOptions{Staged: staged, Paths: paths}
			if a.flags.json {
				files, err := runner.Diff(cmd.Context(), root, opts)
				if err != nil {
					return err
				}
				if files == nil {
					files = []git.FileDiff{}
				}
				return a.writeJSON(files)
			}
			files, err := runner.Diff(cmd.Context(), root, opts)
			if err != nil {
				return err
			}
			a.printDiff(files)
			return nil
		},
	}
	cmd.Flags().BoolVar(&staged, "staged", false, i18n.T(a.lang, i18n.MsgDiffStaged))
	return cmd
}

func (a *app) printDiff(files []git.FileDiff) {
	w := a.env.Stdout
	st := a.styleFor(w)
	if len(files) == 0 {
		fmt.Fprintln(w, i18n.T(a.lang, i18n.MsgDiffNoChanges))
		return
	}
	for _, f := range files {
		name := f.Path
		if f.OldPath != "" {
			name = f.OldPath + " → " + f.Path
		}
		fmt.Fprintln(w, st.bold(fmt.Sprintf("%s (%s)", name, f.Change)))
		if f.Binary {
			fmt.Fprintln(w, "  "+st.dim(i18n.T(a.lang, i18n.MsgDiffBinary)))
		}
		for _, h := range f.Hunks {
			fmt.Fprintln(w, st.dim(fmt.Sprintf("@@ -%d,%d +%d,%d @@ %s", h.OldStart, h.OldLines, h.NewStart, h.NewLines, h.Section)))
			for _, l := range h.Lines {
				text := string(l.Kind) + strings.TrimSuffix(l.Text, "\r")
				switch l.Kind {
				case git.LineAdded:
					text = st.green(text)
				case git.LineDeleted:
					text = st.red(text)
				case git.LineNoNewline:
					text = st.dim(text)
				case git.LineContext:
				}
				fmt.Fprintln(w, text)
			}
		}
		fmt.Fprintln(w)
	}
}
