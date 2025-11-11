package main

import (
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/advisor/armadvisor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
	"go.uber.org/zap"
)

type MetricsCollectorAzureRmAdvisor struct {
	collector.Processor

	prometheus struct {
		advisorRecommendation *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmAdvisor) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.advisorRecommendation = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_advisor_recommendation",
			Help: "Azure Advisor recommendation",
		},
		[]string{
			"recommendationID",
			"resourceID",
			"resourceType",
			"category",
			"impact",
			"risk",
			"recommendationSubCategory",
			"problem",
			"solution",
		},
	)
	m.Collector.RegisterMetricList("advisorRecommendation", m.prometheus.advisorRecommendation, true)
}

func (m *MetricsCollectorAzureRmAdvisor) Reset() {}

func (m *MetricsCollectorAzureRmAdvisor) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger) {
		m.collectAzureAdvisorRecommendations(subscription, logger)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmAdvisor) collectAzureAdvisorRecommendations(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger) {
	client, err := armadvisor.NewRecommendationsClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	// Generate recommendations first (async operation)
	// Note: This is a fire-and-forget operation. The generation is asynchronous,
	// so newly generated recommendations will be available in the next scrape cycle.
	_, err = client.Generate(m.Context(), nil)
	if err != nil {
		logger.Warnf("failed to generate recommendations for subscription %s: %v", to.StringLower(subscription.SubscriptionID), err)
	}

	recommendationMetrics := m.Collector.GetMetricList("advisorRecommendation")

	pager := client.NewListPager(nil)
	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Panic(err)
		}

		for _, recommendation := range result.Value {
			category := ""
			if recommendation.Properties.Category != nil {
				category = strings.ToLower(string(*recommendation.Properties.Category))
			}

			risk := ""
			if recommendation.Properties.Risk != nil {
				risk = strings.ToLower(string(*recommendation.Properties.Risk))
			}

			impact := ""
			if recommendation.Properties.Impact != nil {
				impact = strings.ToLower(string(*recommendation.Properties.Impact))
			}

			problem := ""
			solution := ""
			if recommendation.Properties.ShortDescription != nil {
				problem = to.String(recommendation.Properties.ShortDescription.Problem)
				solution = to.String(recommendation.Properties.ShortDescription.Solution)
			}

			// Truncate problem and solution if configured
			if Config.Collectors.Advisor.ProblemMaxLength > 0 {
				problem = truncateStrings(problem, Config.Collectors.Advisor.ProblemMaxLength, "...")
			}
			if Config.Collectors.Advisor.SolutionMaxLength > 0 {
				solution = truncateStrings(solution, Config.Collectors.Advisor.SolutionMaxLength, "...")
			}

			recommendationSubCategory := ""
			if recommendation.Properties.ExtendedProperties != nil {
				if subCat, ok := recommendation.Properties.ExtendedProperties["recommendationSubCategory"]; ok && subCat != nil {
					recommendationSubCategory = to.String(subCat)
				}
			}

			resourceID := ""
			if recommendation.Properties.ResourceMetadata != nil && recommendation.Properties.ResourceMetadata.ResourceID != nil {
				resourceID = to.String(recommendation.Properties.ResourceMetadata.ResourceID)
			}

			resourceType := to.StringLower(recommendation.Properties.ImpactedField)

			recommendationID := to.StringLower(recommendation.Name)

			infoLabels := prometheus.Labels{
				"recommendationID":          recommendationID,
				"resourceID":                resourceID,
				"resourceType":              resourceType,
				"category":                  category,
				"impact":                    impact,
				"risk":                      risk,
				"recommendationSubCategory": recommendationSubCategory,
				"problem":                   problem,
				"solution":                  solution,
			}
			recommendationMetrics.Add(infoLabels, 1)
		}
	}
}
