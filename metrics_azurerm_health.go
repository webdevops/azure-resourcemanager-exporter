package main

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcehealth/armresourcehealth"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
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
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *log.Entry) {
		m.collectSubscription(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmHealth) collectSubscription(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armresourcehealth.NewAvailabilityStatusesClient(*subscription.SubscriptionID, AzureClient.GetCred(), nil)
	if err != nil {
		logger.Panic(err)
	}

	resourceHealthMetric := prometheusCommon.NewMetricsList()

	availabilityStateValues := armresourcehealth.PossibleAvailabilityStateValuesValues()

	pager := client.NewListBySubscriptionIDPager(nil)

	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Panic(err)
		}

		if result.Value == nil {
			continue
		}

		for _, resourceHealth := range result.Value {

			resourceId := to.String(resourceHealth.ID)
			resourceId = stringsTrimSuffixCI(resourceId, "/providers/"+to.String(resourceHealth.Type)+"/"+to.String(resourceHealth.Name))
			azureResource, _ := armclient.ParseResourceId(resourceId)

			resourceAvailabilityState := armresourcehealth.AvailabilityStateValuesUnknown
			if resourceHealth.Properties != nil && resourceHealth.Properties.AvailabilityState != nil {
				resourceAvailabilityState = *resourceHealth.Properties.AvailabilityState
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
		}
	}

	callback <- func() {
		resourceHealthMetric.GaugeSet(m.prometheus.resourceHealth)
	}
}
