package main

import (
	"fmt"
	"time"
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/advisor/mgmt/advisor"
	"github.com/Azure/azure-sdk-for-go/profiles/preview/preview/security/mgmt/security"
	"github.com/prometheus/client_golang/prometheus"
)

func (m *MetricCollectorAzureRm) collectAzureSecurityCompliance(ctx context.Context, subscriptionId, location string, callback chan<- func()) {
	subscriptionResourceId := fmt.Sprintf("/subscriptions/%v", subscriptionId)
	client := security.NewCompliancesClient(subscriptionResourceId, location)
	client.Authorizer = AzureAuthorizer

	complienceResult, err := client.Get(ctx, subscriptionResourceId, time.Now().Format("2006-01-02Z"))
	if err != nil {
		ErrorLogger.Error(fmt.Sprintf("subscription[%v]", subscriptionId), err)
		return
	}

	infoMetric := prometheusMetricsList{}

	if complienceResult.AssessmentResult != nil {
		for _, result := range *complienceResult.AssessmentResult {
			segmentType := ""
			if result.SegmentType != nil {
				segmentType = *result.SegmentType
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"assessmentType": segmentType,
			}
			infoMetric.Add(infoLabels, *result.Percentage)
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.securitycenterCompliance)
	}
}

func (m *MetricCollectorAzureRm) collectAzureAdvisorRecommendations(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := advisor.NewRecommendationsClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	recommendationResult, err := client.ListComplete(ctx, "", nil, "")
	if err != nil {
		panic(err)
	}

	infoMetric := prometheusMetricsList{}

	for _, item := range *recommendationResult.Response().Value {

		infoLabels := prometheus.Labels{
			"subscriptionID": subscriptionId,
			"category":       string(item.RecommendationProperties.Category),
			"resourceType":   *item.RecommendationProperties.ImpactedField,
			"resourceName":   *item.RecommendationProperties.ImpactedValue,
			"resourceGroup":  extractResourceGroupFromAzureId(*item.ID),
			"impact":         string(item.Impact),
			"risk":           string(item.Risk),
		}

		infoMetric.Add(infoLabels, 1)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.advisorRecommendations)
	}
}
