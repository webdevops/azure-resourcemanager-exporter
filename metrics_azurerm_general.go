package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
	"strconv"
)

type MetricsCollectorAzureRmGeneral struct {
	CollectorProcessorGeneral

	prometheus struct {
		subscription  *prometheus.GaugeVec
		resourceGroup *prometheus.GaugeVec
		apiQuota      *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmGeneral) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.subscription = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_subscription_info",
			Help: "Azure ResourceManager subscription",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"subscriptionName",
			"spendingLimit",
			"quotaID",
			"locationPlacementID",
		},
	)

	m.prometheus.resourceGroup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resourcegroup_info",
			Help: "Azure ResourceManager resourcegroups",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"resourceGroup",
				"location",
			},
			azureResourceGroupTags.prometheusLabels...,
		),
	)

	m.prometheus.apiQuota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_ratelimit",
			Help: "Azure ResourceManager ratelimit",
		},
		[]string{
			"subscriptionID",
			"scope",
			"type",
		},
	)

	prometheus.MustRegister(m.prometheus.subscription)
	prometheus.MustRegister(m.prometheus.resourceGroup)
	prometheus.MustRegister(m.prometheus.apiQuota)
}

func (m *MetricsCollectorAzureRmGeneral) Reset() {
	m.prometheus.subscription.Reset()
	m.prometheus.resourceGroup.Reset()
	m.prometheus.apiQuota.Reset()
}

func (m *MetricsCollectorAzureRmGeneral) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureSubscription(ctx, logger, callback, subscription)
	m.collectAzureResourceGroup(ctx, logger, callback, subscription)
}

// Collect Azure Subscription metrics
func (m *MetricsCollectorAzureRmGeneral) collectAzureSubscription(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := subscriptions.NewClient()
	client.Authorizer = AzureAuthorizer

	sub, err := client.Get(ctx, *subscription.SubscriptionID)
	if err != nil {
		logger.Panic(err)
	}

	subscriptionMetric := prometheusCommon.NewMetricsList()
	subscriptionMetric.AddInfo(prometheus.Labels{
		"resourceID":          *sub.ID,
		"subscriptionID":      *sub.SubscriptionID,
		"subscriptionName":    stringPtrToString(sub.DisplayName),
		"spendingLimit":       string(sub.SubscriptionPolicies.SpendingLimit),
		"quotaID":             stringPtrToString(sub.SubscriptionPolicies.QuotaID),
		"locationPlacementID": stringPtrToString(sub.SubscriptionPolicies.LocationPlacementID),
	})

	// subscription rate limits
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-reads", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "subscription", "type": "read"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-requests", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "subscription", "type": "resource-requests"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-entities-read", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "subscription", "type": "resource-entities-read"}, callback)

	// tenant rate limits
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-reads", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "tenant", "type": "read"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-requests", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "tenant", "type": "resource-requests"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-entities-read", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "tenant", "type": "resource-entities-read"}, callback)

	callback <- func() {
		subscriptionMetric.GaugeSet(m.prometheus.subscription)
	}
}

// Collect Azure ResourceGroup metrics
func (m *MetricsCollectorAzureRmGeneral) collectAzureResourceGroup(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := resources.NewGroupsClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	resourceGroupResult, err := client.ListComplete(ctx, "", nil)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	for _, item := range *resourceGroupResult.Response().Value {
		infoLabels := azureResourceGroupTags.appendPrometheusLabel(prometheus.Labels{
			"resourceID":     *item.ID,
			"subscriptionID": *subscription.SubscriptionID,
			"resourceGroup":  stringPtrToString(item.Name),
			"location":       stringPtrToString(item.Location),
		}, item.Tags)
		infoMetric.AddInfo(infoLabels)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.resourceGroup)
	}
}

// read header and set prometheus api quota (if found)
func (m *MetricsCollectorAzureRmGeneral) probeProcessHeader(response autorest.Response, header string, labels prometheus.Labels, callback chan<- func()) {
	if val := response.Header.Get(header); val != "" {
		valFloat, err := strconv.ParseFloat(val, 64)

		if err == nil {
			callback <- func() {
				m.prometheus.apiQuota.With(labels).Set(valFloat)
			}
		} else {
			m.logger().Errorf("failed to parse value '%v': %v", val, err)
		}
	}
}
