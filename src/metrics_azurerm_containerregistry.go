package main

import (
	"fmt"
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/containerregistry/mgmt/containerregistry"
	"github.com/prometheus/client_golang/prometheus"
)

func (m *MetricCollectorAzureRm) initContainerRegistries() {
	m.prometheus.containerRegistry = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_info",
			Help: "Azure ContainerRegistry limit",
		},
		append(
			[]string{"resourceID", "subscriptionID", "location", "registryName", "resourceGroup", "adminUserEnabled", "skuName", "skuTier"},
			prefixSlice(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)...
		),
	)

	m.prometheus.containerRegistryQuotaCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_quota_current",
			Help: "Azure ContainerRegistry quota current",
		},
		[]string{"subscriptionID", "registryName", "quotaName", "quotaUnit"},
	)

	m.prometheus.containerRegistryQuotaLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_quota_limit",
			Help: "Azure ContainerRegistry quota limit",
		},
		[]string{"subscriptionID", "registryName", "quotaName", "quotaUnit"},
	)

	prometheus.MustRegister(m.prometheus.containerRegistry)
	prometheus.MustRegister(m.prometheus.containerRegistryQuotaCurrent)
	prometheus.MustRegister(m.prometheus.containerRegistryQuotaLimit)
}

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
