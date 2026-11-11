//go:build !gui

// Command gitpal-gui is the desktop interface of gitpal. This build has no
// interface in it: build with `make gui` (the gui tag and the built
// frontend are needed).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "gitpal-gui: built without the interface; build with `make gui`")
	os.Exit(1)
}
