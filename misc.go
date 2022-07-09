package main

import (
	"regexp"
	"strings"
)

var (
	roleDefinitionIdRegExp = regexp.MustCompile("(?i)/subscriptions/[^/]+/providers/Microsoft.Authorization/roleDefinitions/([^/]*)")
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
