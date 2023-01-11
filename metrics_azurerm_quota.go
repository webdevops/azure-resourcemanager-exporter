package main

import (
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	armmachinelearning "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/machinelearning/armmachinelearning/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
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

	m.Collector.RegisterMetricList("quota", m.prometheus.quota, true)
	m.Collector.RegisterMetricList("quotaCurrent", m.prometheus.quotaCurrent, true)
	m.Collector.RegisterMetricList("quotaLimit", m.prometheus.quotaLimit, true)
	m.Collector.RegisterMetricList("quotaUsage", m.prometheus.quotaUsage, true)
}

func (m *MetricsCollectorAzureRmQuota) Reset() {}

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
			m.collectAzureStorageUsage(subscription, logger, callback)
		} else if err != nil {
			logger.Error(err.Error())
		}

		if registered, err := AzureClient.IsResourceProviderRegistered(m.Context(), *subscription.SubscriptionID, "Microsoft.Storage"); registered {
			m.collectAzureStorageUsage(subscription, logger, callback)
		} else if err != nil {
			logger.Error(err.Error())
		}

		if registered, err := AzureClient.IsResourceProviderRegistered(m.Context(), *subscription.SubscriptionID, "Microsoft.MachineLearningServices"); registered {
			m.collectAzureMachineLearningUsage(subscription, logger, callback)
		} else if err != nil {
			logger.Error(err.Error())
		}
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

// collectAzureComputeUsage collects compute usages
func (m *MetricsCollectorAzureRmQuota) collectAzureComputeUsage(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armcompute.NewUsageClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

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
				currentValue := float64(to.Number(resourceUsage.CurrentValue))
				limitValue := float64(to.Number(resourceUsage.Limit))

				infoLabels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "compute",
					"quota":          quotaName,
					"quotaName":      quotaNameLocalized,
				}

				labels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
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
}

// collectAzureComputeUsage collects network usages
func (m *MetricsCollectorAzureRmQuota) collectAzureNetworkUsage(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armnetwork.NewUsagesClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

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
				currentValue := float64(to.Number(resourceUsage.CurrentValue))
				limitValue := float64(to.Number(resourceUsage.Limit))

				infoLabels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "network",
					"quota":          quotaName,
					"quotaName":      quotaNameLocalized,
				}

				labels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
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
}

// collectAzureComputeUsage collects storage usages
func (m *MetricsCollectorAzureRmQuota) collectAzureStorageUsage(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armstorage.NewUsagesClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

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
				currentValue := float64(to.Number(resourceUsage.CurrentValue))
				limitValue := float64(to.Number(resourceUsage.Limit))

				infoLabels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "storage",
					"quota":          quotaName,
					"quotaName":      quotaNameLocalized,
				}

				labels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
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
}

// collectAzureComputeUsage collects machinelearning usages
func (m *MetricsCollectorAzureRmQuota) collectAzureMachineLearningUsage(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armmachinelearning.NewUsagesClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

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
				currentValue := float64(to.Number(resourceUsage.CurrentValue))
				limitValue := float64(to.Number(resourceUsage.Limit))

				infoLabels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "machinelearningservices",
					"quota":          quotaName,
					"quotaName":      quotaNameLocalized,
				}

				labels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"scope":          "machinelearningservices",
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
}
