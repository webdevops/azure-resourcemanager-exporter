package config

import (
	"regexp"
	"unicode"
	"unicode/utf8"
)

var (
	prometheusLabelReplacerRegExp = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}
