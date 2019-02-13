package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollectorAzureRmQuota struct {
	CollectorProcessorGeneral

	prometheus struct {
		quota *prometheus.GaugeVec
		quotaCurrent *prometheus.GaugeVec
		quotaLimit *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmQuota) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.quota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_info",
			Help: "Azure ResourceManager quota info",
		},
		[]string{
			"subscriptionID",
			"location",
			"scope",
			"quota",
			"quotaName",
		},
	)

	m.prometheus.quotaCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_current",
			Help: "Azure ResourceManager quota current value",
		},
		[]string{
			"subscriptionID",
			"location",
			"scope",
			"quota",
		},
	)

	m.prometheus.quotaLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_limit",
			Help: "Azure ResourceManager quota limit",
		},
		[]string{
			"subscriptionID",
			"location",
			"scope",
			"quota",
		},
	)

	prometheus.MustRegister(m.prometheus.quota)
	prometheus.MustRegister(m.prometheus.quotaCurrent)
	prometheus.MustRegister(m.prometheus.quotaLimit)
}

func (m *MetricsCollectorAzureRmQuota) Reset() {
	m.prometheus.quota.Reset()
	m.prometheus.quotaCurrent.Reset()
	m.prometheus.quotaLimit.Reset()
}

func (m *MetricsCollectorAzureRmQuota) Collect(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureComputeUsage(ctx, callback, subscription)
	m.collectAzureNetworkUsage(ctx, callback, subscription)
	m.collectAzureStorageUsage(ctx, callback, subscription)
}

// Collect Azure ComputeUsage metrics
func (m *MetricsCollectorAzureRmQuota) collectAzureComputeUsage(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	client := compute.NewUsageClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	quotaMetric := MetricCollectorList{}
	quotaCurrentMetric := MetricCollectorList{}
	quotaLimitMetric := MetricCollectorList{}

	for _, location := range m.CollectorReference.AzureLocations {
		list, err := client.List(ctx, location)

		if err != nil {
			panic(err)
		}

		for _, val := range list.Values() {
			quotaName := *val.Name.Value
			quotaNameLocalized := *val.Name.LocalizedValue
			currentValue := float64(*val.CurrentValue)
			limitValue := float64(*val.Limit)

			labels := prometheus.Labels{
				"subscriptionID": *subscription.SubscriptionID,
				"location": location,
				"scope": "compute",
				"quota": quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": *subscription.SubscriptionID,
				"location": location,
				"scope": "compute",
				"quota": quotaName,
				"quotaName": quotaNameLocalized,
			}

			quotaMetric.Add(infoLabels, 1)
			quotaCurrentMetric.Add(labels, currentValue)
			quotaLimitMetric.Add(labels, limitValue)
		}
	}

	callback <- func() {
		quotaMetric.GaugeSet(m.prometheus.quota)
		quotaCurrentMetric.GaugeSet(m.prometheus.quotaCurrent)
		quotaLimitMetric.GaugeSet(m.prometheus.quotaLimit)
	}
}

// Collect Azure NetworkUsage metrics
func (m *MetricsCollectorAzureRmQuota) collectAzureNetworkUsage(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	client := network.NewUsagesClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	quotaMetric := MetricCollectorList{}
	quotaCurrentMetric := MetricCollectorList{}
	quotaLimitMetric := MetricCollectorList{}

	for _, location := range opts.AzureLocation {
		list, err := client.List(ctx, location)

		if err != nil {
			panic(err)
		}

		for _, val := range list.Values() {
			quotaName := *val.Name.Value
			quotaNameLocalized := *val.Name.LocalizedValue
			currentValue := float64(*val.CurrentValue)
			limitValue := float64(*val.Limit)

			labels := prometheus.Labels{
				"subscriptionID": *subscription.SubscriptionID,
				"location": location,
				"scope": "network",
				"quota": quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": *subscription.SubscriptionID,
				"location": location,
				"scope": "network",
				"quota": quotaName,
				"quotaName": quotaNameLocalized,
			}

			quotaMetric.Add(infoLabels, 1)
			quotaCurrentMetric.Add(labels, currentValue)
			quotaLimitMetric.Add(labels, limitValue)
		}
	}

	callback <- func() {
		quotaMetric.GaugeSet(m.prometheus.quota)
		quotaCurrentMetric.GaugeSet(m.prometheus.quotaCurrent)
		quotaLimitMetric.GaugeSet(m.prometheus.quotaLimit)
	}
}

// Collect Azure StorageUsage metrics
func (m *MetricsCollectorAzureRmQuota) collectAzureStorageUsage(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	client := storage.NewUsagesClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	quotaMetric := MetricCollectorList{}
	quotaCurrentMetric := MetricCollectorList{}
	quotaLimitMetric := MetricCollectorList{}

	for _, location := range opts.AzureLocation {
		list, err := client.ListByLocation(ctx, location)

		if err != nil {
			panic(err)
		}

		for _, val := range *list.Value {
			quotaName := *val.Name.Value
			quotaNameLocalized := *val.Name.LocalizedValue
			currentValue := float64(*val.CurrentValue)
			limitValue := float64(*val.Limit)

			labels := prometheus.Labels{
				"subscriptionID": *subscription.SubscriptionID,
				"location": location,
				"scope": "storage",
				"quota": quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": *subscription.SubscriptionID,
				"location": location,
				"scope": "storage",
				"quota": quotaName,
				"quotaName": quotaNameLocalized,
			}

			quotaMetric.Add(infoLabels, 1)
			quotaCurrentMetric.Add(labels, currentValue)
			quotaLimitMetric.Add(labels, limitValue)
		}
	}

	callback <- func() {
		quotaMetric.GaugeSet(m.prometheus.quota)
		quotaCurrentMetric.GaugeSet(m.prometheus.quotaCurrent)
		quotaLimitMetric.GaugeSet(m.prometheus.quotaLimit)
	}
}
