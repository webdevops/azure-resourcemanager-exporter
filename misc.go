package main

import (
	"github.com/Azure/go-autorest/autorest/to"
	"regexp"
	"strings"
)

var (
	subscriptionFromResourceIdRegExp  = regexp.MustCompile("/subscriptions/([^/]+)")
	resourceGroupFromResourceIdRegExp = regexp.MustCompile("/subscriptions/[^/]+/resourceGroups/([^/]*)")
	providerFromResourceIdRegExp      = regexp.MustCompile("/subscriptions/[^/]+/resourceGroups/[^/]+/providers/([^/]*)")
	roleDefinitionIdRegExp            = regexp.MustCompile("/Microsoft.Authorization/roleDefinitions/([^/]*)")
)

func toResourceId(val *string) (resourceId string) {
	resourceId = to.String(val)
	if opts.Metrics.ResourceIdLowercase {
		resourceId = strings.ToLower(resourceId)
	}
	return
}

func extractSubscriptionIdFromAzureId(azureId string) (subscriptionId string) {
	if subMatch := subscriptionFromResourceIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		subscriptionId = strings.ToLower(subMatch[1])
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

func stringsTrimSuffixCI(str, suffix string) string {
	if strings.HasSuffix(strings.ToLower(str), strings.ToLower(suffix)) {
		str = str[0 : len(str)-len(suffix)]
	}

	return str
}
