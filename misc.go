package main

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	roleDefinitionIdRegExp        = regexp.MustCompile("(?i)/subscriptions/[^/]+/providers/Microsoft.Authorization/roleDefinitions/([^/]*)")
	prometheusLabelReplacerRegExp = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

func stringToStringLower(val string) string {
	return strings.ToLower(val)
}

func extractRoleDefinitionIdFromAzureId(azureId string) (roleDefinitionId string) {
	if subMatch := roleDefinitionIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		roleDefinitionId = strings.ToLower(subMatch[1])
	}

	return
}

func stringsTrimSuffixCI(str, suffix string) string {
	if strings.HasSuffix(strings.ToLower(str), strings.ToLower(suffix)) {
		str = str[0 : len(str)-len(suffix)]
	}

	return str
}

func truncateStrings(s string, n int, suffix string) string {
	if len(s) <= n {
		return s
	}
	for !utf8.ValidString(s[:n]) {
		n--
	}
	return s[:n] + suffix
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}
