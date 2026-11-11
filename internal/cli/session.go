package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/muhammedeminalan/gitpal/internal/core/config"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
	"github.com/muhammedeminalan/gitpal/internal/core/safety"
	"github.com/muhammedeminalan/gitpal/internal/tui"
)

// errCanceled ends a command the user walked away from (exit code 2).
var errCanceled = &exitError{code: ExitCanceled, err: context.Canceled}

// errDryRun ends a command after its dry run; nothing more is printed.
var errDryRun = &exitError{code: ExitOK, silent: true}

// cfg returns the effective settings. A broken global config file stops every
// command except doctor, which exists to report it.
func (a *app) cfg() (config.Config, error) {
	if a.session.configErr != nil {
		return config.Config{}, a.session.configErr
	}
	return a.session.loaded.Config, nil
}

// repo returns the root of the repository the command works in.
func (a *app) repo() (string, error) {
	if _, err := a.cfg(); err != nil {
		return "", err
	}
	if a.session.repoRoot == "" {
		if a.session.repoErr != nil {
			return "", a.session.repoErr
		}
		return "", fmt.Errorf("%s", i18n.T(a.lang, i18n.MsgNotARepo, a.session.dir))
	}
	return a.session.repoRoot, nil
}

// git returns the runner and the repository root.
func (a *app) git() (*git.Runner, string, error) {
	root, err := a.repo()
	return a.session.runner, root, err
}

// policy turns the safety settings into the safety layer's policy.
func policy(c config.SafetyConfig) safety.Policy {
	return safety.Policy{
		ProtectedBranches: c.ProtectedBranches,
		LargeFileBytes:    int64(c.LargeFileMB) << 20,
		WarnOnSecrets:     c.WarnOnSecrets,
		BackupRetention:   time.Duration(c.BackupRetentionDays) * 24 * time.Hour,
	}
}

func (a *app) guard(ctx context.Context) (*safety.Guard, error) {
	runner, root, err := a.git()
	if err != nil {
		return nil, err
	}
	c, _ := a.cfg()
	return safety.NewGuard(ctx, runner, root, policy(c.Safety))
}

// interactive reports whether questions can be asked. --json is for scripts,
// so it never asks.
func (a *app) interactive() bool { return !a.flags.json && a.env.Interactive() }

// readLine asks a question on stderr and reads the answer from stdin. The end
// of input counts as walking away.
func (a *app) readLine(prompt string) (string, error) {
	fmt.Fprint(a.env.Stderr, prompt)
	if a.input == nil {
		a.input = bufio.NewReader(a.env.Stdin)
	}
	line, err := a.input.ReadString('\n')
	if err != nil && (line == "" || !errors.Is(err, io.EOF)) {
		fmt.Fprintln(a.env.Stderr)
		return "", errCanceled
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// yes reports whether an answer means yes. A dangerous operation takes only
// the whole word.
func (a *app) yes(answer string, wholeWord bool) bool {
	answer = strings.ToLower(strings.TrimSpace(answer))
	accepted := strings.Split(i18n.T(a.lang, i18n.MsgYesAnswers), ",")
	if wholeWord {
		accepted = slices.DeleteFunc(accepted, func(s string) bool { return len(s) < 2 })
	}
	return slices.Contains(accepted, answer)
}

// approver asks the user in the way the risk calls for. It is nil when
// nobody can be asked; the safety layer then refuses whatever needs asking.
func (a *app) approver(shown *bool) safety.Approver {
	if !a.interactive() {
		return nil
	}
	return func(as safety.Assessment) (safety.Answer, error) {
		*shown = true
		a.printAssessment(a.env.Stderr, as)
		switch as.Confirm {
		case safety.ConfirmTypeBranch:
			line, err := a.readLine(i18n.T(a.lang, i18n.MsgConfirmTypeBranch, as.ConfirmText))
			return safety.Answer{Approved: true, Typed: strings.TrimSpace(line)}, err
		case safety.ConfirmExplicit:
			line, err := a.readLine(i18n.T(a.lang, i18n.MsgConfirmExplicit))
			return safety.Answer{Approved: a.yes(line, true)}, err
		default:
			line, err := a.readLine(i18n.T(a.lang, i18n.MsgConfirmQuestion))
			return safety.Answer{Approved: a.yes(line, false)}, err
		}
	}
}

// runOp sends an operation through the safety layer: the dry run under
// --dry-run, otherwise confirmation, backup, the operation and its record.
func (a *app) runOp(ctx context.Context, op safety.Operation) (safety.Result, error) {
	g, err := a.guard(ctx)
	if err != nil {
		return safety.Result{}, err
	}
	assessment, err := g.Assess(ctx, op)
	if err != nil {
		return safety.Result{}, err
	}
	if a.flags.dryRun {
		if a.flags.json {
			if err := a.writeJSON(assessmentJSON(assessment)); err != nil {
				return safety.Result{}, err
			}
		} else {
			a.printAssessment(a.env.Stdout, assessment)
			fmt.Fprintln(a.env.Stdout, i18n.T(a.lang, i18n.MsgDryRunNothingDone))
		}
		if assessment.Blocked() {
			return safety.Result{}, &exitError{code: ExitBlocked, silent: true}
		}
		return safety.Result{}, errDryRun
	}
	if assessment.Blocked() {
		return safety.Result{}, &safety.BlockedError{Assessment: assessment}
	}

	shown := false
	approve := a.approver(&shown)
	// Warnings are shown even when nothing needs confirming.
	if approve == nil || assessment.Confirm == safety.ConfirmNone ||
		(assessment.Confirm == safety.ConfirmSimple && a.flags.yes) {
		a.printFindings(a.env.Stderr, assessment)
	}
	res, err := g.Do(ctx, op, approve, safety.DoOptions{AssumeYes: a.flags.yes})
	if err == nil {
		a.printBackups(res)
	}
	return res, err
}

func (a *app) printAssessment(w io.Writer, as safety.Assessment) {
	st := a.styleFor(w)
	s := as.Summary(a.lang)
	risk := s.Risk
	switch as.Risk {
	case safety.RiskDangerous, safety.RiskCritical:
		risk = st.red(st.bold(risk))
	case safety.RiskCaution:
		risk = st.yellow(risk)
	case safety.RiskSafe:
		risk = st.green(risk)
	}
	fmt.Fprintf(w, "%s: %s\n", i18n.T(a.lang, i18n.MsgRiskLabel), risk)
	if len(s.Effects) > 0 {
		fmt.Fprintln(w, i18n.T(a.lang, i18n.MsgEffectsTitle))
		for _, line := range s.Effects {
			fmt.Fprintf(w, "  - %s\n", line)
		}
	}
	a.printFindings(w, as)
	for _, line := range s.Backups {
		fmt.Fprintf(w, "%s\n", st.dim(line))
	}
}

func (a *app) printFindings(w io.Writer, as safety.Assessment) {
	if len(as.Findings) == 0 {
		return
	}
	st := a.styleFor(w)
	fmt.Fprintln(w, i18n.T(a.lang, i18n.MsgFindingsTitle))
	for _, f := range as.Findings {
		mark := st.yellow("!")
		switch f.Severity {
		case safety.SeverityBlock:
			mark = st.red("✗")
		case safety.SeverityInfo:
			mark = st.dim("·")
		case safety.SeverityWarn:
		}
		fmt.Fprintf(w, "  %s %s\n", mark, f.Localize(a.lang))
	}
}

func (a *app) printBackups(res safety.Result) {
	for _, ref := range []string{res.Entry.Backup, res.Entry.WorktreeBackup} {
		if ref != "" && !a.flags.json {
			fmt.Fprintln(a.env.Stderr, a.styleFor(a.env.Stderr).dim(i18n.T(a.lang, i18n.MsgBackupCreated, ref)))
		}
	}
}

// assessmentJSON is the shape of a dry run under --json.
type dryRunOutput struct {
	Operation string   `json:"operation"`
	Risk      string   `json:"risk"`
	Blocked   bool     `json:"blocked"`
	Confirm   string   `json:"confirm"`
	Effects   []string `json:"effects"`
	Findings  []string `json:"findings"`
	Backups   []string `json:"backups"`
}

func assessmentJSON(as safety.Assessment) dryRunOutput {
	s := as.Summary(i18n.EN)
	confirm := map[safety.Confirmation]string{
		safety.ConfirmNone: "none", safety.ConfirmSimple: "simple",
		safety.ConfirmExplicit: "explicit", safety.ConfirmTypeBranch: "type-branch",
	}[as.Confirm]
	return dryRunOutput{
		Operation: string(as.Op.Kind), Risk: as.Risk.String(), Blocked: as.Blocked(), Confirm: confirm,
		Effects: nonNil(s.Effects), Findings: nonNil(s.Findings), Backups: nonNil(s.Backups),
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// pick shows the file picker; the user walking away is a cancel.
func (a *app) pick(ctx context.Context, title string, items []tui.Item) ([]string, error) {
	chosen, err := a.env.Pick(ctx, tui.PickOptions{
		Title: title, Help: i18n.T(a.lang, i18n.MsgTUIHelpPicker), Items: items, In: a.env.Stdin, Out: a.env.Stderr,
	})
	if errors.Is(err, tui.ErrCanceled) {
		return nil, errCanceled
	}
	return chosen, err
}

// openEditor opens path in $VISUAL or $EDITOR, the way git does.
func openEditor(ctx context.Context, path string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		return errNoEditor
	}
	// The editor setting may carry arguments ("code --wait"); they are split
	// on spaces, never handed to a shell.
	fields := strings.Fields(editor)
	cmd := exec.CommandContext(ctx, fields[0], append(fields[1:], path)...) //nolint:gosec,forbidigo // the user's own editor
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

var errNoEditor = errors.New("no editor configured")

func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

// ErrNeedsYes stops a confirmable action that nobody can confirm.
var ErrNeedsYes = safety.ErrConfirmationRequired

func safetySwitch(branch string) safety.Operation { return safety.Switch(branch) }
