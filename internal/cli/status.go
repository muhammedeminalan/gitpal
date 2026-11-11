package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// statusOutput is the shape of `gitpal status --json`.
type statusOutput struct {
	Branch    string        `json:"branch"`
	Detached  bool          `json:"detached"`
	Unborn    bool          `json:"unborn"`
	Head      string        `json:"head"`
	Upstream  string        `json:"upstream"`
	Gone      bool          `json:"upstream_gone"`
	Ahead     int           `json:"ahead"`
	Behind    int           `json:"behind"`
	Operation string        `json:"operation"`
	Clean     bool          `json:"clean"`
	Files     []statusEntry `json:"files"`
}

type statusEntry struct {
	Path       string `json:"path"`
	OrigPath   string `json:"orig_path,omitempty"`
	Index      string `json:"index"`
	Worktree   string `json:"worktree"`
	Kind       string `json:"kind"`
	Staged     bool   `json:"staged"`
	Unstaged   bool   `json:"unstaged"`
	Conflicted bool   `json:"conflicted"`
}

func (a *app) newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: i18n.T(a.lang, i18n.MsgStatusShort),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner, root, err := a.git()
			if err != nil {
				return err
			}
			st, err := runner.Status(cmd.Context(), root, git.StatusOptions{})
			if err != nil {
				return err
			}
			if a.flags.json {
				return a.writeJSON(statusJSON(st))
			}
			a.printStatus(a.env.Stdout, st)
			return nil
		},
	}
}

func statusJSON(st git.Status) statusOutput {
	b := st.Branch
	out := statusOutput{
		Branch: b.Head, Detached: b.Detached, Unborn: b.Unborn, Head: b.OID, Upstream: b.Upstream,
		Gone: b.UpstreamGone, Ahead: b.Ahead, Behind: b.Behind, Operation: string(st.Operation),
		Clean: st.Clean(), Files: []statusEntry{},
	}
	for _, e := range st.Entries {
		out.Files = append(out.Files, statusEntry{
			Path: e.Path, OrigPath: e.OrigPath, Index: stateText(e.Index), Worktree: stateText(e.Worktree),
			Kind: string(e.Kind), Staged: e.Staged(), Unstaged: e.Unstaged(), Conflicted: e.Conflicted(),
		})
	}
	return out
}

func stateText(s git.FileState) string {
	if s == 0 {
		return ""
	}
	return string(s)
}

func (a *app) printBranchLine(w io.Writer, st git.Status) {
	style := a.styleFor(w)
	loc := i18n.For(a.lang)
	b := st.Branch
	switch {
	case b.Unborn:
		fmt.Fprintln(w, style.bold(loc.T(i18n.MsgStatusUnborn, b.Head)))
	case b.Detached:
		fmt.Fprintln(w, style.yellow(loc.T(i18n.MsgStatusDetached, short(b.OID))))
	default:
		fmt.Fprintln(w, style.bold(loc.T(i18n.MsgStatusOnBranch, b.Head)))
	}
	switch {
	case b.Detached:
	case b.UpstreamGone:
		fmt.Fprintln(w, style.yellow(loc.T(i18n.MsgStatusGone, b.Upstream)))
	case b.Upstream != "":
		fmt.Fprintln(w, loc.T(i18n.MsgStatusUpstream, b.Upstream, b.Ahead, b.Behind))
	default:
		fmt.Fprintln(w, style.dim(loc.T(i18n.MsgStatusNoUpstream)))
	}
	if st.Operation != git.OpNone {
		fmt.Fprintln(w, style.red(loc.T(i18n.MsgStatusOperation, st.Operation)))
	}
}

func (a *app) printStatus(w io.Writer, st git.Status) {
	style := a.styleFor(w)
	loc := i18n.For(a.lang)
	a.printBranchLine(w, st)
	fmt.Fprintln(w)
	if st.Clean() {
		fmt.Fprintln(w, loc.T(i18n.MsgStatusClean))
		return
	}
	section := func(title string, entries []git.StatusEntry, mark func(git.StatusEntry) string, color func(string) string) {
		if len(entries) == 0 {
			return
		}
		fmt.Fprintln(w, title)
		for _, e := range entries {
			name := e.Path
			if e.OrigPath != "" {
				name = e.OrigPath + " → " + e.Path
			}
			fmt.Fprintf(w, "  %s  %s\n", color(mark(e)), name)
		}
		fmt.Fprintln(w)
	}
	section(loc.T(i18n.MsgStatusConflicted), st.Conflicted(), func(git.StatusEntry) string { return "U" }, style.red)
	section(loc.T(i18n.MsgStatusStaged), st.Staged(), func(e git.StatusEntry) string { return string(e.Index) }, style.green)
	section(loc.T(i18n.MsgStatusUnstaged), st.Unstaged(), func(e git.StatusEntry) string { return string(e.Worktree) }, style.red)
	section(loc.T(i18n.MsgStatusUntracked), st.Untracked(), func(git.StatusEntry) string { return "?" }, style.dim)
}
