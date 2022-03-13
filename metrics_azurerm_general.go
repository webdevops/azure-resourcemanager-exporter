package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest/to"
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

}

func (m *MetricsCollectorAzureRmGeneral) Reset() {
	m.prometheus.subscription.Reset()
}

func (m *MetricsCollectorAzureRmGeneral) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureSubscription(ctx, logger, callback, subscription)
}

// Collect Azure Subscription metrics
func (m *MetricsCollectorAzureRmGeneral) collectAzureSubscription(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := subscriptions.NewClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint)
	decorateAzureAutorest(&client.Client)

	sub, err := client.Get(ctx, *subscription.SubscriptionID)
	if err != nil {
		logger.Panic(err)
	}

	subscriptionMetric := prometheusCommon.NewMetricsList()
	subscriptionMetric.AddInfo(prometheus.Labels{
		"resourceID":          stringPtrToAzureResourceInfo(sub.ID),
		"subscriptionID":      stringPtrToAzureResourceInfo(sub.SubscriptionID),
		"subscriptionName":    to.String(sub.DisplayName),
		"spendingLimit":       string(sub.SubscriptionPolicies.SpendingLimit),
		"quotaID":             stringPtrToAzureResourceInfo(sub.SubscriptionPolicies.QuotaID),
		"locationPlacementID": stringPtrToAzureResourceInfo(sub.SubscriptionPolicies.LocationPlacementID),
	})

	callback <- func() {
		subscriptionMetric.GaugeSet(m.prometheus.subscription)
	}
}
