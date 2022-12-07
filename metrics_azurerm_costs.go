package main

import (
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/consumption/armconsumption"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
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

	MetricsCollectorAzureRmCostsMetrics struct {
		Expiry                   *time.Time
		ConsumptionBudgetInfo    *prometheusCommon.MetricList
		ConsumptionBudgetLimit   *prometheusCommon.MetricList
		ConsumptionBudgetCurrent *prometheusCommon.MetricList
		ConsumptionBudgetUsage   *prometheusCommon.MetricList

		CostmanagementOverallUsage      *prometheusCommon.MetricList
		CostmanagementOverallActualCost *prometheusCommon.MetricList

		CostmanagementDetailUsage      *prometheusCommon.MetricList
		CostmanagementDetailActualCost *prometheusCommon.MetricList
	}
)

func (m *MetricsCollectorAzureRmCostsMetrics) Init() {
	if m.ConsumptionBudgetInfo == nil {
		m.ConsumptionBudgetInfo = prometheusCommon.NewMetricsList()
	}
	m.ConsumptionBudgetInfo.Init()

	if m.ConsumptionBudgetLimit == nil {
		m.ConsumptionBudgetLimit = prometheusCommon.NewMetricsList()
	}
	m.ConsumptionBudgetLimit.Init()

	if m.ConsumptionBudgetCurrent == nil {
		m.ConsumptionBudgetCurrent = prometheusCommon.NewMetricsList()
	}
	m.ConsumptionBudgetCurrent.Init()

	if m.ConsumptionBudgetUsage == nil {
		m.ConsumptionBudgetUsage = prometheusCommon.NewMetricsList()
	}
	m.ConsumptionBudgetUsage.Init()

	if m.CostmanagementOverallUsage == nil {
		m.CostmanagementOverallUsage = prometheusCommon.NewMetricsList()
	}
	m.CostmanagementOverallUsage.Init()

	if m.CostmanagementOverallActualCost == nil {
		m.CostmanagementOverallActualCost = prometheusCommon.NewMetricsList()
	}
	m.CostmanagementOverallActualCost.Init()

	if m.CostmanagementDetailUsage == nil {
		m.CostmanagementDetailUsage = prometheusCommon.NewMetricsList()
	}
	m.CostmanagementDetailUsage.Init()

	if m.CostmanagementDetailActualCost == nil {
		m.CostmanagementDetailActualCost = prometheusCommon.NewMetricsList()
	}
	m.CostmanagementDetailActualCost.Init()
}

func (m *MetricsCollectorAzureRmCosts) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

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
	prometheus.MustRegister(m.prometheus.consumptionBudgetInfo)

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
	prometheus.MustRegister(m.prometheus.consumptionBudgetLimit)

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
	prometheus.MustRegister(m.prometheus.consumptionBudgetUsage)

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
	prometheus.MustRegister(m.prometheus.consumptionBudgetCurrent)

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
	prometheus.MustRegister(m.prometheus.costmanagementOverallUsage)

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
	prometheus.MustRegister(m.prometheus.costmanagementOverallActualCost)

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
	prometheus.MustRegister(m.prometheus.costmanagementDetailUsage)

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
	prometheus.MustRegister(m.prometheus.costmanagementDetailActualCost)
}

func (m *MetricsCollectorAzureRmCosts) Reset() {
	m.prometheus.consumptionBudgetInfo.Reset()
	m.prometheus.consumptionBudgetLimit.Reset()
	m.prometheus.consumptionBudgetCurrent.Reset()
	m.prometheus.consumptionBudgetUsage.Reset()

	m.prometheus.costmanagementDetailUsage.Reset()
	m.prometheus.costmanagementDetailActualCost.Reset()
}

func (m *MetricsCollectorAzureRmCosts) Collect(callback chan<- func()) {
	cachePath := opts.GetCachePath("costs.json")

	metrics := &MetricsCollectorAzureRmCostsMetrics{}

	doUpdateRun := true
	if cachePath != nil {
		err := cacheRestoreFromPath(*cachePath, metrics)
		if err == nil {
			if metrics.Expiry != nil && metrics.Expiry.After(time.Now()) {
				// set next scrape run to expiry time
				sleepTime := time.Until(*metrics.Expiry) + 1*time.Minute
				m.Collector.SetNextSleepDuration(sleepTime)

				doUpdateRun = false
				m.Logger().Infof(`restored state from cache path "%s" (expiring %s)`, *cachePath, metrics.Expiry.UTC().String())
			} else {
				metrics = &MetricsCollectorAzureRmCostsMetrics{}
			}
		} else {
			m.Logger().Errorf("failed to load cache: %v", err)
		}
	}

	metrics.Init()

	if doUpdateRun {
		err := AzureSubscriptionsIterator.ForEach(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *log.Entry) {
			m.collectSubscription(subscription, logger, metrics)
		})
		if err != nil {
			m.Logger().Panic(err)
		}

		expiryTime := time.Now().Add(*opts.Scrape.TimeCosts)
		metrics.Expiry = &expiryTime
	}

	if cachePath != nil {
		err := cacheSaveToPath(*cachePath, metrics)
		if err != nil {
			m.Logger().Panic(err)
		}
	}

	callback <- func() {
		metrics.ConsumptionBudgetInfo.GaugeSet(m.prometheus.consumptionBudgetInfo)
		metrics.ConsumptionBudgetLimit.GaugeSet(m.prometheus.consumptionBudgetLimit)
		metrics.ConsumptionBudgetCurrent.GaugeSet(m.prometheus.consumptionBudgetCurrent)
		metrics.ConsumptionBudgetUsage.GaugeSet(m.prometheus.consumptionBudgetUsage)

		metrics.CostmanagementOverallUsage.GaugeSet(m.prometheus.costmanagementOverallUsage)
		metrics.CostmanagementOverallActualCost.GaugeSet(m.prometheus.costmanagementOverallActualCost)
		metrics.CostmanagementDetailUsage.GaugeSet(m.prometheus.costmanagementDetailUsage)
		metrics.CostmanagementDetailActualCost.GaugeSet(m.prometheus.costmanagementDetailActualCost)
	}
}

func (m *MetricsCollectorAzureRmCosts) collectSubscription(subscription *armsubscriptions.Subscription, logger *log.Entry, metrics *MetricsCollectorAzureRmCostsMetrics) {
	for _, timeframe := range opts.Costs.Timeframe {
		m.collectCostManagementMetrics(
			logger.WithField("costreport", "Usage"),
			metrics.CostmanagementOverallUsage,
			subscription,
			armcostmanagement.ExportTypeUsage,
			nil,
			timeframe,
		)

		m.collectCostManagementMetrics(
			logger.WithField("costreport", "ActualCost"),
			metrics.CostmanagementOverallActualCost,
			subscription,
			armcostmanagement.ExportTypeActualCost,
			nil,
			timeframe,
		)

		for _, val := range opts.Costs.Dimension {
			dimension := val

			m.collectCostManagementMetrics(
				logger.WithField("costreport", "Usage"),
				metrics.CostmanagementDetailUsage,
				subscription,
				armcostmanagement.ExportTypeUsage,
				&dimension,
				timeframe,
			)

			m.collectCostManagementMetrics(
				logger.WithField("costreport", "ActualCost"),
				metrics.CostmanagementDetailActualCost,
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
		metrics,
		subscription,
	)
}

func (m *MetricsCollectorAzureRmCosts) collectBugdetMetrics(logger *log.Entry, metrics *MetricsCollectorAzureRmCostsMetrics, subscription *armsubscriptions.Subscription) {
	client, err := armconsumption.NewBudgetsClient(AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

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

			metrics.ConsumptionBudgetInfo.AddInfo(prometheus.Labels{
				"resourceID":     stringToStringLower(resourceId),
				"subscriptionID": azureResource.Subscription,
				"resourceGroup":  azureResource.ResourceGroup,
				"budgetName":     to.String(budget.Name),
				"category":       stringToStringLower(string(*budget.Properties.Category)),
				"timeGrain":      string(*budget.Properties.TimeGrain),
			})

			if budget.Properties.Amount != nil {
				metrics.ConsumptionBudgetLimit.Add(prometheus.Labels{
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"budgetName":     to.String(budget.Name),
				}, *budget.Properties.Amount)
			}

			if budget.Properties.CurrentSpend != nil {
				metrics.ConsumptionBudgetCurrent.Add(prometheus.Labels{
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"budgetName":     to.String(budget.Name),
					"unit":           to.StringLower(budget.Properties.CurrentSpend.Unit),
				}, *budget.Properties.CurrentSpend.Amount)
			}

			if budget.Properties.Amount != nil && budget.Properties.CurrentSpend != nil {
				metrics.ConsumptionBudgetUsage.Add(prometheus.Labels{
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"budgetName":     to.String(budget.Name),
				}, *budget.Properties.CurrentSpend.Amount / *budget.Properties.Amount)
			}
		}
	}
}

func (m *MetricsCollectorAzureRmCosts) collectCostManagementMetrics(logger *log.Entry, metricList *prometheusCommon.MetricList, subscription *armsubscriptions.Subscription, exportType armcostmanagement.ExportType, dimension *string, timeframe string) {
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
