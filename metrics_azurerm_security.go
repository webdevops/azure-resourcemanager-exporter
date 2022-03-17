package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/advisor/mgmt/advisor"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/preview/preview/security/mgmt/security"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
	prometheusAzure "github.com/webdevops/go-prometheus-common/azure"
)

type MetricsCollectorAzureRmSecurity struct {
	CollectorProcessorGeneral

	prometheus struct {
		securitycenterCompliance *prometheus.GaugeVec
		advisorRecommendations   *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmSecurity) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

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

func (m *MetricsCollectorAzureRmSecurity) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureAdvisorRecommendations(ctx, logger, callback, subscription)
	for _, location := range m.CollectorReference.AzureLocations {
		m.collectAzureSecurityCompliance(ctx, logger, callback, subscription, location)
	}
}

func (m *MetricsCollectorAzureRmSecurity) collectAzureSecurityCompliance(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription, location string) {
	subscriptionResourceId := fmt.Sprintf("/subscriptions/%v", *subscription.SubscriptionID)
	client := security.NewCompliancesClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, subscriptionResourceId, location)
	decorateAzureAutorest(&client.Client)

	infoMetric := prometheusCommon.NewMetricsList()

	// try to find latest
	result, err := client.ListComplete(ctx, subscriptionResourceId)
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

		if result.NextWithContext(ctx) != nil {
			break
		}
	}

	if lastReportName != "" {
		complienceResult, err := client.Get(ctx, subscriptionResourceId, lastReportName)
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

func (m *MetricsCollectorAzureRmSecurity) collectAzureAdvisorRecommendations(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := advisor.NewRecommendationsClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	decorateAzureAutorest(&client.Client)

	recommendationResult, err := client.ListComplete(ctx, "", nil, "")
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewHashedMetricsList()

	for _, item := range *recommendationResult.Response().Value {
		resourceId := to.String(item.ID)
		azureResource, _ := prometheusAzure.ParseResourceId(resourceId)

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
