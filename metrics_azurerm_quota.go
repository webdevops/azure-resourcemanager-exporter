package main

import (
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorAzureRmQuota struct {
	collector.Processor

	prometheus struct {
		quota        *prometheus.GaugeVec
		quotaCurrent *prometheus.GaugeVec
		quotaLimit   *prometheus.GaugeVec
		quotaUsage   *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmQuota) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.quota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_info",
			Help: "Azure ResourceManager quota information",
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

	m.prometheus.quotaUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_usage",
			Help: "Azure ResourceManager quota usage in percent",
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
	prometheus.MustRegister(m.prometheus.quotaUsage)
}

func (m *MetricsCollectorAzureRmQuota) Reset() {
	m.prometheus.quota.Reset()
	m.prometheus.quotaCurrent.Reset()
	m.prometheus.quotaLimit.Reset()
}

func (m *MetricsCollectorAzureRmQuota) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription subscriptions.Subscription, logger *log.Entry) {
		m.collectAzureComputeUsage(subscription, logger, callback)
		m.collectAzureNetworkUsage(subscription, logger, callback)
		m.collectAzureStorageUsage(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

// Collect Azure ComputeUsage metrics
func (m *MetricsCollectorAzureRmQuota) collectAzureComputeUsage(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client := compute.NewUsageClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	AzureClient.DecorateAzureAutorest(&client.Client)

	quotaMetric := prometheusCommon.NewMetricsList()
	quotaCurrentMetric := prometheusCommon.NewMetricsList()
	quotaLimitMetric := prometheusCommon.NewMetricsList()
	quotaUsageMetric := prometheusCommon.NewMetricsList()

	for _, location := range opts.Azure.Location {
		list, err := client.List(m.Context(), location)

		if err != nil {
			logger.Panic(err)
		}

		for _, val := range list.Values() {
			quotaName := to.String(val.Name.Value)
			quotaNameLocalized := to.String(val.Name.LocalizedValue)
			currentValue := float64(to.Int32(val.CurrentValue))
			limitValue := float64(to.Int64(val.Limit))

			labels := prometheus.Labels{
				"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
				"location":       strings.ToLower(location),
				"scope":          "compute",
				"quota":          quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
				"location":       strings.ToLower(location),
				"scope":          "compute",
				"quota":          quotaName,
				"quotaName":      quotaNameLocalized,
			}

			quotaMetric.Add(infoLabels, 1)
			quotaCurrentMetric.Add(labels, currentValue)
			quotaLimitMetric.Add(labels, limitValue)
			if limitValue != 0 {
				quotaUsageMetric.Add(labels, currentValue/limitValue)
			}
		}
	}

	callback <- func() {
		quotaMetric.GaugeSet(m.prometheus.quota)
		quotaCurrentMetric.GaugeSet(m.prometheus.quotaCurrent)
		quotaLimitMetric.GaugeSet(m.prometheus.quotaLimit)
		quotaUsageMetric.GaugeSet(m.prometheus.quotaUsage)
	}
}

// Collect Azure NetworkUsage metrics
func (m *MetricsCollectorAzureRmQuota) collectAzureNetworkUsage(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client := network.NewUsagesClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	AzureClient.DecorateAzureAutorest(&client.Client)

	quotaMetric := prometheusCommon.NewMetricsList()
	quotaCurrentMetric := prometheusCommon.NewMetricsList()
	quotaLimitMetric := prometheusCommon.NewMetricsList()

	for _, location := range opts.Azure.Location {
		list, err := client.List(m.Context(), location)

		if err != nil {
			logger.Panic(err)
		}

		for _, val := range list.Values() {
			quotaName := to.String(val.Name.Value)
			quotaNameLocalized := to.String(val.Name.LocalizedValue)
			currentValue := float64(to.Int64(val.CurrentValue))
			limitValue := float64(to.Int64(val.Limit))

			labels := prometheus.Labels{
				"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
				"location":       strings.ToLower(location),
				"scope":          "network",
				"quota":          quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
				"location":       strings.ToLower(location),
				"scope":          "network",
				"quota":          quotaName,
				"quotaName":      quotaNameLocalized,
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
func (m *MetricsCollectorAzureRmQuota) collectAzureStorageUsage(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client := storage.NewUsagesClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	AzureClient.DecorateAzureAutorest(&client.Client)

	quotaMetric := prometheusCommon.NewMetricsList()
	quotaCurrentMetric := prometheusCommon.NewMetricsList()
	quotaLimitMetric := prometheusCommon.NewMetricsList()

	for _, location := range opts.Azure.Location {
		list, err := client.ListByLocation(m.Context(), location)

		if err != nil {
			logger.Panic(err)
		}

		for _, val := range *list.Value {
			quotaName := to.String(val.Name.Value)
			quotaNameLocalized := to.String(val.Name.LocalizedValue)
			currentValue := float64(to.Int32(val.CurrentValue))
			limitValue := float64(to.Int32(val.Limit))

			quotaMetric.AddInfo(prometheus.Labels{
				"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
				"location":       strings.ToLower(location),
				"scope":          "storage",
				"quota":          quotaName,
				"quotaName":      quotaNameLocalized,
			})

			labels := prometheus.Labels{
				"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
				"location":       strings.ToLower(location),
				"scope":          "storage",
				"quota":          quotaName,
			}

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
