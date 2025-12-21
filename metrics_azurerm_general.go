package main

import (
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type MetricsCollectorAzureRmGeneral struct {
	collector.Processor

	prometheus struct {
		subscription  *prometheus.GaugeVec
		resourceGroup *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmGeneral) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

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
	m.Collector.RegisterMetricList("subscription", m.prometheus.subscription, true)

}

func (m *MetricsCollectorAzureRmGeneral) Reset() {}

func (m *MetricsCollectorAzureRmGeneral) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *slog.Logger) {
		m.collectSubscription(subscription, logger, callback)
	})
	if err != nil {
		panic(err)
	}
}

// Collect Azure Subscription metrics
func (m *MetricsCollectorAzureRmGeneral) collectSubscription(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	subscriptionMetric := m.Collector.GetMetricList("subscription")

	spendingLimit := ""
	if subscription.SubscriptionPolicies.SpendingLimit != nil {
		spendingLimit = string(*subscription.SubscriptionPolicies.SpendingLimit)
	}

	subscriptionMetric.AddInfo(prometheus.Labels{
		"resourceID":          to.StringLower(subscription.ID),
		"subscriptionID":      to.StringLower(subscription.SubscriptionID),
		"subscriptionName":    to.String(subscription.DisplayName),
		"spendingLimit":       spendingLimit,
		"quotaID":             to.StringLower(subscription.SubscriptionPolicies.QuotaID),
		"locationPlacementID": to.StringLower(subscription.SubscriptionPolicies.LocationPlacementID),
	})
}
