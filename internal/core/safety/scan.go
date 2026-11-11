package safety

import (
	"path"
	"regexp"
	"strings"
)

// secretPattern recognizes one kind of credential in a line of text.
type secretPattern struct {
	label string
	re    *regexp.Regexp
}

// secretPatterns are tried in order; the first match names the secret. More
// specific patterns come first (an Anthropic key also looks like an OpenAI
// key).
var secretPatterns = []secretPattern{
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY( BLOCK)?-----`)},
	{"AWS access key", regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"GitHub token", regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{22,})`)},
	{"Anthropic API key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}`)},
	{"OpenAI API key", regexp.MustCompile(`\bsk-(proj-)?[A-Za-z0-9_-]{32,}`)},
	{"Slack token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`)},
	{"Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{"Stripe key", regexp.MustCompile(`\b[sr]k_live_[0-9A-Za-z]{16,}`)},
}

// assignmentRE finds `password = "value"` in its many spellings; the value is
// the last group.
var assignmentRE = regexp.MustCompile(
	`(?i)\b(password|passwd|secret|api[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret)\b["']?\s*[:=]\s*["']([^"'\s]{8,})["']`)

// placeholderHints mark a quoted value as documentation rather than a secret.
var placeholderHints = []string{
	"example", "changeme", "placeholder", "dummy", "your", "xxxx", "****", "<", "${", "{{", "todo", "sample",
}

// maxScannedLine bounds the work done on one line; a minified bundle is not
// read to the end.
const maxScannedLine = 4096

// scanLine returns what kind of secret a line of text appears to contain, or
// "". The secret itself is never returned.
func scanLine(line string) string {
	if len(line) > maxScannedLine {
		line = line[:maxScannedLine]
	}
	for _, p := range secretPatterns {
		if p.re.MatchString(line) {
			return p.label
		}
	}
	if m := assignmentRE.FindStringSubmatch(line); m != nil && !looksLikePlaceholder(m[2]) {
		return "password or secret"
	}
	return ""
}

func looksLikePlaceholder(value string) bool {
	lower := strings.ToLower(value)
	for _, hint := range placeholderHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	// One character repeated is filler, not a secret.
	return strings.Trim(value, value[:1]) == ""
}

// Names that mark a file as holding credentials.
var (
	secretFileNames = map[string]bool{
		".env": true, ".netrc": true, "id_rsa": true, "id_dsa": true, "id_ecdsa": true, "id_ed25519": true,
		"credentials.json": true, "secrets.json": true, "secrets.yaml": true, "secrets.yml": true,
	}
	secretFileExts = map[string]bool{
		".pem": true, ".key": true, ".p12": true, ".pfx": true, ".p8": true, ".jks": true, ".keystore": true, ".ppk": true,
	}
	// envTemplateSuffixes are .env.* files meant to be committed.
	envTemplateSuffixes = []string{".example", ".sample", ".template", ".dist"}

	// junkDirs are dependency and build output directories; junkFiles and
	// junkExts are files an operating system or a compiler leaves behind.
	junkDirs = map[string]bool{
		"node_modules": true, ".dart_tool": true, "__pycache__": true, ".gradle": true, "Pods": true,
		"DerivedData": true, "obj": true, "dist": true, "build": true, "target": true,
	}
	junkFiles = map[string]bool{".DS_Store": true, "Thumbs.db": true}
	junkExts  = map[string]bool{".pyc": true, ".class": true, ".o": true}
)

// pathFinding is what a file's name alone says about it.
type pathFinding struct {
	code Code
	// group is what junk findings are counted by: the directory that makes
	// the file junk, so that a whole node_modules is reported once.
	group string
}

// scanPath judges a repository-relative path by its name.
func scanPath(p string) pathFinding {
	base := path.Base(p)
	segments := strings.Split(p, "/")
	for i, segment := range segments[:len(segments)-1] {
		if junkDirs[segment] {
			return pathFinding{code: CodeJunkFile, group: strings.Join(segments[:i+1], "/")}
		}
	}
	if junkFiles[base] || junkExts[path.Ext(base)] {
		return pathFinding{code: CodeJunkFile, group: p}
	}

	if strings.HasPrefix(base, ".env.") {
		for _, suffix := range envTemplateSuffixes {
			if strings.HasSuffix(base, suffix) {
				return pathFinding{}
			}
		}
		return pathFinding{code: CodeSecretFile}
	}
	if secretFileNames[base] || secretFileExts[path.Ext(base)] {
		return pathFinding{code: CodeSecretFile}
	}
	return pathFinding{}
}
