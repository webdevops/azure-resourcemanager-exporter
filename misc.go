package main

import (
	"github.com/Azure/go-autorest/autorest/to"
	"regexp"
	"strings"
)

var (
	subscriptionFromResourceIdRegExp  = regexp.MustCompile("(?i)/subscriptions/([^/]+)")
	resourceGroupFromResourceIdRegExp = regexp.MustCompile("(?i)/subscriptions/[^/]+/resourceGroups/([^/]*)")
	providerFromResourceIdRegExp      = regexp.MustCompile("(?i)/subscriptions/[^/]+/resourceGroups/[^/]+/providers/([^/]*)")
	roleDefinitionIdRegExp            = regexp.MustCompile("(?i)/subscriptions/[^/]+/providers/Microsoft.Authorization/roleDefinitions/([^/]*)")
)

func stringPtrToAzureResourceInfo(val *string) (ret string) {
	return stringToAzureResourceInfo(to.String(val))
}

func stringToAzureResourceInfo(val string) (ret string) {
	ret = val
	if *opts.Metrics.ResourceIdLowercase {
		ret = strings.ToLower(ret)
	}
	return
}

func extractSubscriptionIdFromAzureId(azureId string) (subscriptionId string) {
	if subMatch := subscriptionFromResourceIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		subscriptionId = stringToAzureResourceInfo(subMatch[1])
	}
	return
}

func extractResourceGroupFromAzureId(azureId string) (resourceGroup string) {
	if subMatch := resourceGroupFromResourceIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		resourceGroup = stringToAzureResourceInfo(subMatch[1])
	}

	return
}

func extractProviderFromAzureId(azureId string) (provider string) {
	if subMatch := providerFromResourceIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		provider = stringToAzureResourceInfo(subMatch[1])
	}

	return
}

func extractRoleDefinitionIdFromAzureId(azureId string) (roleDefinitionId string) {
	if subMatch := roleDefinitionIdRegExp.FindStringSubmatch(azureId); len(subMatch) >= 1 {
		roleDefinitionId = stringToAzureResourceInfo(subMatch[1])
	}

	return
}

func stringsTrimSuffixCI(str, suffix string) string {
	if strings.HasSuffix(strings.ToLower(str), strings.ToLower(suffix)) {
		str = str[0 : len(str)-len(suffix)]
	}

	return str
}
