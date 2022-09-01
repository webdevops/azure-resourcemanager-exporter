package main

import (
	"strings"

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
		resourceHealth                         *prometheus.GaugeVec
		resourceHealthReportTime               *prometheus.GaugeVec
		resourceHealthRootCauseAttributionTime *prometheus.GaugeVec
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
			"healthEventType",
			"healthEventCategory",
			"healthEventCause",
			"reason",
			"summary",
		},
	)
	prometheus.MustRegister(m.prometheus.resourceHealth)

	m.prometheus.resourceHealthReportTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resource_health_reporttime",
			Help: "Azure Resource health status information",
		},
		[]string{
			"subscriptionID",
			"resourceID",
			"resourceGroup",
		},
	)
	prometheus.MustRegister(m.prometheus.resourceHealthReportTime)

	m.prometheus.resourceHealthRootCauseAttributionTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resource_health_rootcauseattributiontime",
			Help: "Azure Resource health status information",
		},
		[]string{
			"subscriptionID",
			"resourceID",
			"resourceGroup",
		},
	)
	prometheus.MustRegister(m.prometheus.resourceHealthRootCauseAttributionTime)
}

func (m *MetricsCollectorAzureRmHealth) Reset() {
	m.prometheus.resourceHealth.Reset()
	m.prometheus.resourceHealthReportTime.Reset()
	m.prometheus.resourceHealthRootCauseAttributionTime.Reset()
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
	client, err := armresourcehealth.NewAvailabilityStatusesClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	resourceHealthMetric := prometheusCommon.NewMetricsList()
	resourceHealthReportTimeMetric := prometheusCommon.NewMetricsList()
	resourceHealthRootCauseAttributionTimeMetric := prometheusCommon.NewMetricsList()

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

			if resourceHealth.Properties.ReportedTime != nil {
				resourceHealthReportTimeMetric.AddTime(prometheus.Labels{
					"subscriptionID": azureResource.Subscription,
					"resourceID":     stringToStringLower(resourceId),
					"resourceGroup":  azureResource.ResourceGroup,
				}, *resourceHealth.Properties.ReportedTime)
			}

			if resourceHealth.Properties.RootCauseAttributionTime != nil {
				resourceHealthRootCauseAttributionTimeMetric.AddTime(prometheus.Labels{
					"subscriptionID": azureResource.Subscription,
					"resourceID":     stringToStringLower(resourceId),
					"resourceGroup":  azureResource.ResourceGroup,
				}, *resourceHealth.Properties.RootCauseAttributionTime)
			}

			for _, availabilityState := range availabilityStateValues {
				if availabilityState == resourceAvailabilityState {
					summary := ""
					if !strings.EqualFold(string(resourceAvailabilityState), string(armresourcehealth.AvailabilityStateValuesAvailable)) {
						summary = truncateStrings(to.String(resourceHealth.Properties.Summary), opts.ResourceHealth.SummaryMaxLength, "...")
					}

					resourceHealthMetric.Add(prometheus.Labels{
						"subscriptionID":      azureResource.Subscription,
						"resourceID":          stringToStringLower(resourceId),
						"resourceGroup":       azureResource.ResourceGroup,
						"availabilityState":   stringToStringLower(string(availabilityState)),
						"healthEventType":     to.String(resourceHealth.Properties.HealthEventType),
						"healthEventCategory": to.String(resourceHealth.Properties.HealthEventType),
						"healthEventCause":    to.String(resourceHealth.Properties.HealthEventCause),
						"reason":              to.String(resourceHealth.Properties.ReasonType),
						"summary":             summary,
					}, 1)
				}
			}
		}
	}

	callback <- func() {
		resourceHealthMetric.GaugeSet(m.prometheus.resourceHealth)
		resourceHealthReportTimeMetric.GaugeSet(m.prometheus.resourceHealthReportTime)
		resourceHealthRootCauseAttributionTimeMetric.GaugeSet(m.prometheus.resourceHealthRootCauseAttributionTime)
	}
}
