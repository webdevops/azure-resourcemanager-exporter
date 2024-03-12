package main

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/consumption/armconsumption"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
	"go.uber.org/zap"
)

// Define MetricsCollectorAzureRmBudgets struct
type MetricsCollectorAzureRmBudgets struct {
	collector.Processor

	prometheus struct {
		consumptionBudgetInfo     *prometheus.GaugeVec
		consumptionBudgetLimit    *prometheus.GaugeVec
		consumptionBudgetCurrent  *prometheus.GaugeVec
		consumptionBudgetForecast *prometheus.GaugeVec
		consumptionBudgetUsage    *prometheus.GaugeVec
	}
}

// Setup method to initialize Prometheus metrics
func (m *MetricsCollectorAzureRmBudgets) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	// ----------------------------------------------------
	// Budget
	m.prometheus.consumptionBudgetInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_budgets_info",
			Help: "Azure ResourceManager consumption budget info",
		},
		[]string{
			"scope",
			"resourceID",
			"subscriptionID",
			"budgetName",
			"resourceGroup",
			"category",
			"timeGrain",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetInfo", m.prometheus.consumptionBudgetInfo, true)

	m.prometheus.consumptionBudgetLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_budgets_limit",
			Help: "Azure ResourceManager consumption budget limit",
		},
		[]string{
			"scope",
			"resourceID",
			"subscriptionID",
			"resourceGroup",
			"budgetName",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetLimit", m.prometheus.consumptionBudgetLimit, true)

	m.prometheus.consumptionBudgetUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_budgets_usage",
			Help: "Azure ResourceManager consumption budget usage percentage",
		},
		[]string{
			"scope",
			"resourceID",
			"subscriptionID",
			"resourceGroup",
			"budgetName",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetUsage", m.prometheus.consumptionBudgetUsage, true)

	m.prometheus.consumptionBudgetCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_budgets_current",
			Help: "Azure ResourceManager consumption budget current",
		},
		[]string{
			"scope",
			"resourceID",
			"subscriptionID",
			"resourceGroup",
			"budgetName",
			"unit",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetCurrent", m.prometheus.consumptionBudgetCurrent, true)

	m.prometheus.consumptionBudgetForecast = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_budgets_forecast",
			Help: "Azure ResourceManager consumption budget forecast",
		},
		[]string{
			"scope",
			"resourceID",
			"subscriptionID",
			"resourceGroup",
			"budgetName",
			"unit",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetForecast", m.prometheus.consumptionBudgetForecast, true)
}

func (m *MetricsCollectorAzureRmBudgets) Reset() {}

func (m *MetricsCollectorAzureRmBudgets) Collect(callback chan<- func()) {
	if Config.Collectors.Budgets.Scopes != nil && len(Config.Collectors.Budgets.Scopes) > 0 {
		for _, scope := range Config.Collectors.Budgets.Scopes {
			// Run the budget query for the current scope
			m.collectBudgetMetrics(logger, scope, callback)
		}
	} else {
		// using subscription iterator
		iterator := AzureSubscriptionsIterator

		err := iterator.ForEach(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger) {
			m.collectBudgetMetrics(
				logger,
				*subscription.ID,
				callback,
			)
		})
		if err != nil {
			m.Logger().Panic(err)
		}
	}
}

func (m *MetricsCollectorAzureRmBudgets) collectBudgetMetrics(logger *zap.SugaredLogger, scope string, callback chan<- func()) {
	clientFactory, err := armconsumption.NewClientFactory("<subscription-id>", AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := m.Collector.GetMetricList("consumptionBudgetInfo")
	usageMetric := m.Collector.GetMetricList("consumptionBudgetUsage")
	limitMetric := m.Collector.GetMetricList("consumptionBudgetLimit")
	currentMetric := m.Collector.GetMetricList("consumptionBudgetCurrent")
	forecastMetric := m.Collector.GetMetricList("consumptionBudgetForecast")

	pager := clientFactory.NewBudgetsClient().NewListPager(scope, nil)

	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Panic(err)
		}

		if result.Value == nil {
			continue
		}

		for _, budget := range result.Value {
			resourceId := to.String(budget.ID)

			azureResource, _ := armclient.ParseResourceId(resourceId)

			infoMetric.AddInfo(prometheus.Labels{
				"scope":          scope,
				"resourceID":     stringToStringLower(resourceId),
				"subscriptionID": azureResource.Subscription,
				"resourceGroup":  azureResource.ResourceGroup,
				"budgetName":     to.String(budget.Name),
				"category":       stringToStringLower(string(*budget.Properties.Category)),
				"timeGrain":      string(*budget.Properties.TimeGrain),
			})

			if budget.Properties.Amount != nil {
				limitMetric.Add(prometheus.Labels{
					"scope":          scope,
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"resourceGroup":  azureResource.ResourceGroup,
					"budgetName":     to.String(budget.Name),
				}, *budget.Properties.Amount)
			}

			if budget.Properties.CurrentSpend != nil {
				currentMetric.Add(prometheus.Labels{
					"scope":          scope,
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"resourceGroup":  azureResource.ResourceGroup,
					"budgetName":     to.String(budget.Name),
					"unit":           to.StringLower(budget.Properties.CurrentSpend.Unit),
				}, *budget.Properties.CurrentSpend.Amount)
			}

			if budget.Properties.ForecastSpend != nil {
				forecastMetric.Add(prometheus.Labels{
					"scope":          scope,
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"resourceGroup":  azureResource.ResourceGroup,
					"budgetName":     to.String(budget.Name),
					"unit":           to.StringLower(budget.Properties.ForecastSpend.Unit),
				}, *budget.Properties.ForecastSpend.Amount)
			}

			if budget.Properties.Amount != nil && budget.Properties.CurrentSpend != nil {
				usageMetric.Add(prometheus.Labels{
					"scope":          scope,
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"resourceGroup":  azureResource.ResourceGroup,
					"budgetName":     to.String(budget.Name),
				}, *budget.Properties.CurrentSpend.Amount / *budget.Properties.Amount)
			}
		}
	}
}
