package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	armruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	armmachinelearning "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/machinelearning/armmachinelearning/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/prometheus/client_golang/prometheus"
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
			"provider",
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
			"provider",
			"scope",
			"quota",
			"quotaName",
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
			"provider",
			"scope",
			"quota",
			"quotaName",
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
			"provider",
			"scope",
			"quota",
			"quotaName",
		},
	)

	m.Collector.RegisterMetricList("quota", m.prometheus.quota, true)
	m.Collector.RegisterMetricList("quotaCurrent", m.prometheus.quotaCurrent, true)
	m.Collector.RegisterMetricList("quotaLimit", m.prometheus.quotaLimit, true)
	m.Collector.RegisterMetricList("quotaUsage", m.prometheus.quotaUsage, true)
}

func (m *MetricsCollectorAzureRmQuota) Reset() {}

func (m *MetricsCollectorAzureRmQuota) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *slog.Logger) {
		m.collectAuthorizationUsage(subscription, logger, callback)

		// if registered, err := AzureClient.IsResourceProviderRegistered(m.Context(), *subscription.SubscriptionID, "Microsoft.Capacity"); registered {
		// 	m.collectQuotaUsage(subscription, logger, callback)
		// } else if err != nil {
		// 	logger.Error(err.Error())
		// }

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
		panic(err)
	}
}

// collectAzureComputeUsage collects compute usages
func (m *MetricsCollectorAzureRmQuota) collectAuthorizationUsage(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	options := AzureClient.NewArmClientOptions()
	ep := cloud.AzurePublic.Services[cloud.ResourceManager].Endpoint
	if c, ok := options.Cloud.Services[cloud.ResourceManager]; ok {
		ep = c.Endpoint
	}

	pl, err := armruntime.NewPipeline("azurerm-quota", gitTag, AzureClient.GetCred(), runtime.PipelineOptions{}, options)
	if err != nil {
		panic(err)
	}

	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

	ctx := context.Background()

	urlPath := "/subscriptions/{subscriptionId}/providers/Microsoft.Authorization/roleassignmentsusagemetrics"
	urlPath = strings.ReplaceAll(urlPath, "{subscriptionId}", url.PathEscape(*subscription.SubscriptionID))

	req, err := runtime.NewRequest(ctx, http.MethodGet, runtime.JoinPaths(ep, urlPath))
	if err != nil {
		panic(err)
	}
	reqQP := req.Raw().URL.Query()
	reqQP.Set("api-version", "2019-08-01-preview")
	req.Raw().URL.RawQuery = reqQP.Encode()
	req.Raw().Header["Accept"] = []string{"application/json"}

	resp, err := pl.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if runtime.HasStatusCode(resp, http.StatusOK) {
		result := struct {
			RoleAssignmentsLimit          float64 `json:"roleAssignmentsLimit"`
			RoleAssignmentsCurrentCount   float64 `json:"roleAssignmentsCurrentCount"`
			RoleAssignmentsRemainingCount float64 `json:"roleAssignmentsRemainingCount"`
		}{}

		if err := runtime.UnmarshalAsJSON(resp, &result); err == nil {
			currentValue := result.RoleAssignmentsCurrentCount
			limitValue := result.RoleAssignmentsLimit

			labels := prometheus.Labels{
				"subscriptionID": to.StringLower(subscription.SubscriptionID),
				"location":       "",
				"provider":       "microsoft.authorization",
				"scope":          "authorization",
				"quota":          "RoleAssignments",
				"quotaName":      "Role Assignments",
			}

			quotaMetric.Add(labels, 1)
			quotaCurrentMetric.Add(labels, currentValue)
			quotaLimitMetric.Add(labels, limitValue)
			if limitValue != 0 {
				quotaUsageMetric.Add(labels, currentValue/limitValue)
			}
		}
	}
}

// collectQuotaUsage collect generic quota usages
// func (m *MetricsCollectorAzureRmQuota) collectQuotaUsage(subscription *armsubscriptions.Subscription, provider string, logger *slog.Logger, callback chan<- func()) {
// 	client, err := armquota.NewUsagesClient(AzureClient.GetCred(), AzureClient.NewArmClientOptions())
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	quotaMetric := m.Collector.GetMetricList("quota")
// 	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
// 	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
// 	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")
//
// 	for _, location := range Opts.Azure.Location {
// 		scope := "/subscriptions/{subscriptionId}/providers/{provider}/locations/{location}"
// 		scope = strings.ReplaceAll(scope, "{subscriptionId}", url.PathEscape(*subscription.SubscriptionID))
// 		scope = strings.ReplaceAll(scope, "{provider}", url.PathEscape(provider))
// 		scope = strings.ReplaceAll(scope, "{location}", url.PathEscape(location))
//
// 		pager := client.NewListPager(scope, nil)
// 		for pager.More() {
// 			result, err := pager.NextPage(m.Context())
// 			if err != nil {
// 				panic(err)
// 			}
//
// 			if result.Value == nil {
// 				continue
// 			}
//
// 			for _, resourceUsage := range result.Value {
// 				currentValue := float64(to.Number(resourceUsage.Properties.Usages.Value))
// 				limitValue := float64(to.Number(resourceUsage.Properties.Usages.Limit))
//
// 				labels := prometheus.Labels{
// 					"subscriptionID": to.StringLower(subscription.SubscriptionID),
// 					"location":       strings.ToLower(location),
// 					"scope":          provider,
// 					"quota":          to.String(resourceUsage.Properties.Name.Value),
// 					"quotaName":      to.String(resourceUsage.Properties.Name.LocalizedValue),
// 				}
//
// 				quotaMetric.Add(labels, 1)
// 				quotaCurrentMetric.Add(labels, currentValue)
// 				quotaLimitMetric.Add(labels, limitValue)
// 				if limitValue != 0 {
// 					quotaUsageMetric.Add(labels, currentValue/limitValue)
// 				}
// 			}
// 		}
// 	}
// }

// collectAzureComputeUsage collects compute usages
func (m *MetricsCollectorAzureRmQuota) collectAzureComputeUsage(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	client, err := armcompute.NewUsageClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

	for _, location := range Config.Azure.Locations {
		pager := client.NewListPager(location, nil)

		for pager.More() {
			result, err := pager.NextPage(m.Context())
			if err != nil {
				panic(err)
			}

			if result.Value == nil {
				continue
			}

			for _, resourceUsage := range result.Value {
				currentValue := float64(to.Number(resourceUsage.CurrentValue))
				limitValue := float64(to.Number(resourceUsage.Limit))

				labels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"provider":       "microsoft.compute",
					"scope":          "compute",
					"quota":          to.String(resourceUsage.Name.Value),
					"quotaName":      to.String(resourceUsage.Name.LocalizedValue),
				}

				quotaMetric.Add(labels, 1)
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
func (m *MetricsCollectorAzureRmQuota) collectAzureNetworkUsage(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	client, err := armnetwork.NewUsagesClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

	for _, location := range Config.Azure.Locations {
		pager := client.NewListPager(location, nil)

		for pager.More() {
			result, err := pager.NextPage(m.Context())
			if err != nil {
				panic(err)
			}

			if result.Value == nil {
				continue
			}

			for _, resourceUsage := range result.Value {
				currentValue := float64(to.Number(resourceUsage.CurrentValue))
				limitValue := float64(to.Number(resourceUsage.Limit))

				labels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"provider":       "microsoft.network",
					"scope":          "network",
					"quota":          to.String(resourceUsage.Name.Value),
					"quotaName":      to.String(resourceUsage.Name.LocalizedValue),
				}

				quotaMetric.Add(labels, 1)
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
func (m *MetricsCollectorAzureRmQuota) collectAzureStorageUsage(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	client, err := armstorage.NewUsagesClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

	for _, location := range Config.Azure.Locations {
		pager := client.NewListByLocationPager(location, nil)

		for pager.More() {
			result, err := pager.NextPage(m.Context())
			if err != nil {
				panic(err)
			}

			if result.Value == nil {
				continue
			}

			for _, resourceUsage := range result.Value {
				currentValue := float64(to.Number(resourceUsage.CurrentValue))
				limitValue := float64(to.Number(resourceUsage.Limit))

				labels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"provider":       "microsoft.storage",
					"scope":          "storage",
					"quota":          to.String(resourceUsage.Name.Value),
					"quotaName":      to.String(resourceUsage.Name.LocalizedValue),
				}

				quotaMetric.Add(labels, 1)
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
func (m *MetricsCollectorAzureRmQuota) collectAzureMachineLearningUsage(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	client, err := armmachinelearning.NewUsagesClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

	for _, location := range Config.Azure.Locations {
		pager := client.NewListPager(location, nil)

		for pager.More() {
			result, err := pager.NextPage(m.Context())
			if err != nil {
				panic(err)
			}

			if result.Value == nil {
				continue
			}

			for _, resourceUsage := range result.Value {
				currentValue := float64(to.Number(resourceUsage.CurrentValue))
				limitValue := float64(to.Number(resourceUsage.Limit))

				labels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"provider":       "microsoft.machinelearningservices",
					"scope":          "machinelearningservices",
					"quota":          to.String(resourceUsage.Name.Value),
					"quotaName":      to.String(resourceUsage.Name.LocalizedValue),
				}

				quotaMetric.Add(labels, 1)
				quotaCurrentMetric.Add(labels, currentValue)
				quotaLimitMetric.Add(labels, limitValue)
				if limitValue != 0 {
					quotaUsageMetric.Add(labels, currentValue/limitValue)
				}
			}
		}
	}
}
