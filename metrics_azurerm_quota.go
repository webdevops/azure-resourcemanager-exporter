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
	armquota "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/quota/armquota/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
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
				if registered, err := AzureClient.IsResourceProviderRegistered(m.Context(), *subscription.SubscriptionID, provider); registered {
					m.collectQuotaUsage(subscription, provider, logger, callback)
				} else if err != nil {
					logger.Error("quota for resourceProvider requested, but not registered", slog.String("resourceProvider", provider), slog.Any("error", err))
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

type (
	Quota struct {
		Labels  prometheus.Labels
		Limit   *float64
		Current *float64
	}

	QuotaList map[string]*Quota
)

func (q *QuotaList) Get(name string) *Quota {
	name = strings.ToLower(name)
	if _, ok := (*q)[name]; !ok {
		(*q)[name] = &Quota{}
	}

	return (*q)[name]
}

// collectQuotaUsage collect generic quota usages
func (m *MetricsCollectorAzureRmQuota) collectQuotaUsage(subscription *armsubscriptions.Subscription, provider string, logger *slog.Logger, callback chan<- func()) {
	quotaClient, err := armquota.NewClient(AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	usageClient, err := armquota.NewUsagesClient(AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	quotaMetric := m.Collector.GetMetricList("quota")
	quotaCurrentMetric := m.Collector.GetMetricList("quotaCurrent")
	quotaLimitMetric := m.Collector.GetMetricList("quotaLimit")
	quotaUsageMetric := m.Collector.GetMetricList("quotaUsage")

	for _, location := range Config.Azure.Locations {
		scope := "/subscriptions/{subscriptionId}/providers/{provider}/locations/{location}"
		scope = strings.ReplaceAll(scope, "{subscriptionId}", url.PathEscape(*subscription.SubscriptionID))
		scope = strings.ReplaceAll(scope, "{provider}", url.PathEscape(provider))
		scope = strings.ReplaceAll(scope, "{location}", url.PathEscape(location))

		quotaList := QuotaList{}

		// -----------------------
		// Quotas
		quotaPager := quotaClient.NewListPager(scope, nil)
		for quotaPager.More() {
			result, err := quotaPager.NextPage(m.Context())
			if err != nil {
				panic(err)
			}

			if result.Value == nil {
				continue
			}

			for _, row := range result.Value {
				switch v := row.Properties.Limit.(type) {
				case *armquota.LimitObject:
					if v.Value != nil {
						quotaName := to.String(row.Name)

						labels := prometheus.Labels{
							"subscriptionID": to.StringLower(subscription.SubscriptionID),
							"location":       strings.ToLower(location),
							"provider":       provider,
							"quota":          to.String(row.Properties.Name.Value),
							"quotaName":      to.String(row.Properties.Name.LocalizedValue),
						}

						quotaList.Get(quotaName).Limit = to.Ptr(float64(to.Number(v.Value)))
						quotaList.Get(quotaName).Labels = labels
					}
				}
			}
		}

		// -----------------------
		// Usages
		usagePager := usageClient.NewListPager(scope, nil)
		for usagePager.More() {
			result, err := usagePager.NextPage(m.Context())
			if err != nil {
				panic(err)
			}

			if result.Value == nil {
				continue
			}

			for _, row := range result.Value {
				quotaName := to.String(row.Name)
				value := float64(to.Number(row.Properties.Usages.Value))

				labels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"location":       strings.ToLower(location),
					"provider":       provider,
					"quota":          to.String(row.Properties.Name.Value),
					"quotaName":      to.String(row.Properties.Name.LocalizedValue),
				}

				quotaList.Get(quotaName).Current = to.Ptr(value)
				quotaList.Get(quotaName).Labels = labels
			}
		}

		for _, quota := range quotaList {
			quotaMetric.Add(quota.Labels, 1)
			quotaCurrentMetric.AddIfNotNil(quota.Labels, quota.Current)
			quotaLimitMetric.AddIfNotNil(quota.Labels, quota.Limit)
			if quota.Current != nil && quota.Limit != nil && *quota.Limit != 0 {
				quotaUsageMetric.Add(quota.Labels, *quota.Current / *quota.Limit)
			}
		}
	}
}
