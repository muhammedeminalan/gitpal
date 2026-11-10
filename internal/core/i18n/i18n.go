// Package i18n holds gitpal's user-facing messages in Turkish and English.
//
// Core packages return structured results and typed errors; the text shown to
// the user is always looked up here, so every interface (CLI, TUI, GUI) speaks
// the same words in the configured language.
package i18n

import (
	"errors"
	"fmt"
	"strings"
)

// Lang is a supported interface language.
type Lang string

// Supported languages.
const (
	TR Lang = "tr"
	EN Lang = "en"
)

// Default is the language used when none is configured (spec §10: `language: tr`).
const Default = TR

// Key identifies a message in the catalog.
type Key string

// message carries one text per language, side by side, so a translation cannot
// drift away from its counterpart unnoticed.
type message struct{ tr, en string }

func (m message) text(lang Lang) string {
	if lang == TR && m.tr != "" {
		return m.tr
	}
	return m.en
}

// Localizable is implemented by errors and results that can describe
// themselves to the user in a given language.
type Localizable interface {
	Localize(lang Lang) string
}

// Describe renders err for the user. An error that implements Localizable
// (itself or anywhere in its chain) is rendered in lang; any other error keeps
// its own text.
func Describe(lang Lang, err error) string {
	if err == nil {
		return ""
	}
	var loc Localizable
	if errors.As(err, &loc) {
		return loc.Localize(lang)
	}
	return err.Error()
}

// Parse converts a configuration value into a Lang.
func Parse(s string) (Lang, bool) {
	switch lang := Lang(strings.ToLower(strings.TrimSpace(s))); lang {
	case TR, EN:
		return lang, true
	default:
		return "", false
	}
}

// T returns the message for key in lang, formatted with args.
//
// Rendering never fails: a missing translation falls back to English and an
// unknown key to the key itself.
func T(lang Lang, key Key, args ...any) string {
	return lookup(catalog, lang, key, args...)
}

func lookup(cat map[Key]message, lang Lang, key Key, args ...any) string {
	text := cat[key].text(lang)
	if text == "" {
		return string(key)
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

// Localizer renders messages in one fixed language.
type Localizer struct{ lang Lang }

// For returns a Localizer for lang.
func For(lang Lang) Localizer { return Localizer{lang: lang} }

// Lang returns the language this Localizer renders in.
func (l Localizer) Lang() Lang { return l.lang }

// T returns the message for key, formatted with args.
func (l Localizer) T(key Key, args ...any) string { return T(l.lang, key, args...) }

func mergeCatalogs(parts ...map[Key]message) map[Key]message {
	merged := make(map[Key]message)
	for _, part := range parts {
		for key, msg := range part {
			if _, dup := merged[key]; dup {
				panic(fmt.Sprintf("i18n: duplicate message key %q", key))
			}
			merged[key] = msg
		}
	}
	return merged
}
