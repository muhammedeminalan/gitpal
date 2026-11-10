package git

import (
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

var (
	// scheme://userinfo@ — userinfo is everything up to the last "@" before
	// the path, so a password that itself contains "@" is covered.
	urlCredentialsRE = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)([^/?#\s'"]+)@`)
	extraHeaderRE    = regexp.MustCompile(`(?i)(extraheader=).*`)
	authHeaderRE     = regexp.MustCompile(`(?i)(authorization:[ \t]*)[^\r\n]*`)
)

// redact removes credentials from text that may be shown, logged or stored:
// user info embedded in URLs and HTTP authorization headers.
//
// A bare user name in a non-HTTP URL (ssh://git@host) is not a secret and is
// kept. In an HTTP URL even a lone user name is hidden, because access tokens
// are commonly written there (https://<token>@github.com/...).
func redact(s string) string {
	s = urlCredentialsRE.ReplaceAllStringFunc(s, func(match string) string {
		parts := urlCredentialsRE.FindStringSubmatch(match)
		scheme, userinfo := parts[1], parts[2]
		isHTTP := strings.HasPrefix(strings.ToLower(scheme), "http")
		if !isHTTP && !strings.Contains(userinfo, ":") {
			return match
		}
		return scheme + redacted + "@"
	})
	s = extraHeaderRE.ReplaceAllString(s, "${1}"+redacted)
	s = authHeaderRE.ReplaceAllString(s, "${1}"+redacted)
	return s
}

func redactAll(args []string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = redact(arg)
	}
	return out
}
