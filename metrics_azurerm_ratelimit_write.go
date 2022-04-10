package main

import (
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorAzureRmRateLimitWrite struct {
	collector.Processor
}

func (m *MetricsCollectorAzureRmRateLimitWrite) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)
}

func (m *MetricsCollectorAzureRmRateLimitWrite) Reset() {

}

func (m *MetricsCollectorAzureRmRateLimitWrite) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription subscriptions.Subscription, logger *log.Entry) {
		m.collectSubscription(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmRateLimitWrite) collectSubscription(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client := resources.NewTagsClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	AzureClient.DecorateAzureAutorest(&client.Client)

	params := resources.TagsPatchResource{
		Operation: "Merge",
		Properties: &resources.Tags{
			Tags: map[string]*string{},
		},
	}
	_, err := client.UpdateAtScope(m.Context(), *subscription.ID, params)
	if err != nil {
		logger.Warn(err)
	}
}
