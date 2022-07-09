package main

import (
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
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
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *log.Entry) {

		if registered, err := AzureClient.IsResourceProviderRegistered(m.Context(), *subscription.SubscriptionID, "Microsoft.Compute"); registered {
			m.collectAzureComputeUsage(subscription, logger, callback)
		} else if err != nil {
			logger.Error(err.Error())
		}

		if registered, err := AzureClient.IsResourceProviderRegistered(m.Context(), *subscription.SubscriptionID, "Microsoft.Network"); registered {
			m.collectAzureNetworkUsage(subscription, logger, callback)
		} else if err != nil {
			logger.Error(err.Error())
		}

		if registered, err := AzureClient.IsResourceProviderRegistered(m.Context(), *subscription.SubscriptionID, "Microsoft.Storage"); registered {
			m.collectAzureNetworkUsage(subscription, logger, callback)
		} else if err != nil {
			logger.Error(err.Error())
		}
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

// Collect Azure ComputeUsage metrics
func (m *MetricsCollectorAzureRmQuota) collectAzureComputeUsage(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armcompute.NewUsageClient(*subscription.SubscriptionID, AzureClient.GetCred(), nil)
	if err != nil {
		logger.Panic(err)
	}

	quotaMetric := prometheusCommon.NewMetricsList()
	quotaCurrentMetric := prometheusCommon.NewMetricsList()
	quotaLimitMetric := prometheusCommon.NewMetricsList()
	quotaUsageMetric := prometheusCommon.NewMetricsList()

	for _, location := range opts.Azure.Location {
		pager := client.NewListPager(location, nil)

		for pager.More() {
			result, err := pager.NextPage(m.Context())
			if err != nil {
				logger.Panic(err)
			}

			if result.Value == nil {
				continue
			}

			for _, resourceUsage := range result.Value {
				quotaName := to.String(resourceUsage.Name.Value)
				quotaNameLocalized := to.String(resourceUsage.Name.LocalizedValue)
				currentValue := float64(to.Int32(resourceUsage.CurrentValue))
				limitValue := float64(to.Int64(resourceUsage.Limit))

				infoLabels := prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "compute",
					"quota":          quotaName,
					"quotaName":      quotaNameLocalized,
				}

				labels := prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "compute",
					"quota":          quotaName,
				}

				quotaMetric.Add(infoLabels, 1)
				quotaCurrentMetric.Add(labels, currentValue)
				quotaLimitMetric.Add(labels, limitValue)
				if limitValue != 0 {
					quotaUsageMetric.Add(labels, currentValue/limitValue)
				}
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
func (m *MetricsCollectorAzureRmQuota) collectAzureNetworkUsage(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armnetwork.NewUsagesClient(*subscription.SubscriptionID, AzureClient.GetCred(), nil)
	if err != nil {
		logger.Panic(err)
	}

	quotaMetric := prometheusCommon.NewMetricsList()
	quotaCurrentMetric := prometheusCommon.NewMetricsList()
	quotaLimitMetric := prometheusCommon.NewMetricsList()
	quotaUsageMetric := prometheusCommon.NewMetricsList()

	for _, location := range opts.Azure.Location {
		pager := client.NewListPager(location, nil)

		for pager.More() {
			result, err := pager.NextPage(m.Context())
			if err != nil {
				logger.Panic(err)
			}

			if result.Value == nil {
				continue
			}

			for _, resourceUsage := range result.Value {
				quotaName := to.String(resourceUsage.Name.Value)
				quotaNameLocalized := to.String(resourceUsage.Name.LocalizedValue)
				currentValue := float64(to.Int64(resourceUsage.CurrentValue))
				limitValue := float64(to.Int64(resourceUsage.Limit))

				infoLabels := prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "network",
					"quota":          quotaName,
					"quotaName":      quotaNameLocalized,
				}

				labels := prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "network",
					"quota":          quotaName,
				}

				quotaMetric.Add(infoLabels, 1)
				quotaCurrentMetric.Add(labels, currentValue)
				quotaLimitMetric.Add(labels, limitValue)
				if limitValue != 0 {
					quotaUsageMetric.Add(labels, currentValue/limitValue)
				}
			}
		}
	}

	callback <- func() {
		quotaMetric.GaugeSet(m.prometheus.quota)
		quotaCurrentMetric.GaugeSet(m.prometheus.quotaCurrent)
		quotaLimitMetric.GaugeSet(m.prometheus.quotaLimit)
	}
}

// Collect Azure StorageUsage metrics
func (m *MetricsCollectorAzureRmQuota) collectAzureStorageUsage(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armstorage.NewUsagesClient(*subscription.SubscriptionID, AzureClient.GetCred(), nil)
	if err != nil {
		logger.Panic(err)
	}

	quotaMetric := prometheusCommon.NewMetricsList()
	quotaCurrentMetric := prometheusCommon.NewMetricsList()
	quotaLimitMetric := prometheusCommon.NewMetricsList()
	quotaUsageMetric := prometheusCommon.NewMetricsList()

	for _, location := range opts.Azure.Location {
		pager := client.NewListByLocationPager(location, nil)

		for pager.More() {
			result, err := pager.NextPage(m.Context())
			if err != nil {
				logger.Panic(err)
			}

			if result.Value == nil {
				continue
			}

			for _, resourceUsage := range result.Value {
				quotaName := to.String(resourceUsage.Name.Value)
				quotaNameLocalized := to.String(resourceUsage.Name.LocalizedValue)
				currentValue := float64(to.Int32(resourceUsage.CurrentValue))
				limitValue := float64(to.Int32(resourceUsage.Limit))

				infoLabels := prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "storage",
					"quota":          quotaName,
					"quotaName":      quotaNameLocalized,
				}

				labels := prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "storage",
					"quota":          quotaName,
				}

				quotaMetric.Add(infoLabels, 1)
				quotaCurrentMetric.Add(labels, currentValue)
				quotaLimitMetric.Add(labels, limitValue)
				if limitValue != 0 {
					quotaUsageMetric.Add(labels, currentValue/limitValue)
				}
			}
		}
	}

	callback <- func() {
		quotaMetric.GaugeSet(m.prometheus.quota)
		quotaCurrentMetric.GaugeSet(m.prometheus.quotaCurrent)
		quotaLimitMetric.GaugeSet(m.prometheus.quotaLimit)
	}
}
