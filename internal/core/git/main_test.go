package git

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"
)

// The test binary doubles as a stand-in for the git executable: a Runner
// pointed at os.Args[0] with GITPAL_FAKE_GIT set starts this process again,
// which then behaves as told instead of running tests. That gives runner tests
// a child process whose arguments, environment and timing they fully control,
// on every platform, without mocking os/exec.
const fakeGitEnv = "GITPAL_FAKE_GIT"

// fakeReport is what the "report" mode prints: everything the child was given.
type fakeReport struct {
	Args  []string          `json:"args"`
	Dir   string            `json:"dir"`
	Stdin string            `json:"stdin"`
	Env   map[string]string `json:"env"`
}

func TestMain(m *testing.M) {
	// git starts the test binary as its editor during interactive rebases.
	if handled, code := RunEditorHelper(os.Args[1:]); handled {
		os.Exit(code)
	}
	if mode := os.Getenv(fakeGitEnv); mode != "" {
		os.Exit(runFakeGit(mode))
	}
	os.Exit(m.Run())
}

func runFakeGit(mode string) int {
	switch mode {
	case "report":
		stdin, _ := io.ReadAll(os.Stdin)
		dir, _ := os.Getwd()
		env := map[string]string{}
		for _, key := range []string{
			"LC_MESSAGES", "LANGUAGE", "GIT_TERMINAL_PROMPT", "GIT_EDITOR",
			"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR",
			"GIT_SSH_COMMAND", "GITPAL_TEST_VALUE",
		} {
			if value, ok := os.LookupEnv(key); ok {
				env[key] = value
			}
		}
		report := fakeReport{Args: os.Args[1:], Dir: dir, Stdin: string(stdin), Env: env}
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			return 70
		}
		return 0
	case "hang":
		time.Sleep(time.Minute)
		return 0
	case "fail":
		fmt.Fprint(os.Stdout, os.Getenv("GITPAL_FAKE_STDOUT"))
		fmt.Fprint(os.Stderr, os.Getenv("GITPAL_FAKE_STDERR"))
		code, err := strconv.Atoi(os.Getenv("GITPAL_FAKE_EXIT"))
		if err != nil {
			return 70
		}
		return code
	default:
		fmt.Fprintf(os.Stderr, "unknown fake git mode %q\n", mode)
		return 70
	}
}
