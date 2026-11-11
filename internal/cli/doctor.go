package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muhammedeminalan/gitpal/internal/core/doctor"
	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// doctorOutput is the shape of `gitpal doctor --json`.
type doctorOutput struct {
	Healthy bool           `json:"healthy"`
	Summary doctorSummary  `json:"summary"`
	Checks  []doctor.Check `json:"checks"`
}

type doctorSummary struct {
	OK   int `json:"ok"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
}

func (a *app) newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: i18n.T(a.lang, i18n.MsgDoctorShort),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s := a.session
			report := doctor.Run(cmd.Context(), doctor.Options{
				Git:       s.runner,
				Dir:       s.dir,
				Lang:      a.lang,
				Config:    s.loaded,
				ConfigErr: s.configErr,
				Secrets:   a.env.Secrets,
				LookPath:  a.env.LookPath,
			})

			if a.flags.json {
				if err := a.writeJSON(doctorOutput{
					Healthy: report.Healthy(),
					Summary: doctorSummary{
						OK:   report.Count(doctor.StatusOK),
						Warn: report.Count(doctor.StatusWarn),
						Fail: report.Count(doctor.StatusFail),
					},
					Checks: report.Checks,
				}); err != nil {
					return err
				}
			} else {
				a.printDoctor(report)
			}

			if !report.Healthy() {
				// The report itself is the error message.
				return &exitError{code: ExitError, silent: true}
			}
			return nil
		},
	}
}

func (a *app) printDoctor(report doctor.Report) {
	out := a.env.Stdout
	style := a.styleFor(out)
	loc := i18n.For(a.lang)

	fmt.Fprintf(out, "%s\n\n", style.bold(loc.T(i18n.MsgDoctorTitle)))
	for _, check := range report.Checks {
		fmt.Fprintf(out, "  %s %s\n", statusMark(style, check.Status), check.Message)
		for _, detail := range check.Details {
			fmt.Fprintf(out, "      - %s\n", detail)
		}
		if check.Hint != "" {
			fmt.Fprintf(out, "      %s %s\n", style.dim(loc.T(i18n.MsgHintPrefix)+":"), check.Hint)
		}
	}
	fmt.Fprintf(out, "\n%s\n", loc.T(i18n.MsgDoctorSummary,
		report.Count(doctor.StatusOK), report.Count(doctor.StatusWarn), report.Count(doctor.StatusFail)))
}

func statusMark(style style, status doctor.Status) string {
	switch status {
	case doctor.StatusOK:
		return style.green("✓")
	case doctor.StatusWarn:
		return style.yellow("!")
	default:
		return style.red("✗")
	}
}
