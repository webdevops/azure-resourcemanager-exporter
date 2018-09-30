package main

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
