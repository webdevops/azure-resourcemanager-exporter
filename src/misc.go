package main

import (
	"regexp"
	"strconv"
)

var (
	resourceGroupFromResourceIdRegExp = regexp.MustCompile("/resourceGroups/([^/]*)")
)

func prefixSlice(prefix string, valueMap []string) (ret []string) {
	for _, value := range valueMap {
		ret = append(ret, prefix + value)
	}
	return
}

func extractResourceGroupFromAzureId (azureId string) (resourceGroup string) {
	rgSubMatch := resourceGroupFromResourceIdRegExp.FindStringSubmatch(azureId)

	if len(rgSubMatch) >= 1 {
		resourceGroup = rgSubMatch[1]
	}

	return
}

func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}
