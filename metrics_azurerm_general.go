package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
)

type MetricsCollectorAzureRmGeneral struct {
	CollectorProcessorGeneral

	prometheus struct {
		subscription  *prometheus.GaugeVec
		resourceGroup *prometheus.GaugeVec
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
	prometheus.MustRegister(m.prometheus.subscription)

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
	prometheus.MustRegister(m.prometheus.resourceGroup)
}

func (m *MetricsCollectorAzureRmGeneral) Reset() {
	m.prometheus.subscription.Reset()
	m.prometheus.resourceGroup.Reset()
}

func (m *MetricsCollectorAzureRmGeneral) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureSubscription(ctx, logger, callback, subscription)
	m.collectAzureResourceGroup(ctx, logger, callback, subscription)
}

// Collect Azure Subscription metrics
func (m *MetricsCollectorAzureRmGeneral) collectAzureSubscription(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := subscriptions.NewClient()
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

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

	callback <- func() {
		subscriptionMetric.GaugeSet(m.prometheus.subscription)
	}
}

// Collect Azure ResourceGroup metrics
func (m *MetricsCollectorAzureRmGeneral) collectAzureResourceGroup(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := resources.NewGroupsClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

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
