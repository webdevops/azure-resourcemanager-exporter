package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	resourceGroupFromResourceIdRegExp = regexp.MustCompile("/resourceGroups/([^/]*)")
	providerFromResourceIdRegExp      = regexp.MustCompile("/providers/([^/]*)")
	roleDefinitionIdRegExp            = regexp.MustCompile("/Microsoft.Authorization/roleDefinitions/([^/]*)")
)

func extractResourceGroupFromAzureId(azureId string) (resourceGroup string) {
	if subMatch := resourceGroupFromResourceIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		resourceGroup = subMatch[1]
	}

	return
}

func extractProviderFromAzureId(azureId string) (provider string) {
	if subMatch := providerFromResourceIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		provider = subMatch[1]
	}

	return
}

func extractRoleDefinitionIdFromAzureId(azureId string) (provider string) {
	if subMatch := roleDefinitionIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		provider = subMatch[1]
	}

	return
}

func stringPtrToString(val *string) (ret string) {
	if val != nil {
		ret = *val
	}
	return
}

func boolPtrToString(b *bool) string {
	if b == nil {
		return ""
	}

	if *b {
		return "true"
	}
	return "false"
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func int32ToString(v int32) string {
	return strconv.FormatInt(int64(v), 10)
}

func stringsTrimSuffixCI(str, suffix string) string {
	if strings.HasSuffix(strings.ToLower(str), strings.ToLower(suffix)) {
		str = str[0 : len(str)-len(suffix)]
	}

	return str
}

func timeToFloat64(v time.Time) float64 {
	return float64(v.Unix())
}
