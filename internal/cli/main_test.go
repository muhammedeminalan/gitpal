package cli_test

import (
	"os"
	"testing"

	"github.com/muhammedeminalan/gitpal/internal/core/git"
)

func TestMain(m *testing.M) {
	// During an interactive rebase git starts the running program — this
	// test binary — as its editor, as it would start gitpal.
	if handled, code := git.RunEditorHelper(os.Args[1:]); handled {
		os.Exit(code)
	}
	os.Exit(m.Run())
}
