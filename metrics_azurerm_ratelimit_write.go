package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	log "github.com/sirupsen/logrus"
)

type MetricsCollectorAzureRmRateLimitWrite struct {
	CollectorProcessorGeneral
}

func (m *MetricsCollectorAzureRmRateLimitWrite) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector
}

func (m *MetricsCollectorAzureRmRateLimitWrite) Reset() {
}

func (m *MetricsCollectorAzureRmRateLimitWrite) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := resources.NewTagsClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	params := resources.TagsPatchResource{
		Operation: "Merge",
		Properties: &resources.Tags{
			Tags: map[string]*string{},
		},
	}
	_, err := client.UpdateAtScope(ctx, *subscription.ID, params)
	if err != nil {
		logger.Warn(err)
	}
}
