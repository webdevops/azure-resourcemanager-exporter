package main

import (
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resourcehealth/mgmt/resourcehealth"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	azureCommon "github.com/webdevops/go-common/azure"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorAzureRmHealth struct {
	collector.Processor

	prometheus struct {
		resourceHealth *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmHealth) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.resourceHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resource_health",
			Help: "Azure Resource health status information",
		},
		[]string{
			"subscriptionID",
			"resourceID",
			"resourceGroup",
			"availabilityState",
		},
	)
	prometheus.MustRegister(m.prometheus.resourceHealth)
}

func (m *MetricsCollectorAzureRmHealth) Reset() {
	m.prometheus.resourceHealth.Reset()
}

func (m *MetricsCollectorAzureRmHealth) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription subscriptions.Subscription, logger *log.Entry) {
		m.collectSubscription(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmHealth) collectSubscription(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client := resourcehealth.NewAvailabilityStatusesClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	AzureClient.DecorateAzureAutorest(&client.Client)

	list, err := client.ListBySubscriptionIDComplete(m.Context(), "", "")

	if err != nil {
		logger.Panic(err)
	}

	availabilityStateValues := resourcehealth.PossibleAvailabilityStateValuesValues()

	resourceHealthMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		resourceId := to.String(val.ID)
		resourceId = stringsTrimSuffixCI(resourceId, "/providers/"+to.String(val.Type)+"/"+to.String(val.Name))
		azureResource, _ := azureCommon.ParseResourceId(resourceId)

		resourceAvailabilityState := resourcehealth.AvailabilityStateValuesUnknown

		if val.Properties != nil {
			resourceAvailabilityState = val.Properties.AvailabilityState
		}

		for _, availabilityState := range availabilityStateValues {
			if availabilityState == resourceAvailabilityState {
				resourceHealthMetric.Add(prometheus.Labels{
					"subscriptionID":    azureResource.Subscription,
					"resourceID":        stringToStringLower(resourceId),
					"resourceGroup":     azureResource.ResourceGroup,
					"availabilityState": stringToStringLower(string(availabilityState)),
				}, 1)
			}
		}

		if list.NextWithContext(m.Context()) != nil {
			break
		}
	}

	callback <- func() {
		resourceHealthMetric.GaugeSet(m.prometheus.resourceHealth)
	}
}
