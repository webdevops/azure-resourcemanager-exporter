package main

import (
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	"regexp"
	"strconv"
	"strings"
)

var (
	resourceGroupFromResourceIdRegExp = regexp.MustCompile("/subscriptions/[^/]+/resourceGroups/([^/]*)")
	providerFromResourceIdRegExp      = regexp.MustCompile("/subscriptions/[^/]+/resourceGroups/[^/]+/providers/([^/]*)")
	roleDefinitionIdRegExp            = regexp.MustCompile("/Microsoft.Authorization/roleDefinitions/([^/]*)")
)

type (
	MetricRow struct {
		Labels prometheus.Labels
		Value  float64
	}
)

func (m *MetricRow) Inc() {
	m.Value++
}

func toResourceId(val *string) (resourceId string) {
	resourceId = to.String(val)
	if opts.Metrics.ResourceIdLowercase {
		resourceId = strings.ToLower(resourceId)
	}
	return
}

func extractResourceGroupFromAzureId(azureId string) (resourceGroup string) {
	if subMatch := resourceGroupFromResourceIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		resourceGroup = strings.ToLower(subMatch[1])
	}

	if opts.Metrics.ResourceIdLowercase {
		resourceGroup = strings.ToLower(resourceGroup)
	}

	return
}

func extractProviderFromAzureId(azureId string) (provider string) {
	if subMatch := providerFromResourceIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		provider = subMatch[1]
	}

	if opts.Metrics.ResourceIdLowercase {
		provider = strings.ToLower(provider)
	}

	return
}

func extractRoleDefinitionIdFromAzureId(azureId string) (roleDefinitionId string) {
	if subMatch := roleDefinitionIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		roleDefinitionId = subMatch[1]
	}

	if opts.Metrics.ResourceIdLowercase {
		roleDefinitionId = strings.ToLower(roleDefinitionId)
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
