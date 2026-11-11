package cli

import (
	"context"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
	"github.com/muhammedeminalan/gitpal/internal/tui"
)

// runDashboard opens the interactive screen `gitpal` shows without a
// subcommand.
func (a *app) runDashboard(ctx context.Context) error {
	runner, root, err := a.git()
	if err != nil {
		return err
	}
	g, err := a.guard(ctx)
	if err != nil {
		return err
	}
	c, _ := a.cfg()
	return a.env.Dashboard(ctx, tui.DashboardOptions{
		Lang: a.lang, Git: runner, Dir: root, Guard: g, Pull: git.PullStrategy(c.Pull.Strategy),
		In: a.env.Stdin, Out: a.env.Stdout,
	})
}
