package main

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	log "github.com/sirupsen/logrus"
)

type MetricsCollectorAzureRmRateLimitRead struct {
	CollectorProcessorGeneral
}

func (m *MetricsCollectorAzureRmRateLimitRead) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector
}

func (m *MetricsCollectorAzureRmRateLimitRead) Reset() {
}

func (m *MetricsCollectorAzureRmRateLimitRead) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := subscriptions.NewClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint)
	decorateAzureAutorest(&client.Client)

	_, err := client.Get(ctx, *subscription.SubscriptionID)
	if err != nil {
		logger.Panic(err)
	}

}
