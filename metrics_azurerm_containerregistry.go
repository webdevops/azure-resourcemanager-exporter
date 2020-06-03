package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/containerregistry/mgmt/containerregistry"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/prometheus/client_golang/prometheus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
)

type MetricsCollectorAzureRmContainerRegistry struct {
	CollectorProcessorGeneral

	prometheus struct {
		containerRegistry             *prometheus.GaugeVec
		containerRegistryQuotaCurrent *prometheus.GaugeVec
		containerRegistryQuotaLimit   *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmContainerRegistry) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.containerRegistry = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_info",
			Help: "Azure ContainerRegistry limit",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"location",
				"registryName",
				"resourceGroup",
				"adminUserEnabled",
				"skuName",
				"skuTier",
			},
			opts.azureResourceTags.prometheusLabels...,
		),
	)

	m.prometheus.containerRegistryQuotaCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_quota_current",
			Help: "Azure ContainerRegistry quota current",
		},
		[]string{
			"subscriptionID",
			"registryName",
			"quotaName",
			"quotaUnit",
		},
	)

	m.prometheus.containerRegistryQuotaLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_quota_limit",
			Help: "Azure ContainerRegistry quota limit",
		},
		[]string{
			"subscriptionID",
			"registryName",
			"quotaName",
			"quotaUnit",
		},
	)

	prometheus.MustRegister(m.prometheus.containerRegistry)
	prometheus.MustRegister(m.prometheus.containerRegistryQuotaCurrent)
	prometheus.MustRegister(m.prometheus.containerRegistryQuotaLimit)
}

func (m *MetricsCollectorAzureRmContainerRegistry) Reset() {
	m.prometheus.containerRegistry.Reset()
	m.prometheus.containerRegistryQuotaCurrent.Reset()
	m.prometheus.containerRegistryQuotaLimit.Reset()
}

func (m *MetricsCollectorAzureRmContainerRegistry) Collect(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	client := containerregistry.NewRegistriesClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListComplete(ctx)

	if err != nil {
		panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()
	quotaCurrentMetric := prometheusCommon.NewMetricsList()
	quotaLimitMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		arcUsage, err := client.ListUsages(ctx, extractResourceGroupFromAzureId(*val.ID), *val.Name)

		if err != nil {
			Logger.Errorf("subscription[%v]: unable to fetch ACR usage for %v: %v", *subscription.SubscriptionID, *val.Name, err)
		}

		skuName := ""
		skuTier := ""

		if val.Sku != nil {
			skuName = string(val.Sku.Name)
			skuTier = string(val.Sku.Tier)
		}

		infoLabels := prometheus.Labels{
			"resourceID":       *val.ID,
			"subscriptionID":   *subscription.SubscriptionID,
			"location":         stringPtrToString(val.Location),
			"registryName":     stringPtrToString(val.Name),
			"resourceGroup":    extractResourceGroupFromAzureId(*val.ID),
			"adminUserEnabled": boolPtrToString(val.AdminUserEnabled),
			"skuName":          skuName,
			"skuTier":          skuTier,
		}
		infoLabels = opts.azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		infoMetric.AddInfo(infoLabels)

		if arcUsage.Value != nil {
			for _, usage := range *arcUsage.Value {
				quotaLabels := prometheus.Labels{
					"subscriptionID": *subscription.SubscriptionID,
					"registryName":   stringPtrToString(val.Name),
					"quotaUnit":      string(usage.Unit),
					"quotaName":      stringPtrToString(usage.Name),
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
