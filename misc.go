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

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func int32ToString(v int32) string {
	return strconv.FormatInt(int64(v), 10)
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
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
