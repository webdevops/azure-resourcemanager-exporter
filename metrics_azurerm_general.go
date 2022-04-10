package main

import (
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
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
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription subscriptions.Subscription, logger *log.Entry) {
		m.collectSubscription(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

// Collect Azure Subscription metrics
func (m *MetricsCollectorAzureRmGeneral) collectSubscription(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client := subscriptions.NewClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint)
	AzureClient.DecorateAzureAutorest(&client.Client)

	sub, err := client.Get(m.Context(), *subscription.SubscriptionID)
	if err != nil {
		logger.Panic(err)
	}

	subscriptionMetric := prometheusCommon.NewMetricsList()
	subscriptionMetric.AddInfo(prometheus.Labels{
		"resourceID":          stringPtrToStringLower(sub.ID),
		"subscriptionID":      stringPtrToStringLower(sub.SubscriptionID),
		"subscriptionName":    to.String(sub.DisplayName),
		"spendingLimit":       string(sub.SubscriptionPolicies.SpendingLimit),
		"quotaID":             stringPtrToStringLower(sub.SubscriptionPolicies.QuotaID),
		"locationPlacementID": stringPtrToStringLower(sub.SubscriptionPolicies.LocationPlacementID),
	})

	callback <- func() {
		subscriptionMetric.GaugeSet(m.prometheus.subscription)
	}
}
