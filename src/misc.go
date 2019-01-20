package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	resourceGroupFromResourceIdRegExp = regexp.MustCompile("/resourceGroups/([^/]*)")
	providerFromResourceIdRegExp = regexp.MustCompile("/providers/([^/]*)")
)

func prefixSlice(prefix string, valueMap []string) (ret []string) {
	for _, value := range valueMap {
		ret = append(ret, prefix + value)
	}
	return
}

func extractResourceGroupFromAzureId (azureId string) (resourceGroup string) {
	if subMatch := resourceGroupFromResourceIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		resourceGroup = subMatch[1]
	}

	return
}

func extractProviderFromAzureId (azureId string) (provider string) {
	if subMatch := providerFromResourceIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		provider = subMatch[1]
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

func int32ToString(v int32) string {
	return strconv.FormatInt(int64(v), 10)
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func stringsTrimSuffixCI(str, suffix string) (string) {

	if strings.HasSuffix(strings.ToLower(str), strings.ToLower(suffix)) {
		str = str[0:len(str)-len(suffix)]
	}

	return str
}

func timeToFloat64(v time.Time) float64 {
	return float64(v.Unix())
}

func addAzureResourceTags(labels prometheus.Labels, tags map[string]*string) (prometheus.Labels) {
	for _, rgTag := range opts.AzureResourceTags {
		rgTabLabel := AZURE_RESOURCE_TAG_PREFIX + rgTag

		if _, ok := tags[rgTag]; ok {
			labels[rgTabLabel] = *tags[rgTag]
		} else {
			labels[rgTabLabel] = ""
		}
	}

	return labels
}
