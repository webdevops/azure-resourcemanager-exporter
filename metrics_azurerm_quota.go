package main

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	armruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"

	"github.com/webdevops/azure-resourcemanager-exporter/config"
	"github.com/webdevops/azure-resourcemanager-exporter/models/quota"
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

	if len(Config.Collectors.Quota.ResourceProviders) == 0 {
		panic("no resourceProviders defined for quota [collectors.quota.resourceProviders]")
	}

	m.prometheus.quota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_info",
			Help: "Azure ResourceManager quota information",
		},
		[]string{
			"subscriptionID",
			"location",
			"provider",
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

		if registered, err := AzureClient.IsResourceProviderRegistered(m.Context(), *subscription.SubscriptionID, "Microsoft.Capacity"); registered {
			for _, provider := range Config.Collectors.Quota.ResourceProviders {
				if registered, err := AzureClient.IsResourceProviderRegistered(m.Context(), *subscription.SubscriptionID, provider.Provider); registered {

					for _, location := range Config.Azure.Locations {
						quotaLogger := logger.With(slog.String("provider", provider.Provider), slog.String("location", location))
						m.collectQuotaUsage(subscription, provider, location, quotaLogger, callback)
					}

				} else if err != nil {
					logger.Error("quota for resourceProvider requested, but not registered", slog.String("resourceProvider", provider.Provider), slog.Any("error", err))
				}
			}
		} else if err != nil {
			logger.Error("resourceProvider Microsoft.Capacity is needed for quotas", slog.Any("error", err))
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

	urlPath := "/subscriptions/{subscriptionId}/providers/Microsoft.Authorization/roleassignmentsusagemetrics"
	urlPath = strings.ReplaceAll(urlPath, "{subscriptionId}", url.PathEscape(*subscription.SubscriptionID))

	req, err := runtime.NewRequest(m.Context(), http.MethodGet, runtime.JoinPaths(ep, urlPath))
	if err != nil {
		panic(err)
	}
	defer req.Close()
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
		result := quota.RoleAssignmentUsage{}

		if err := runtime.UnmarshalAsJSON(resp, &result); err == nil {
			currentValue := result.RoleAssignmentsCurrentCount
			limitValue := result.RoleAssignmentsLimit

			labels := prometheus.Labels{
				"subscriptionID": to.StringLower(subscription.SubscriptionID),
				"location":       "",
				"provider":       "microsoft.authorization",
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
func (m *MetricsCollectorAzureRmQuota) collectQuotaUsage(subscription *armsubscriptions.Subscription, provider config.CollectorQuotaResourceProvider, location string, logger *slog.Logger, callback chan<- func()) {
	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

	if provider.ApiVersion == "" || strings.EqualFold(provider.ApiVersion, "auto") {
		provider.ApiVersion = ""

		// lookup api version
		providerInfo, err := AzureClient.GetResourceProvider(m.Context(), *subscription.SubscriptionID, provider.Provider)
		if err != nil {
			logger.Error("failed to lookup Azure resource provider", slog.Any("error", err))
			panic(err)
		}

		if providerInfo != nil {
			for _, providerResource := range providerInfo.ResourceTypes {
				if to.String(providerResource.DefaultAPIVersion) != "" {
					provider.ApiVersion = to.String(providerResource.DefaultAPIVersion)
					break
				}
			}
		}

		if provider.ApiVersion == "" {
			logger.Error("failed to lookup Azure resource provider apiVersion")
			panic("failed to lookup Azure resource provider apiVersion")
		}
	}

	logger = logger.With(slog.String("apiVersion", provider.ApiVersion))

	options := AzureClient.NewArmClientOptions()
	ep := cloud.AzurePublic.Services[cloud.ResourceManager].Endpoint
	if c, ok := options.Cloud.Services[cloud.ResourceManager]; ok {
		ep = c.Endpoint
	}

	pl, err := armruntime.NewPipeline("azurerm-quota", gitTag, AzureClient.GetCred(), runtime.PipelineOptions{}, options)
	if err != nil {
		logger.Error("failed to create arm client", slog.Any("error", err))
		panic(err)
	}

	urlPath := "/subscriptions/{subscriptionId}/providers/{provider}/locations/{location}/usages"
	urlPath = strings.ReplaceAll(urlPath, "{subscriptionId}", url.PathEscape(*subscription.SubscriptionID))
	urlPath = strings.ReplaceAll(urlPath, "{provider}", url.PathEscape(provider.Provider))
	urlPath = strings.ReplaceAll(urlPath, "{location}", url.PathEscape(location))

	logger.Info("fetch resource usage and quota")
	requestUrl := runtime.JoinPaths(ep, urlPath)
	for {
		req, err := runtime.NewRequest(m.Context(), http.MethodGet, requestUrl)
		if err != nil {
			logger.Error("failed to create request", slog.Any("error", err))
			panic(err)
		}
		defer req.Close()
		reqQP := req.Raw().URL.Query()
		reqQP.Set("api-version", provider.ApiVersion)
		req.Raw().URL.RawQuery = reqQP.Encode()
		req.Raw().Header["Accept"] = []string{"application/json"}

		resp, err := pl.Do(req)
		if err != nil {
			logger.Error("failed to send request", slog.Any("error", err))
			panic(err)
		}
		defer resp.Body.Close()

		if !runtime.HasStatusCode(resp, http.StatusOK) {
			buf := new(strings.Builder)
			_, err := io.Copy(buf, resp.Body)
			if err != nil {
				panic(err)
			}
			logger.Error("request failed", slog.String("statusCode", resp.Status), slog.Any("error", errors.New(buf.String())))
			panic("request failed")
		}

		result := quota.ListUsageResult{}
		if err := runtime.UnmarshalAsJSON(resp, &result); err == nil {
			for _, quotaUsage := range result.Value {

				labels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"provider":       provider.Provider,
					"quota":          to.String(quotaUsage.Name.Value),
					"quotaName":      to.String(quotaUsage.Name.LocalizedValue),
				}

				quotaMetric.Add(labels, 1)
				quotaCurrentMetric.AddIfNotNil(labels, quotaUsage.CurrentValue)
				quotaLimitMetric.AddIfNotNil(labels, quotaUsage.Limit)
				if quotaUsage.CurrentValue != nil && quotaUsage.Limit != nil && *quotaUsage.Limit != 0 {
					quotaUsageMetric.Add(labels, *quotaUsage.CurrentValue / *quotaUsage.Limit)
				}
			}
		} else {
			logger.Error("failed to parse request", slog.Any("error", err))
			panic("parsing failed")
		}

		if result.NextLink != nil && *result.NextLink != "" {
			requestUrl = *result.NextLink
			continue
		}

		break
	}
}
