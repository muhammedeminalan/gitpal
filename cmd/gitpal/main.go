// Command gitpal runs the everyday git workflow from one place, with a safety
// layer in front of the operations that are hard to undo.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/muhammedeminalan/gitpal/internal/cli"
	"github.com/muhammedeminalan/gitpal/internal/core/git"
)

func main() {
	// git starts gitpal as its editor during an interactive rebase.
	if handled, code := git.RunEditorHelper(os.Args[1:]); handled {
		os.Exit(code)
	}
	os.Exit(run())
}

func run() int {
	// Ctrl-C cancels the context, which stops any git command in flight.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, os.Args[1:], cli.SystemEnv())
}
