package main

import (
	"encoding/json"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcehealth/armresourcehealth"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/azuresdk/armclient"
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
	m.Collector.RegisterMetricList("resourceHealth", m.prometheus.resourceHealth, true)

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
	m.Collector.RegisterMetricList("resourceHealthReportTime", m.prometheus.resourceHealthReportTime, true)

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
	m.Collector.RegisterMetricList("resourceHealthRootCauseAttributionTime", m.prometheus.resourceHealthRootCauseAttributionTime, true)
}

func (m *MetricsCollectorAzureRmHealth) Reset() {}

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

	resourceHealthMetric := m.Collector.GetMetricList("resourceHealth")
	resourceHealthReportTimeMetric := m.Collector.GetMetricList("resourceHealthReportTime")
	resourceHealthRootCauseAttributionTimeMetric := m.Collector.GetMetricList("resourceHealthRootCauseAttributionTime")

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

						// log resourcehealth
						var resourceHealthLogObject interface{}
						if resourceHealthData, err := json.Marshal(resourceHealth); err == nil {
							err := json.Unmarshal(resourceHealthData, &resourceHealthLogObject)
							if err != nil {
								m.Logger().Warnf("unable to convert resourcehealth to json: %v", err.Error())
							}
						}

						m.Logger().WithFields(log.Fields{
							"subscriptionID":    azureResource.Subscription,
							"resourceID":        stringToStringLower(resourceId),
							"resourceGroup":     azureResource.ResourceGroup,
							"availabilityState": stringToStringLower(string(availabilityState)),
							"resourceHealth":    resourceHealthLogObject,
						}).Info("unhealthy resource detected")

						if opts.ResourceHealth.SummaryMaxLength > 0 {
							summary = truncateStrings(to.String(resourceHealth.Properties.Summary), opts.ResourceHealth.SummaryMaxLength, "...")
						}
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
}
