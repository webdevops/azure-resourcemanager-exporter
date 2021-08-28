package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resourcehealth/mgmt/resourcehealth"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
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
			Name: "azurerm_resource_health",
			Help: "Azure Resource health info",
		},
		[]string{
			"subscriptionID",
			"resourceID",
			"availabilityState",
		},
	)
	prometheus.MustRegister(m.prometheus.resourceHealth)
}

func (m *MetricsCollectorAzureRmHealth) Reset() {
	m.prometheus.resourceHealth.Reset()
}

func (m *MetricsCollectorAzureRmHealth) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := resourcehealth.NewAvailabilityStatusesClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	list, err := client.ListBySubscriptionIDComplete(ctx, *subscription.SubscriptionID, "")

	if err != nil {
		logger.Panic(err)
	}

	availabilityStateValues := resourcehealth.PossibleAvailabilityStateValuesValues()

	resourceHealthMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		resourceId := stringsTrimSuffixCI(to.String(val.ID), ("/providers/" + *val.Type + "/" + *val.Name))

		resourceAvailabilityState := resourcehealth.Unknown

		if val.Properties != nil {
			resourceAvailabilityState = val.Properties.AvailabilityState
		}

		for _, availabilityState := range availabilityStateValues {
			labels := prometheus.Labels{
				"subscriptionID":    to.String(subscription.SubscriptionID),
				"resourceID":        toResourceId(&resourceId),
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
