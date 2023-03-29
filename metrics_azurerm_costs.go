package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/consumption/armconsumption"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
	"go.uber.org/zap"

	metrics "github.com/webdevops/azure-resourcemanager-exporter/policy"
)

const (
	CostsQueryEnvVarPrefix = "COSTS_QUERY_"
)

type (
	MetricsCollectorAzureRmCosts struct {
		collector.Processor

		queries map[string]MetricsCollectorAzureRmCostsQuery

		resourceTagConfig      armclient.ResourceTagConfig
		resourceGroupTagConfig armclient.ResourceTagConfig

		prometheus struct {
			consumptionBudgetInfo    *prometheus.GaugeVec
			consumptionBudgetLimit   *prometheus.GaugeVec
			consumptionBudgetCurrent *prometheus.GaugeVec
			consumptionBudgetUsage   *prometheus.GaugeVec

			costmanagementOverallUsage      *prometheus.GaugeVec
			costmanagementOverallActualCost *prometheus.GaugeVec
		}
	}

	MetricsCollectorAzureRmCostsQuery struct {
		Name       string
		Dimensions []MetricsCollectorAzureRmCostsQueryDimension
		MetricName string
		MetricHelp string
	}
	MetricsCollectorAzureRmCostsQueryDimension struct {
		Dimension string
		Label     string
	}

	CostQueryConfigDimension struct {
		Name string
		Type armcostmanagement.QueryColumnType

		ResultColumnName   string
		ResultColumnNumber int
		LabelName          string
	}
)

func (m *MetricsCollectorAzureRmCosts) collectQueries() {
	m.queries = map[string]MetricsCollectorAzureRmCostsQuery{}

	addQuery := func(query MetricsCollectorAzureRmCostsQuery) {
		query.Name = strings.ToLower(strings.TrimSpace(query.Name))

		if _, exists := m.queries[query.Name]; exists {
			m.Logger().Fatalf(`found duplicate query config name "%v"`, query.Name)
		}
		m.queries[query.Name] = query
	}

	createCostQuery := func(name string, config string) MetricsCollectorAzureRmCostsQuery {
		dimensionList := strings.Split(config, ",")

		query := MetricsCollectorAzureRmCostsQuery{
			Name:       name,
			MetricName: fmt.Sprintf(`azurerm_costs_%v`, name),
			MetricHelp: fmt.Sprintf(`Azure ResourceManager costmanagement query with dimensions %v`, strings.Join(dimensionList, ",")),
			Dimensions: []MetricsCollectorAzureRmCostsQueryDimension{},
		}

		for _, dimension := range dimensionList {
			labelName := lowerFirst(prometheusLabelReplacerRegExp.ReplaceAllString(dimension, "_"))

			switch {
			case strings.EqualFold(dimension, "ResourceGroupName"):
				labelName = "resourceGroup"
			case strings.EqualFold(dimension, "ResourceId"):
				labelName = "resourceID"
			}

			query.Dimensions = append(
				query.Dimensions,
				MetricsCollectorAzureRmCostsQueryDimension{
					Dimension: dimension,
					Label:     labelName,
				},
			)
		}

		return query
	}

	for _, queryConfig := range opts.Costs.Queries {
		if !strings.Contains(queryConfig, "=") {
			m.Logger().Fatalf(`query config "%v" is not valid`, queryConfig)
		}

		queryConfigParts := strings.SplitN(queryConfig, "=", 2)
		query := createCostQuery(queryConfigParts[0], queryConfigParts[1])
		addQuery(query)
	}

	for _, val := range os.Environ() {
		envParts := strings.SplitN(val, "=", 2)
		envName := envParts[0]
		envVal := envParts[1]

		if strings.HasPrefix(envName, CostsQueryEnvVarPrefix) {
			query := createCostQuery(
				strings.TrimPrefix(envName, CostsQueryEnvVarPrefix),
				envVal,
			)
			addQuery(query)
		}
	}
}

func (m *MetricsCollectorAzureRmCosts) Setup(collector *collector.Collector) {
	var err error
	m.Processor.Setup(collector)

	m.resourceTagConfig, err = AzureClient.TagManager.ParseTagConfig(opts.Azure.ResourceTags)
	if err != nil {
		m.Logger().Panicf(`unable to parse resourceTag configuration "%s": %v"`, opts.Azure.ResourceTags, err.Error())
	}

	m.resourceGroupTagConfig, err = AzureClient.TagManager.ParseTagConfig(opts.Azure.ResourceGroupTags)
	if err != nil {
		m.Logger().Panicf(`unable to parse resourceGroupTag configuration "%s": %v"`, opts.Azure.ResourceGroupTags, err.Error())
	}

	m.collectQueries()

	// ----------------------------------------------------
	// Budget
	m.prometheus.consumptionBudgetInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_costs_budget_info",
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
			Name: "azurerm_costs_budget_limit",
			Help: "Azure ResourceManager consumption budget limit",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"resourceGroup",
			"budgetName",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetLimit", m.prometheus.consumptionBudgetLimit, true)

	m.prometheus.consumptionBudgetUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_costs_budget_usage",
			Help: "Azure ResourceManager consumption budget usage percentage",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"resourceGroup",
			"budgetName",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetUsage", m.prometheus.consumptionBudgetUsage, true)

	m.prometheus.consumptionBudgetCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_costs_budget_current",
			Help: "Azure ResourceManager consumption budget current",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"resourceGroup",
			"budgetName",
			"unit",
		},
	)
	m.Collector.RegisterMetricList("consumptionBudgetCurrent", m.prometheus.consumptionBudgetCurrent, true)

	// ----------------------------------------------------
	// Costs (by Query)

	for _, query := range m.queries {
		costLabels := []string{
			"subscriptionID",
			"currency",
			"timeframe",
		}

		for _, dimension := range query.Dimensions {
			switch dimension.Label {
			case "resourceGroup":
				// add additional resourceGroup labels
				costLabels = m.resourceGroupTagConfig.AddToPrometheusLabels(costLabels)
			case "resourceID":
				// add additional resourceGroup labels
				costLabels = m.resourceTagConfig.AddToPrometheusLabels(costLabels)
			}

			costLabels = append(costLabels, dimension.Label)
		}

		queryGaugeVec := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: query.MetricName,
				Help: query.MetricHelp,
			},
			costLabels,
		)
		m.Collector.RegisterMetricList(
			fmt.Sprintf(`query:%v`, query.Name),
			queryGaugeVec,
			true,
		)
	}
}

func (m *MetricsCollectorAzureRmCosts) Reset() {}

func (m *MetricsCollectorAzureRmCosts) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEach(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger) {
		m.collectSubscription(subscription, logger)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmCosts) collectSubscription(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger) {
	for _, timeframe := range opts.Costs.Timeframe {
		for _, query := range m.queries {
			logger.Infof(`fetching cost report for query "%v" and timeframe "%v"`, query.Name, timeframe)
			m.collectCostManagementMetrics(
				logger.With(
					zap.String("costQuery", query.Name),
				),
				m.Collector.GetMetricList(fmt.Sprintf(`query:%v`, query.Name)),
				subscription,
				armcostmanagement.ExportTypeActualCost,
				query,
				timeframe,
			)

			// avoid rate limit
			time.Sleep(opts.Costs.RequestDelay)
		}
	}

	logger.Info(`fetching cost budget report`)
	m.collectBugdetMetrics(
		logger.With(zap.String("consumption", "Budgets")),
		subscription,
	)
}

func (m *MetricsCollectorAzureRmCosts) collectBugdetMetrics(logger *zap.SugaredLogger, subscription *armsubscriptions.Subscription) {
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
					"resourceGroup":  azureResource.ResourceGroup,
					"budgetName":     to.String(budget.Name),
				}, *budget.Properties.Amount)
			}

			if budget.Properties.CurrentSpend != nil {
				currentMetric.Add(prometheus.Labels{
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"resourceGroup":  azureResource.ResourceGroup,
					"budgetName":     to.String(budget.Name),
					"unit":           to.StringLower(budget.Properties.CurrentSpend.Unit),
				}, *budget.Properties.CurrentSpend.Amount)
			}

			if budget.Properties.Amount != nil && budget.Properties.CurrentSpend != nil {
				usageMetric.Add(prometheus.Labels{
					"resourceID":     stringToStringLower(resourceId),
					"subscriptionID": azureResource.Subscription,
					"resourceGroup":  azureResource.ResourceGroup,
					"budgetName":     to.String(budget.Name),
				}, *budget.Properties.CurrentSpend.Amount / *budget.Properties.Amount)
			}
		}
	}
}

func (m *MetricsCollectorAzureRmCosts) collectCostManagementMetrics(logger *zap.SugaredLogger, metricList *collector.MetricList, subscription *armsubscriptions.Subscription, exportType armcostmanagement.ExportType, query MetricsCollectorAzureRmCostsQuery, timeframe string) {
	clientOpts := AzureClient.NewArmClientOptions()
	clientOpts.Retry = policy.RetryOptions{
		MaxRetries:    1,
		RetryDelay:    1 * time.Minute,
		MaxRetryDelay: 5 * time.Minute,
	}
	clientOpts.PerCallPolicies = append(clientOpts.PerCallPolicies, metrics.CostRateLimitPolicy{Logger: logger})
	client, err := armcostmanagement.NewQueryClient(AzureClient.GetCred(), clientOpts)
	if err != nil {
		logger.Panic(err)
	}

	dimensionList := make([]*CostQueryConfigDimension, len(query.Dimensions))

	for i, dimension := range query.Dimensions {
		dimensionConfig := CostQueryConfigDimension{
			Name:               dimension.Dimension,
			Type:               armcostmanagement.QueryColumnTypeDimension,
			ResultColumnName:   dimension.Dimension,
			ResultColumnNumber: -1,
			LabelName:          dimension.Label,
		}

		if strings.Contains(dimension.Dimension, ":") {
			dimensionParts := strings.SplitN(dimension.Dimension, ":", 2)
			switch strings.ToLower(dimensionParts[0]) {
			case "tag":
				dimensionConfig.Type = "TagKey"
				dimensionConfig.Name = dimensionParts[1]
				dimensionConfig.ResultColumnName = "TagValue"
			default:
				logger.Fatalf(`cost dimension %v is not supported`, dimension)
			}
		}

		dimensionList[i] = &dimensionConfig
	}

	queryGrouping := make([]*armcostmanagement.QueryGrouping, len(dimensionList))
	for i, dimensionConfig := range dimensionList {
		queryGrouping[i] = &armcostmanagement.QueryGrouping{
			Name: &dimensionConfig.Name,
			Type: &dimensionConfig.Type,
		}
	}

	granularity := armcostmanagement.GranularityType("none")
	timeframeType := armcostmanagement.TimeframeType(timeframe)

	aggregationFunction := armcostmanagement.FunctionTypeSum
	params := armcostmanagement.QueryDefinition{
		Dataset: &armcostmanagement.QueryDataset{
			Aggregation: map[string]*armcostmanagement.QueryAggregation{
				"Cost": {
					Name:     to.StringPtr("Cost"),
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

	// detect column numbers
	columnNumberCost := -1
	columnNumberCurrency := -1

	for num, col := range list.Columns {
		if col.Name == nil {
			continue
		}

		switch stringToStringLower(*col.Name) {
		case "cost":
			columnNumberCost = num
		case "currency":
			columnNumberCurrency = num
		}

		for _, dimensionConfig := range dimensionList {
			if strings.EqualFold(dimensionConfig.ResultColumnName, *col.Name) {
				dimensionConfig.ResultColumnNumber = num
			}
		}
	}

	// check if we detected all columns
	if columnNumberCost == -1 || columnNumberCurrency == -1 {
		logger.Warnln("unable to detect columns")
		return
	}

	for _, dimensionConfig := range dimensionList {
		if dimensionConfig.ResultColumnNumber == -1 {
			logger.Warnf(`unable to detect column "%s"`, dimensionConfig.Name)
			return
		}
	}

	// process metrics
	for _, row := range list.Rows {
		usage := float64(0)
		if v, ok := row[columnNumberCost].(float64); ok {
			usage = v
		}

		labels := prometheus.Labels{
			"subscriptionID": to.StringLower(subscription.SubscriptionID),
			"currency":       stringToStringLower(row[columnNumberCurrency].(string)),
			"timeframe":      timeframe,
		}

		for _, dimensionConfig := range dimensionList {
			labels[dimensionConfig.LabelName] = ""
			if row[dimensionConfig.ResultColumnNumber] != nil {
				labels[dimensionConfig.LabelName] = row[dimensionConfig.ResultColumnNumber].(string)

				switch dimensionConfig.LabelName {
				case "subscriptionName":
					labels[dimensionConfig.LabelName] = to.String(subscription.DisplayName)
				case "resourceGroup":
					// add resourceGroups labels using tag manager
					resourceId := fmt.Sprintf(
						"/subscriptions/%s/resourceGroups/%s",
						to.StringLower(subscription.SubscriptionID),
						row[dimensionConfig.ResultColumnNumber].(string),
					)
					labels = AzureClient.TagManager.AddResourceTagsToPrometheusLabels(m.Context(), labels, resourceId, m.resourceGroupTagConfig)
				case "resourceID":
					// add resource labels using tag manager
					labels = AzureClient.TagManager.AddResourceTagsToPrometheusLabels(m.Context(), labels, row[dimensionConfig.ResultColumnNumber].(string), m.resourceTagConfig)
				}
			}
		}

		metricList.Add(labels, usage)
	}

	// avoid rate limit
	time.Sleep(opts.Costs.RequestDelay)
}
