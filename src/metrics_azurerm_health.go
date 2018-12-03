package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resourcehealth/mgmt/resourcehealth"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollectorAzureRmHealth struct {
	CollectorProcessorGeneral

	prometheus struct {
		resourceHealth *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmHealth) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.resourceHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resourcehealth_info",
			Help: "Azure ResourceHealth info",
		},
		[]string{"resourceID", "subscriptionID", "resourceGroup", "availabilityState"},
	)

	prometheus.MustRegister(m.prometheus.resourceHealth)
}

func (m *MetricsCollectorAzureRmHealth) Reset() {
	m.prometheus.resourceHealth.Reset()
}

func (m *MetricsCollectorAzureRmHealth) Collect(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	client := resourcehealth.NewAvailabilityStatusesClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListBySubscriptionIDComplete(ctx, *subscription.SubscriptionID, "")

	if err != nil {
		panic(err)
	}

	availabilityStateValues := resourcehealth.PossibleAvailabilityStateValuesValues()


	resourceHealthMetric := MetricCollectorList{}

	for list.NotDone() {
		val := list.Value()

		resourceId := stringsTrimSuffixCI(*val.ID, ("/providers/" + *val.Type + "/" + *val.Name))

		resourceAvailabilityState := resourcehealth.Unknown

		if val.Properties != nil {
			resourceAvailabilityState = val.Properties.AvailabilityState
		}

		for _, availabilityState := range availabilityStateValues {
			labels := prometheus.Labels{
				"subscriptionID": *subscription.SubscriptionID,
				"resourceID": resourceId,
				"resourceGroup": extractResourceGroupFromAzureId(resourceId),
				"availabilityState": string(availabilityState),
			}

			if availabilityState == resourceAvailabilityState {
				resourceHealthMetric.Add(labels, 1)
			} else {
				resourceHealthMetric.Add(labels, 0)
			}
		}

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		resourceHealthMetric.GaugeSet(m.prometheus.resourceHealth)
	}
}
