package main

import (
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/advisor/mgmt/advisor"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/preview/preview/security/mgmt/security"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	azureCommon "github.com/webdevops/go-common/azure"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorAzureRmSecurity struct {
	collector.Processor

	prometheus struct {
		securitycenterCompliance *prometheus.GaugeVec
		advisorRecommendations   *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmSecurity) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.securitycenterCompliance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_securitycenter_compliance",
			Help: "Azure Audit SecurityCenter compliance status",
		},
		[]string{
			"subscriptionID",
			"location",
			"assessmentType",
		},
	)
	prometheus.MustRegister(m.prometheus.securitycenterCompliance)

	m.prometheus.advisorRecommendations = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_advisor_recommendation",
			Help: "Azure Audit Advisor recommendation",
		},
		[]string{
			"subscriptionID",
			"category",
			"resourceType",
			"resourceName",
			"resourceGroup",
			"problem",
			"impact",
			"risk",
		},
	)
	prometheus.MustRegister(m.prometheus.advisorRecommendations)
}

func (m *MetricsCollectorAzureRmSecurity) Reset() {
	m.prometheus.securitycenterCompliance.Reset()
	m.prometheus.advisorRecommendations.Reset()
}

func (m *MetricsCollectorAzureRmSecurity) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription subscriptions.Subscription, logger *log.Entry) {
		m.collectAzureAdvisorRecommendations(subscription, logger, callback)
		for _, location := range opts.Azure.Location {
			m.collectAzureSecurityCompliance(subscription, logger, callback, location)
		}

	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmSecurity) collectAzureSecurityCompliance(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func(), location string) {
	subscriptionResourceId := fmt.Sprintf("/subscriptions/%v", *subscription.SubscriptionID)
	client := security.NewCompliancesClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint, subscriptionResourceId, location)
	AzureClient.DecorateAzureAutorest(&client.Client)

	infoMetric := prometheusCommon.NewMetricsList()

	// try to find latest
	result, err := client.ListComplete(m.Context(), subscriptionResourceId)
	if err != nil {
		logger.Error(err)
		return
	}

	lastReportName := ""
	var lastReportTimestamp *time.Time
	for result.NotDone() {
		row := result.Value()

		if lastReportTimestamp == nil || row.AssessmentTimestampUtcDate.UTC().After(*lastReportTimestamp) {
			timestamp := row.AssessmentTimestampUtcDate.UTC()
			lastReportTimestamp = &timestamp
			lastReportName = to.String(row.Name)
		}

		if result.NextWithContext(m.Context()) != nil {
			break
		}
	}

	if lastReportName != "" {
		complienceResult, err := client.Get(m.Context(), subscriptionResourceId, lastReportName)
		if err != nil {
			logger.Error(err)
			return
		}

		if complienceResult.AssessmentResult != nil {
			for _, result := range *complienceResult.AssessmentResult {
				infoLabels := prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"location":       stringToStringLower(location),
					"assessmentType": stringPtrToStringLower(result.SegmentType),
				}
				infoMetric.Add(infoLabels, to.Float64(result.Percentage))
			}
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.securitycenterCompliance)
	}
}

func (m *MetricsCollectorAzureRmSecurity) collectAzureAdvisorRecommendations(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client := advisor.NewRecommendationsClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	AzureClient.DecorateAzureAutorest(&client.Client)

	recommendationResult, err := client.ListComplete(m.Context(), "", nil, "")
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewHashedMetricsList()

	for _, item := range *recommendationResult.Response().Value {
		resourceId := to.String(item.ID)
		azureResource, _ := azureCommon.ParseResourceId(resourceId)

		infoLabels := prometheus.Labels{
			"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
			"category":       stringToStringLower(string(item.RecommendationProperties.Category)),
			"resourceType":   stringPtrToStringLower(item.RecommendationProperties.ImpactedField),
			"resourceName":   stringPtrToStringLower(item.RecommendationProperties.ImpactedValue),
			"resourceGroup":  azureResource.ResourceGroup,
			"problem":        to.String(item.RecommendationProperties.ShortDescription.Problem),
			"impact":         stringToStringLower(string(item.Impact)),
			"risk":           stringToStringLower(string(item.Risk)),
		}

		infoMetric.Inc(infoLabels)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.advisorRecommendations)
	}
}
