package cli

import "context"

// aiByDefault reports whether commit messages are suggested by the AI unless
// the user asks otherwise. Filled in with the AI layer (M4).
func (a *app) aiByDefault() bool { return false }

// aiMessage asks the AI for a commit message. An empty result with no error
// means "fall back to typing one". Filled in with the AI layer (M4).
func (a *app) aiMessage(context.Context, bool) (string, error) { return "", nil }
