package safety

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanSecretsInText(t *testing.T) {
	// Assembled at run time so this file does not itself look like a leak.
	join := func(parts ...string) string { return strings.Join(parts, "") }

	for name, tt := range map[string]struct {
		line string
		want string
	}{
		"private key":       {"-----BEGIN RSA PRIVATE KEY-----", "private key"},
		"openssh key":       {"-----BEGIN OPENSSH PRIVATE KEY-----", "private key"},
		"aws access key":    {join("aws_access_key_id = AKIA", "IOSFODNN7EXAMPLE"), "AWS access key"},
		"github token":      {join("token: ghp", "_", strings.Repeat("a1B2", 9)), "GitHub token"},
		"github fine-grain": {join("github_pat", "_", strings.Repeat("A1b2C3d4", 4)), "GitHub token"},
		"anthropic key":     {join("ANTHROPIC_API_KEY=sk-ant", "-api03-", strings.Repeat("xY9", 12)), "Anthropic API key"},
		"openai key":        {join("sk-proj", "-", strings.Repeat("aB3", 14)), "OpenAI API key"},
		"slack token":       {join("xoxb", "-123456789012-", strings.Repeat("a", 24)), "Slack token"},
		"google key":        {join("AIza", strings.Repeat("Sy0", 11), "ab"), "Google API key"},
		"stripe key":        {join("sk_live", "_", strings.Repeat("4eC3", 6)), "Stripe key"},
		"password literal":  {`password = "kT9#mQ2vLp8x"`, "password or secret"},
		"api key in json":   {`"api_key": "f3a91c0d7b2e4a6f"`, "password or secret"},
		"client secret":     {`client_secret: 'Zx81-qPl0_mmN2a'`, "password or secret"},
	} {
		t.Run(name, func(t *testing.T) {
			got := scanLine(tt.line)
			assert.Equal(t, tt.want, got, "line %q", tt.line)
		})
	}
}

func TestScanIgnoresWhatIsNotASecret(t *testing.T) {
	for _, line := range []string{
		"",
		"func main() {}",
		`password = ""`,
		`password = "changeme"`,
		`api_key: "<your-api-key>"`,
		`secret = "${SECRET_FROM_ENV}"`,
		`token = "xxxxxxxxxxxxxxxx"`,
		`password = os.Getenv("DB_PASSWORD")`,
		`apiKey: "example-key-123456"`,
		"the BEGIN PUBLIC KEY block",
		"-----BEGIN PUBLIC KEY-----",
		"-----BEGIN CERTIFICATE-----",
		"sk-short",
		"commit 3f9a1c0d7b2e4a6f3f9a1c0d7b2e4a6f3f9a1c0d",
		"AKIA is how AWS key ids start",
	} {
		assert.Empty(t, scanLine(line), "line %q", line)
	}
}

func TestScanFileNames(t *testing.T) {
	for path, want := range map[string]Code{
		".env":                       CodeSecretFile,
		"config/.env.production":     CodeSecretFile,
		"deploy/id_rsa":              CodeSecretFile,
		"certs/server.pem":           CodeSecretFile,
		"android/app/upload.jks":     CodeSecretFile,
		"ios/AuthKey_ABC123.p8":      CodeSecretFile,
		"secrets.yaml":               CodeSecretFile,
		"node_modules/x/index.js":    CodeJunkFile,
		"app/build/outputs/a.apk":    CodeJunkFile,
		".dart_tool/package_config":  CodeJunkFile,
		"src/Api/obj/project.assets": CodeJunkFile,
		"ios/Pods/Firebase/x.h":      CodeJunkFile,
		".DS_Store":                  CodeJunkFile,
		"pkg/__pycache__/m.pyc":      CodeJunkFile,
		".env.example":               "",
		".env.sample":                "",
		"docs/environment.md":        "",
		"src/main.go":                "",
		"keys/README.md":             "",
		"lib/builder.dart":           "",
		"public.pem.md":              "",
	} {
		assert.Equal(t, want, scanPath(path).code, "path %q", path)
	}
}

// Thousands of files under one ignored-looking directory are one finding.
func TestScanJunkIsGroupedByDirectory(t *testing.T) {
	a := scanPath("web/node_modules/a/index.js")
	b := scanPath("web/node_modules/b/deep/er.js")
	other := scanPath("node_modules/a.js")

	assert.Equal(t, "web/node_modules", a.group)
	assert.Equal(t, a.group, b.group)
	assert.Equal(t, "node_modules", other.group)
	assert.Equal(t, ".DS_Store", scanPath(".DS_Store").group)
}
