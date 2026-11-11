package ai

import (
	"math"
	"regexp"
	"strings"

	"github.com/muhammedeminalan/gitpal/internal/core/safety"
)

// tokenRE finds long runs of the characters API keys and tokens are made of.
var tokenRE = regexp.MustCompile(`[A-Za-z0-9+/_=-]{24,}`)

// minSecretEntropy is the Shannon entropy, in bits per character, from which
// a long token is treated as random — a key rather than a word. Hex strings
// (commit hashes, checksums) top out at 4.0 and stay readable.
const minSecretEntropy = 4.2

// Redact removes secrets from text before it leaves the machine: everything
// the safety layer recognizes, plus long random-looking tokens.
func Redact(text string) string {
	text = safety.RedactSecrets(text)
	return tokenRE.ReplaceAllStringFunc(text, func(token string) string {
		if looksRandom(token) {
			return safety.Redacted
		}
		return token
	})
}

// looksRandom reports whether a token mixes letters and digits and has the
// entropy of generated data rather than of an identifier.
func looksRandom(token string) bool {
	if !strings.ContainsAny(token, "0123456789") || !strings.ContainsFunc(token, isLetter) {
		return false
	}
	return entropy(token) >= minSecretEntropy
}

func isLetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }

func entropy(s string) float64 {
	counts := map[rune]int{}
	for _, r := range s {
		counts[r]++
	}
	var h float64
	n := float64(len(s))
	for _, c := range counts {
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}
