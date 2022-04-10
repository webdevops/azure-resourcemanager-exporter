package main

import (
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorAzureRmRateLimitRead struct {
	collector.Processor
}

func (m *MetricsCollectorAzureRmRateLimitRead) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)
}

func (m *MetricsCollectorAzureRmRateLimitRead) Reset() {
}

func (m *MetricsCollectorAzureRmRateLimitRead) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription subscriptions.Subscription, logger *log.Entry) {
		m.collectSubscription(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmRateLimitRead) collectSubscription(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client := subscriptions.NewClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint)
	AzureClient.DecorateAzureAutorest(&client.Client)

	_, err := client.Get(m.Context(), *subscription.SubscriptionID)
	if err != nil {
		logger.Panic(err)
	}
}
