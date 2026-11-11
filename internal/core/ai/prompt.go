package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/i18n"
)

// commitSystem tells the model what a good commit message is (spec §6.2).
const commitSystem = `You write git commit messages for the change a developer is about to commit.

Rules:
- The subject line is at most %d characters, in the imperative mood, without a trailing period.
- %s
- Write the message in %s.
- If the change needs explaining, add a body after a blank line. The body says why the change was made, not what each line does; the diff already shows that.
- Match the style of the repository's recent commit messages where they are consistent.
- If the staged change mixes unrelated work, say which files belong together in "split" so the developer can commit them separately; otherwise leave "split" empty.
- Parts of the diff may read "[REDACTED]" or say content was withheld. Do not guess what was there and do not mention it.

Reply with JSON only, no other text: {"messages": [%d alternative commit messages, best first], "split": "advice or empty"}`

var styleRules = map[Style]string{
	StyleConventional: `Use the Conventional Commits format: "type(scope): subject" with a type such as feat, fix, refactor, docs, test, chore, perf, build or ci. The scope is optional.`,
	StyleFree:         `Use no prefix convention; write a plain, descriptive subject line.`,
}

var languageNames = map[i18n.Lang]string{i18n.TR: "Turkish", i18n.EN: "English"}

func commitPrompt(in CommitInput) (system, user string) {
	n := min(max(in.Suggestions, 1), 3)
	maxSubject := in.MaxSubjectLength
	if maxSubject <= 0 {
		maxSubject = 72
	}
	style := styleRules[in.Style]
	if style == "" {
		style = styleRules[StyleConventional]
	}
	lang := languageNames[in.Language]
	if lang == "" {
		lang = "English"
	}
	system = fmt.Sprintf(commitSystem, maxSubject, style, lang, n)

	var b strings.Builder
	if in.Branch != "" {
		fmt.Fprintf(&b, "Branch: %s\n\n", Redact(in.Branch))
	}
	if len(in.RecentMessages) > 0 {
		b.WriteString("Recent commit messages in this repository:\n")
		for _, m := range in.RecentMessages {
			b.WriteString("---\n" + Redact(m) + "\n")
		}
		b.WriteString("---\n\n")
	}
	b.WriteString(in.Payload.Text)
	return system, b.String()
}

const prSystem = `You write the title and description of a pull request from its commits and diff.

Rules:
- The title is at most 72 characters and says what the pull request achieves.
- The description explains why the change is needed, summarizes what changed, and lists anything a reviewer should look at closely. Use Markdown.
- Write in %s.
- Parts of the diff may read "[REDACTED]" or say content was withheld. Do not guess what was there.

Reply with JSON only, no other text: {"title": "...", "body": "..."}`

func prPrompt(in PRInput) (system, user string) {
	lang := languageNames[in.Language]
	if lang == "" {
		lang = "English"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Branch %s into %s.\n\nCommits:\n", Redact(in.Branch), Redact(in.Base))
	for _, c := range in.Commits {
		b.WriteString("- " + Redact(c) + "\n")
	}
	b.WriteString("\n" + in.Payload.Text)
	return fmt.Sprintf(prSystem, lang), b.String()
}

// jsonObject returns the first {...} object in text: models sometimes wrap
// their JSON in a code fence or a sentence.
func jsonObject(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return ""
	}
	return text[start : end+1]
}

// parseCommit reads the model's answer. A reply that is not the requested
// JSON but is plain text is taken as a single message.
func parseCommit(text string, maxSubject int) CommitSuggestions {
	var raw struct {
		Messages []string `json:"messages"`
		Split    string   `json:"split"`
	}
	var out CommitSuggestions
	if obj := jsonObject(text); obj != "" && json.Unmarshal([]byte(obj), &raw) == nil {
		out.Split = strings.TrimSpace(raw.Split)
		for _, m := range raw.Messages {
			if m = cleanMessage(m); m != "" {
				out.Messages = append(out.Messages, m)
			}
		}
		return out
	}
	if m := cleanMessage(text); m != "" && !strings.Contains(m, "{") {
		subject, _, _ := strings.Cut(m, "\n")
		if maxSubject <= 0 || len([]rune(subject)) <= maxSubject*2 {
			out.Messages = []string{m}
		}
	}
	return out
}

func cleanMessage(m string) string {
	m = strings.TrimSpace(m)
	m = strings.TrimPrefix(m, "```")
	m = strings.TrimSuffix(m, "```")
	return strings.TrimSpace(m)
}

func parsePR(text string) PRText {
	var raw PRText
	var parsed struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if obj := jsonObject(text); obj != "" && json.Unmarshal([]byte(obj), &parsed) == nil {
		raw.Title, raw.Body = strings.TrimSpace(parsed.Title), strings.TrimSpace(parsed.Body)
	}
	return raw
}
