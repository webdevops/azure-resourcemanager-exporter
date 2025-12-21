package main

import (
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/advisor/armadvisor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/security/armsecurity"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type MetricsCollectorAzureRmDefender struct {
	collector.Processor

	prometheus struct {
		defenderSecureScorePercentage *prometheus.GaugeVec
		defenderSecureScoreMax        *prometheus.GaugeVec
		defenderSecureScoreCurrent    *prometheus.GaugeVec

		defenderComplianceScore         *prometheus.GaugeVec
		defenderComplianceResourceCount *prometheus.GaugeVec
		defenderAdvisorRecommendations  *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmDefender) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.defenderSecureScorePercentage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_defender_secure_score_percentage",
			Help: "Azure Defender secure score in percent",
		},
		[]string{
			"subscriptionID",
			"secureScoreName",
		},
	)
	m.Collector.RegisterMetricList("defenderSecureScorePercentage", m.prometheus.defenderSecureScorePercentage, true)

	m.prometheus.defenderSecureScoreMax = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_defender_secure_score_max",
			Help: "Azure Defender maximum secure score which can be achieved",
		},
		[]string{
			"subscriptionID",
			"secureScoreName",
		},
	)
	m.Collector.RegisterMetricList("defenderSecureScoreMax", m.prometheus.defenderSecureScoreMax, true)

	m.prometheus.defenderSecureScoreCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_defender_secure_score_current",
			Help: "Azure Defender current secure score",
		},
		[]string{
			"subscriptionID",
			"secureScoreName",
		},
	)
	m.Collector.RegisterMetricList("defenderSecureScoreCurrent", m.prometheus.defenderSecureScoreCurrent, true)

	m.prometheus.defenderComplianceScore = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_defender_compliance_score",
			Help: "Azure Defender compliance score",
		},
		[]string{
			"subscriptionID",
			"assessmentType",
		},
	)
	m.Collector.RegisterMetricList("defenderComplianceScore", m.prometheus.defenderComplianceScore, true)

	m.prometheus.defenderComplianceResourceCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_defender_compliance_resources",
			Help: "Azure Defender count of compliance resource in assessment",
		},
		[]string{
			"subscriptionID",
		},
	)
	m.Collector.RegisterMetricList("defenderComplianceResourceCount", m.prometheus.defenderComplianceResourceCount, true)

	m.prometheus.defenderAdvisorRecommendations = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_defender_advisor_recommendation",
			Help: "Azure Advisor recommendation",
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
	m.Collector.RegisterMetricList("defenderAdvisorRecommendations", m.prometheus.defenderAdvisorRecommendations, true)
}

func (m *MetricsCollectorAzureRmDefender) Reset() {}

func (m *MetricsCollectorAzureRmDefender) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *slog.Logger) {
		m.collectAzureSecureScore(subscription, logger, callback)
		m.collectAzureSecurityCompliance(subscription, logger, callback)
		m.collectAzureAdvisorRecommendations(subscription, logger, callback)
	})
	if err != nil {
		panic(err)
	}
}

func (m *MetricsCollectorAzureRmDefender) collectAzureSecureScore(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	client, err := armsecurity.NewSecureScoresClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	secureScorePercentageMetrics := m.Collector.GetMetricList("defenderSecureScorePercentage")
	secureScoreMaxMetrics := m.Collector.GetMetricList("defenderSecureScoreMax")
	secureScoreCurrentMetrics := m.Collector.GetMetricList("defenderSecureScoreCurrent")

	pager := client.NewListPager(nil)
	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			panic(err)
		}

		for _, secureScore := range result.Value {
			infoLabels := prometheus.Labels{
				"subscriptionID":  to.StringLower(subscription.SubscriptionID),
				"secureScoreName": to.StringLower(secureScore.Name),
			}
			secureScorePercentageMetrics.Add(infoLabels, to.Float64(secureScore.Properties.Score.Percentage))
			secureScoreMaxMetrics.Add(infoLabels, float64(to.Number(secureScore.Properties.Score.Max)))
			secureScoreCurrentMetrics.Add(infoLabels, to.Float64(secureScore.Properties.Score.Current))
		}
	}
}

func (m *MetricsCollectorAzureRmDefender) collectAzureSecurityCompliance(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	client, err := armsecurity.NewCompliancesClient(AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	complianceMetric := m.Collector.GetMetricList("defenderComplianceScore")
	resourceCountMetric := m.Collector.GetMetricList("defenderComplianceResourceCount")

	pager := client.NewListPager(*subscription.ID, nil)

	lastReportName := ""
	var lastReportTimestamp *time.Time
	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			panic(err)
		}

		if result.Value == nil {
			continue
		}

		for _, complianceReport := range result.Value {
			if lastReportTimestamp == nil || complianceReport.Properties.AssessmentTimestampUTCDate.UTC().After(*lastReportTimestamp) {
				timestamp := complianceReport.Properties.AssessmentTimestampUTCDate.UTC()
				lastReportTimestamp = &timestamp
				lastReportName = to.String(complianceReport.Name)
			}
		}
	}

	if lastReportName != "" {
		report, err := client.Get(m.Context(), *subscription.ID, lastReportName, nil)
		if err != nil {
			logger.Error(err.Error())
			return
		}

		if report.Properties.AssessmentResult != nil {
			for _, result := range report.Properties.AssessmentResult {
				infoLabels := prometheus.Labels{
					"subscriptionID": to.StringLower(subscription.SubscriptionID),
					"assessmentType": to.StringLower(result.SegmentType),
				}
				complianceMetric.Add(infoLabels, to.Float64(result.Percentage))
			}

			resourceCountMetric.Add(prometheus.Labels{
				"subscriptionID": to.StringLower(subscription.SubscriptionID),
			}, float64(to.Number(report.Properties.ResourceCount)))
		}
	}
}

func (m *MetricsCollectorAzureRmDefender) collectAzureAdvisorRecommendations(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	client, err := armadvisor.NewRecommendationsClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	recommendationMetrics := m.Collector.GetMetricList("defenderAdvisorRecommendations")

	pager := client.NewListPager(nil)
	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			panic(err)
		}

		for _, recommendation := range result.Value {
			resourceId := to.String(recommendation.ID)
			azureResource, _ := armclient.ParseResourceId(resourceId)

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

			infoLabels := prometheus.Labels{
				"subscriptionID": to.StringLower(subscription.SubscriptionID),
				"category":       category,
				"resourceType":   to.StringLower(recommendation.Properties.ImpactedField),
				"resourceName":   to.StringLower(recommendation.Properties.ImpactedValue),
				"resourceGroup":  azureResource.ResourceGroup,
				"problem":        to.String(recommendation.Properties.ShortDescription.Problem),
				"impact":         impact,
				"risk":           risk,
			}
			recommendationMetrics.Add(infoLabels, 1)
		}
	}
}
