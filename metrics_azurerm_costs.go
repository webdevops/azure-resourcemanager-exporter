package main

import (
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/consumption/armconsumption"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type (
	MetricsCollectorAzureRmCosts struct {
		collector.Processor

		prometheus struct {
			consumptionBudgetInfo    *prometheus.GaugeVec
			consumptionBudgetLimit   *prometheus.GaugeVec
			consumptionBudgetCurrent *prometheus.GaugeVec
			consumptionBudgetUsage   *prometheus.GaugeVec

			costmanagementOverallUsage      *prometheus.GaugeVec
			costmanagementOverallActualCost *prometheus.GaugeVec

			costmanagementDetailUsage      *prometheus.GaugeVec
			costmanagementDetailActualCost *prometheus.GaugeVec
		}
	}
)

func (m *MetricsCollectorAzureRmCosts) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)
	m.Collector.SetCache(opts.GetCachePath("costs.json"))

	m.prometheus.consumptionBudgetInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_consumption_bugdet_info",
			Help: "Azure ResourceManager consumption budget info",
		},
		[]string{
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
			Name: "azurerm_consumption_bugdet_limit",
			Help: "Azure ResourceManager consumption budget limit",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"budgetName",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetLimit", m.prometheus.consumptionBudgetLimit, true)

	m.prometheus.consumptionBudgetUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_consumption_bugdet_usage",
			Help: "Azure ResourceManager consumption budget usage percentage",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"budgetName",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetUsage", m.prometheus.consumptionBudgetUsage, true)

	m.prometheus.consumptionBudgetCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_consumption_bugdet_current",
			Help: "Azure ResourceManager consumption budget current",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"budgetName",
			"unit",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetCurrent", m.prometheus.consumptionBudgetCurrent, true)

	m.prometheus.costmanagementOverallUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_costmanagement_overall_usage",
			Help: "Azure ResourceManager costmanagement overall usage",
		},
		[]string{
			"subscriptionID",
			"resourceGroup",
			"currency",
			"timeframe",
		},
	)
	m.Collector.RegisterMetricList("costmanagementOverallUsage", m.prometheus.costmanagementOverallUsage, true)

	m.prometheus.costmanagementOverallActualCost = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_costmanagement_overall_actualcost",
			Help: "Azure ResourceManager costmanagement overall actualcost",
		},
		[]string{
			"subscriptionID",
			"resourceGroup",
			"currency",
			"timeframe",
		},
	)
	m.Collector.RegisterMetricList("costmanagementOverallActualCost", m.prometheus.costmanagementOverallActualCost, true)

	m.prometheus.costmanagementDetailUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_costmanagement_detail_usage",
			Help: "Azure ResourceManager costmanagement detail usage report by dimensions",
		},
		[]string{
			"subscriptionID",
			"resourceGroup",
			"dimensionName",
			"dimensionValue",
			"currency",
			"timeframe",
		},
	)
	m.Collector.RegisterMetricList("costmanagementDetailUsage", m.prometheus.costmanagementDetailUsage, true)

	m.prometheus.costmanagementDetailActualCost = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_costmanagement_detail_actualcost",
			Help: "Azure ResourceManager costmanagement detail actualcost report by dimensions",
		},
		[]string{
			"subscriptionID",
			"resourceGroup",
			"dimensionName",
			"dimensionValue",
			"currency",
			"timeframe",
		},
	)
	m.Collector.RegisterMetricList("costmanagementDetailActualCost", m.prometheus.costmanagementDetailActualCost, true)
}

func (m *MetricsCollectorAzureRmCosts) Reset() {}

func (m *MetricsCollectorAzureRmCosts) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEach(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *log.Entry) {
		m.collectSubscription(subscription, logger)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmCosts) collectSubscription(subscription *armsubscriptions.Subscription, logger *log.Entry) {
	for _, timeframe := range opts.Costs.Timeframe {
		m.collectCostManagementMetrics(
			logger.WithField("costreport", "Usage"),
			m.Collector.GetMetricList("costmanagementOverallUsage"),
			subscription,
			armcostmanagement.ExportTypeUsage,
			nil,
			timeframe,
		)

		m.collectCostManagementMetrics(
			logger.WithField("costreport", "ActualCost"),
			m.Collector.GetMetricList("costmanagementOverallActualCost"),
			subscription,
			armcostmanagement.ExportTypeActualCost,
			nil,
			timeframe,
		)

		for _, val := range opts.Costs.Dimension {
			dimension := val

			m.collectCostManagementMetrics(
				logger.WithField("costreport", "Usage"),
				m.Collector.GetMetricList("costmanagementDetailUsage"),
				subscription,
				armcostmanagement.ExportTypeUsage,
				&dimension,
				timeframe,
			)

			m.collectCostManagementMetrics(
				logger.WithField("costreport", "ActualCost"),
				m.Collector.GetMetricList("costmanagementDetailActualCost"),
				subscription,
				armcostmanagement.ExportTypeActualCost,
				&dimension,
				timeframe,
			)
		}

		// avoid rate limit
		time.Sleep(opts.Costs.RequestDelay)
	}

	m.collectBugdetMetrics(
		logger.WithField("consumption", "Budgets"),
		subscription,
	)
}

func (m *MetricsCollectorAzureRmCosts) collectBugdetMetrics(logger *log.Entry, subscription *armsubscriptions.Subscription) {
	client, err := armconsumption.NewBudgetsClient(AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := m.Collector.GetMetricList("consumptionBudgetInfo")
	usageMetric := m.Collector.GetMetricList("consumptionBudgetUsage")
	limitMetric := m.Collector.GetMetricList("consumptionBudgetLimit")
	currentMetric := m.Collector.GetMetricList("consumptionBudgetCurrent")

	pager := client.NewListPager(*subscription.ID, nil)

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
				"resourceID":     stringToStringLower(resourceId),
				"subscriptionID": azureResource.Subscription,
				"resourceGroup":  azureResource.ResourceGroup,
				"budgetName":     to.String(budget.Name),
				"category":       stringToStringLower(string(*budget.Properties.Category)),
				"timeGrain":      string(*budget.Properties.TimeGrain),
			})

			if budget.Properties.Amount != nil {
				limitMetric.Add(prometheus.Labels{
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"budgetName":     to.String(budget.Name),
				}, *budget.Properties.Amount)
			}

			if budget.Properties.CurrentSpend != nil {
				currentMetric.Add(prometheus.Labels{
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"budgetName":     to.String(budget.Name),
					"unit":           to.StringLower(budget.Properties.CurrentSpend.Unit),
				}, *budget.Properties.CurrentSpend.Amount)
			}

			if budget.Properties.Amount != nil && budget.Properties.CurrentSpend != nil {
				usageMetric.Add(prometheus.Labels{
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"budgetName":     to.String(budget.Name),
				}, *budget.Properties.CurrentSpend.Amount / *budget.Properties.Amount)
			}
		}
	}
}

func (m *MetricsCollectorAzureRmCosts) collectCostManagementMetrics(logger *log.Entry, metricList *collector.MetricList, subscription *armsubscriptions.Subscription, exportType armcostmanagement.ExportType, dimension *string, timeframe string) {
	client, err := armcostmanagement.NewQueryClient(AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	dimensionType := armcostmanagement.QueryColumnTypeDimension
	queryGrouping := []*armcostmanagement.QueryGrouping{
		{
			Name: to.StringPtr("ResourceGroupName"),
			Type: &dimensionType,
		},
	}

	if dimension != nil {
		queryGrouping = append(
			queryGrouping,
			&armcostmanagement.QueryGrouping{
				Name: dimension,
				Type: &dimensionType,
			},
		)
	}

	granularity := armcostmanagement.GranularityType("none")
	timeframeType := armcostmanagement.TimeframeType(timeframe)

	aggregationFunction := armcostmanagement.FunctionTypeSum
	params := armcostmanagement.QueryDefinition{
		Dataset: &armcostmanagement.QueryDataset{
			Aggregation: map[string]*armcostmanagement.QueryAggregation{
				"PreTaxCost": {
					Name:     to.StringPtr("PreTaxCost"),
					Function: &aggregationFunction,
				},
			},
			Configuration: nil,
			Filter:        nil,
			Granularity:   &granularity,
			Grouping:      queryGrouping,
		},
		Timeframe:  &timeframeType,
		Type:       &exportType,
		TimePeriod: nil,
	}
	result, err := client.Usage(m.Context(), *subscription.ID, params, nil)
	if err != nil {
		logger.Panic(err)
	}

	if result.Properties == nil || result.Properties.Columns == nil || result.Properties.Rows == nil {
		// no result
		logger.Warnln("got invalid response (no columns or rows)")
		return
	}

	list := result.Properties

	columnNumberCost := -1
	columnNumberResourceGroupName := -1
	columnNumberDimension := -1
	columnNumberCurrency := -1

	dimensionColName := ""
	if dimension != nil {
		dimensionColName = *dimension
	}

	for num, col := range list.Columns {
		if col.Name == nil {
			continue
		}

		switch stringToStringLower(*col.Name) {
		case "pretaxcost":
			columnNumberCost = num
		case "resourcegroupname":
			columnNumberResourceGroupName = num
		case stringToStringLower(dimensionColName):
			columnNumberDimension = num
		case "currency":
			columnNumberCurrency = num
		}
	}

	if columnNumberCost == -1 || columnNumberResourceGroupName == -1 || columnNumberCurrency == -1 {
		logger.Warnln("unable to detect columns")
		return
	}

	if dimension != nil {
		if columnNumberDimension == -1 {
			logger.Warnln("unable to detect columns")
			return
		}
	}

	for _, row := range list.Rows {
		usage := float64(0)
		if v, ok := row[columnNumberCost].(float64); ok {
			usage = v
		}

		resourceGroup := row[columnNumberResourceGroupName].(string)

		labels := prometheus.Labels{
			"subscriptionID": to.StringLower(subscription.SubscriptionID),
			"resourceGroup":  to.StringLower(&resourceGroup),
			"currency":       stringToStringLower(row[columnNumberCurrency].(string)),
			"timeframe":      timeframe,
		}

		if dimension != nil {
			labels["dimensionName"] = *dimension
			labels["dimensionValue"] = row[columnNumberDimension].(string)
		}

		metricList.Add(labels, usage)
	}

	// avoid rate limit
	time.Sleep(opts.Costs.RequestDelay)
}
