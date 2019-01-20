package old

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resourcehealth/mgmt/resourcehealth"
	"github.com/prometheus/client_golang/prometheus"
)

func (m *MetricCollectorAzureRm) initResourceHealth() {
	m.prometheus.resourceHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resourcehealth_info",
			Help: "Azure ResourceHealth info",
		},
		[]string{"resourceID", "subscriptionID", "availabilityState"},
	)

	prometheus.MustRegister(m.prometheus.resourceHealth)
}

// Collect Azure ComputeUsage metrics
func (m *MetricCollectorAzureRm) collectAzureResourceHealth(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := resourcehealth.NewAvailabilityStatusesClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListBySubscriptionIDComplete(ctx, subscriptionId, "")

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
				"subscriptionID": subscriptionId,
				"resourceID": resourceId,
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
