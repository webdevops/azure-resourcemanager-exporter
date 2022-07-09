package main

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
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
	prometheus.MustRegister(m.prometheus.subscription)

}

func (m *MetricsCollectorAzureRmGeneral) Reset() {
	m.prometheus.subscription.Reset()
}

func (m *MetricsCollectorAzureRmGeneral) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *log.Entry) {
		m.collectSubscription(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

// Collect Azure Subscription metrics
func (m *MetricsCollectorAzureRmGeneral) collectSubscription(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	subscriptionMetric := prometheusCommon.NewMetricsList()
	spendingLimit := ""
	if subscription.SubscriptionPolicies.SpendingLimit != nil {
		spendingLimit = string(*subscription.SubscriptionPolicies.SpendingLimit)
	}

	subscriptionMetric.AddInfo(prometheus.Labels{
		"resourceID":          stringPtrToStringLower(subscription.ID),
		"subscriptionID":      stringPtrToStringLower(subscription.SubscriptionID),
		"subscriptionName":    to.String(subscription.DisplayName),
		"spendingLimit":       spendingLimit,
		"quotaID":             stringPtrToStringLower(subscription.SubscriptionPolicies.QuotaID),
		"locationPlacementID": stringPtrToStringLower(subscription.SubscriptionPolicies.LocationPlacementID),
	})

	callback <- func() {
		subscriptionMetric.GaugeSet(m.prometheus.subscription)
	}
}
