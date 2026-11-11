package git

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// git asks an editor for two things during an interactive rebase: the todo
// list, and commit messages (reword, squash). gitpal answers both itself by
// naming its own executable as the editor, with one of these arguments; the
// answers come from files gitpal wrote beforehand. No user input is ever
// part of the editor command line.
const (
	editorTodoArg    = "__gitpal-sequence-editor"
	editorMessageArg = "__gitpal-message-editor"

	// envTodoFile names the file holding the todo list to hand to git.
	envTodoFile = "GITPAL_REBASE_TODO"
	// envMessageDir names a directory of numbered files: the commit
	// messages, in the order git asks for them.
	envMessageDir = "GITPAL_REBASE_MESSAGES"
)

// RunEditorHelper handles gitpal being started by git as an editor. Call it
// first thing in main: when handled is true, exit with code.
func RunEditorHelper(args []string) (handled bool, code int) {
	if len(args) != 2 || (args[0] != editorTodoArg && args[0] != editorMessageArg) {
		return false, 0
	}
	var err error
	if args[0] == editorTodoArg {
		err = copyFile(os.Getenv(envTodoFile), args[1])
	} else {
		err = nextMessage(os.Getenv(envMessageDir), args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitpal editor:", err)
		return true, 1
	}
	return true, 0
}

func copyFile(from, to string) error {
	if from == "" {
		return errors.New("no todo list")
	}
	data, err := os.ReadFile(from) //nolint:gosec // G304: a file gitpal wrote itself
	if err != nil {
		return err
	}
	return os.WriteFile(to, data, 0o600) //nolint:gosec // G703: the file git asked to have edited
}

// nextMessage writes the next queued message over the file git opened, and
// marks it used. An empty queued message keeps what git proposed.
func nextMessage(dir, to string) error {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var queue []int
	for _, e := range entries {
		if n, err := strconv.Atoi(e.Name()); err == nil {
			queue = append(queue, n)
		}
	}
	if len(queue) == 0 {
		return nil
	}
	sort.Ints(queue)
	path := filepath.Join(dir, strconv.Itoa(queue[0]))
	data, err := os.ReadFile(path) //nolint:gosec // G304: a file gitpal wrote itself
	if err != nil {
		return err
	}
	if err := os.Rename(path, path+".used"); err != nil && !errors.Is(err, fs.ErrNotExist) { //nolint:gosec // G703: gitpal's own temp dir
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return os.WriteFile(to, data, 0o600) //nolint:gosec // G703: the file git asked to have edited
}

// editorCommand is the editor value given to git: gitpal's executable,
// quoted for the shell git runs it with, and the role argument.
func (r *Runner) editorCommand(role string) (string, error) {
	self := r.self
	if self == "" {
		var err error
		if self, err = os.Executable(); err != nil {
			return "", fmt.Errorf("find gitpal's own executable: %w", err)
		}
	}
	return shellQuote(filepath.ToSlash(self)) + " " + role, nil
}
