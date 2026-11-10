package i18n

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in     string
		want   Lang
		wantOK bool
	}{
		{in: "tr", want: TR, wantOK: true},
		{in: "en", want: EN, wantOK: true},
		{in: "EN", want: EN, wantOK: true},
		{in: " tr ", want: TR, wantOK: true},
		{in: "de", want: "", wantOK: false},
		{in: "", want: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := Parse(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLookup(t *testing.T) {
	cat := map[Key]message{
		"greet":   {tr: "Merhaba %s", en: "Hello %s"},
		"only_en": {en: "English only"},
		"percent": {tr: "%100 güvenli", en: "100% safe"},
		"swapped": {tr: "%[2]s içinde %[1]d dosya", en: "%[1]d files in %[2]s"},
	}

	tests := []struct {
		name string
		lang Lang
		key  Key
		args []any
		want string
	}{
		{name: "turkish", lang: TR, key: "greet", args: []any{"Emin"}, want: "Merhaba Emin"},
		{name: "english", lang: EN, key: "greet", args: []any{"Emin"}, want: "Hello Emin"},
		{name: "missing translation falls back to english", lang: TR, key: "only_en", want: "English only"},
		{name: "unsupported language falls back to english", lang: Lang("de"), key: "greet", args: []any{"Emin"}, want: "Hello Emin"},
		{name: "unknown key returns the key", lang: TR, key: "no.such.key", want: "no.such.key"},
		{name: "literal percent survives when there are no args", lang: EN, key: "percent", want: "100% safe"},
		{name: "indexed args can be reordered per language", lang: TR, key: "swapped", args: []any{3, "src"}, want: "src içinde 3 dosya"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, lookup(cat, tt.lang, tt.key, tt.args...))
		})
	}
}

func TestLocalizerUsesItsLanguage(t *testing.T) {
	require.NotEmpty(t, catalog, "catalog must not be empty")
	var key Key
	for k, m := range catalog {
		if m.tr != m.en {
			key = k
			break
		}
	}
	require.NotEmpty(t, key, "need at least one message that differs between languages")

	assert.Equal(t, catalog[key].tr, For(TR).T(key))
	assert.Equal(t, catalog[key].en, For(EN).T(key))
	assert.Equal(t, TR, For(TR).Lang())
}

// A message missing in one language silently degrades the UI; a verb mismatch
// prints "%!s(MISSING)" to the user. Both are caught here for the whole catalog.
func TestCatalogIsCompleteAndConsistent(t *testing.T) {
	for key, msg := range catalog {
		t.Run(string(key), func(t *testing.T) {
			assert.NotEmpty(t, msg.tr, "missing Turkish text")
			assert.NotEmpty(t, msg.en, "missing English text")
			assert.Equal(t, verbSignature(msg.en), verbSignature(msg.tr), "format verbs differ between languages")
		})
	}
}

type localizedError struct{}

func (localizedError) Error() string { return "english text" }

func (localizedError) Localize(lang Lang) string {
	if lang == TR {
		return "türkçe metin"
	}
	return "localized english"
}

func TestDescribe(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "localizable error", err: localizedError{}, want: "türkçe metin"},
		{name: "wrapped localizable error", err: fmt.Errorf("load: %w", localizedError{}), want: "türkçe metin"},
		{name: "plain error keeps its own text", err: errors.New("plain"), want: "plain"},
		{name: "nil", err: nil, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Describe(TR, tt.err))
		})
	}
}

func TestMergeCatalogsRejectsDuplicateKeys(t *testing.T) {
	a := map[Key]message{"dup": {tr: "a", en: "a"}}
	b := map[Key]message{"dup": {tr: "b", en: "b"}}

	assert.PanicsWithValue(t, `i18n: duplicate message key "dup"`, func() { mergeCatalogs(a, b) })
}

var verbRE = regexp.MustCompile(`%(?:\[(\d+)\])?[-+# 0]*\d*(?:\.\d+)?([a-zA-Z%])`)

// verbSignature maps each argument position to the verb that consumes it.
func verbSignature(s string) map[int]string {
	sig := map[int]string{}
	next := 1
	for _, m := range verbRE.FindAllStringSubmatch(s, -1) {
		if m[2] == "%" {
			continue
		}
		idx := next
		if m[1] != "" {
			idx, _ = strconv.Atoi(m[1])
		}
		sig[idx] = m[2]
		next = idx + 1
	}
	return sig
}
