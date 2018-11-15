package main

import (
	"fmt"
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/containerregistry/mgmt/containerregistry"
	"github.com/prometheus/client_golang/prometheus"
)

func (m *MetricCollectorAzureRm) collectAzureContainerRegistries(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := containerregistry.NewRegistriesClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListComplete(ctx)

	if err != nil {
		panic(err)
	}

	infoMetric := prometheusMetricsList{}
	quotaCurrentMetric := prometheusMetricsList{}
	quotaLimitMetric := prometheusMetricsList{}

	for list.NotDone() {
		val := list.Value()

		arcUsage, err := client.ListUsages(ctx, extractResourceGroupFromAzureId(*val.ID), *val.Name)

		if err != nil {
			ErrorLogger.Error(fmt.Sprintf("subscription[%v]: unable to fetch ACR usage for %v", subscriptionId, *val.Name), err)
		}

		skuName := ""
		skuTier := ""

		if val.Sku != nil {
			skuName = string(val.Sku.Name)
			skuTier = string(val.Sku.Tier)
		}

		infoLabels := prometheus.Labels{
			"resourceID": *val.ID,
			"subscriptionID": subscriptionId,
			"location": *val.Location,
			"registryName": *val.Name,
			"resourceGroup": extractResourceGroupFromAzureId(*val.ID),
			"adminUserEnabled": boolToString(*val.AdminUserEnabled),
			"skuName": skuName,
			"skuTier": skuTier,
		}
		infoLabels = m.addAzureResourceTags(infoLabels, val.Tags)

		infoMetric.Add(infoLabels, 1)


		if arcUsage.Value != nil {
			for _, usage := range *arcUsage.Value {
				quotaLabels := prometheus.Labels{
					"subscriptionID": subscriptionId,
					"registryName": *val.Name,
					"quotaUnit": string(usage.Unit),
					"quotaName": *usage.Name,
				}

				quotaCurrentMetric.Add(quotaLabels, float64(*usage.CurrentValue))
				quotaLimitMetric.Add(quotaLabels, float64(*usage.Limit))
			}
		}

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.containerRegistry)
		quotaCurrentMetric.GaugeSet(m.prometheus.containerRegistryQuotaCurrent)
		quotaLimitMetric.GaugeSet(m.prometheus.containerRegistryQuotaLimit)
	}
}
