package main

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/consumption/mgmt/consumption"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/services/costmanagement/mgmt/2019-10-01/costmanagement"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
	"strings"
)

type MetricsCollectorAzureRmCosts struct {
	CollectorProcessorGeneral

	prometheus struct {
		consumptionBudgetInfo    *prometheus.GaugeVec
		consumptionBudgetLimit   *prometheus.GaugeVec
		consumptionBudgetCurrent *prometheus.GaugeVec
		consumptionBudgetUsage   *prometheus.GaugeVec

		costmanagementUsage      *prometheus.GaugeVec
		costmanagementActualCost *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmCosts) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.consumptionBudgetInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_consumtion_bugdet_info",
			Help: "Azure ResourceManager consumtion budget info",
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
			Name: "azurerm_consumtion_bugdet_limit",
			Help: "Azure ResourceManager consumtion budget limit",
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
			Name: "azurerm_consumtion_bugdet_usage",
			Help: "Azure ResourceManager consumtion budget usage percentage",
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
			Name: "azurerm_consumtion_bugdet_current",
			Help: "Azure ResourceManager consumtion budget current",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"budgetName",
			"unit",
		},
	)
	prometheus.MustRegister(m.prometheus.consumptionBudgetCurrent)

	m.prometheus.costmanagementUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_costmanagement_usage",
			Help: "Azure ResourceManager costmanagement usage",
		},
		[]string{
			"subscriptionID",
			"resourceGroup",
			"currency",
			"timeframe",
		},
	)
	prometheus.MustRegister(m.prometheus.costmanagementUsage)

	m.prometheus.costmanagementActualCost = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_costmanagement_actualcost",
			Help: "Azure ResourceManager costmanagement actualcost",
		},
		[]string{
			"subscriptionID",
			"resourceGroup",
			"currency",
			"timeframe",
		},
	)
	prometheus.MustRegister(m.prometheus.costmanagementActualCost)
}

func (m *MetricsCollectorAzureRmCosts) Reset() {
	m.prometheus.consumptionBudgetInfo.Reset()
	m.prometheus.consumptionBudgetLimit.Reset()
	m.prometheus.consumptionBudgetCurrent.Reset()

	m.prometheus.costmanagementUsage.Reset()
	m.prometheus.costmanagementActualCost.Reset()
}

func (m *MetricsCollectorAzureRmCosts) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	for _, timeframe := range opts.Costs.Timeframe {
		m.collectCostManagementMetrics(
			ctx,
			logger.WithField("costreport", "Usage"),
			callback,
			subscription,
			"Usage",
			timeframe,
			m.prometheus.costmanagementUsage,
		)

		m.collectCostManagementMetrics(
			ctx,
			logger.WithField("costreport", "ActualCost"),
			callback,
			subscription,
			"ActualCost",
			timeframe,
			m.prometheus.costmanagementActualCost,
		)
	}

	m.collectBugdetMetrics(
		ctx,
		logger.WithField("consumtion", "Budgets"),
		callback,
		subscription,
	)

}

func (m *MetricsCollectorAzureRmCosts) collectBugdetMetrics(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := consumption.NewBudgetsClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	scope := fmt.Sprintf("/subscriptions/%s/", *subscription.SubscriptionID)

	result, err := client.ListComplete(ctx, scope)
	if err != nil {
		log.Error(err.Error())
		return
	}

	infoMetric := prometheusCommon.NewMetricsList()
	limitMetric := prometheusCommon.NewMetricsList()
	currentMetric := prometheusCommon.NewMetricsList()
	usageMetric := prometheusCommon.NewMetricsList()

	for result.NotDone() {
		val := result.Value()

		infoMetric.AddInfo(prometheus.Labels{
			"resourceID":     toResourceId(val.ID),
			"subscriptionID": to.String(subscription.SubscriptionID),
			"budgetName":     to.String(val.Name),
			"resourceGroup":  extractResourceGroupFromAzureId(to.String(val.ID)),
			"category":       to.String(val.BudgetProperties.Category),
			"timeGrain":      string(val.BudgetProperties.TimeGrain),
		})

		if val.BudgetProperties.Amount != nil {
			limitAmount, _ := val.BudgetProperties.Amount.Float64()
			limitMetric.Add(prometheus.Labels{
				"resourceID":     toResourceId(val.ID),
				"subscriptionID": to.String(subscription.SubscriptionID),
				"budgetName":     to.String(val.Name),
			}, limitAmount)
		}

		if val.BudgetProperties.CurrentSpend != nil {
			budgetCurrentSpend, _ := val.BudgetProperties.CurrentSpend.Amount.Float64()
			currentMetric.Add(prometheus.Labels{
				"resourceID":     toResourceId(val.ID),
				"subscriptionID": to.String(subscription.SubscriptionID),
				"budgetName":     to.String(val.Name),
				"unit":           to.String(val.BudgetProperties.CurrentSpend.Unit),
			}, budgetCurrentSpend)
		}

		if val.BudgetProperties.Amount != nil && val.BudgetProperties.CurrentSpend != nil {
			budgetCurrentSpend, _ := val.BudgetProperties.CurrentSpend.Amount.Float64()
			limitAmount, _ := val.BudgetProperties.Amount.Float64()
			usageMetric.Add(prometheus.Labels{
				"resourceID":     toResourceId(val.ID),
				"subscriptionID": to.String(subscription.SubscriptionID),
				"budgetName":     to.String(val.Name),
			}, budgetCurrentSpend/limitAmount)
		}

		if result.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.consumptionBudgetInfo)
		limitMetric.GaugeSet(m.prometheus.consumptionBudgetLimit)
		currentMetric.GaugeSet(m.prometheus.consumptionBudgetCurrent)
		usageMetric.GaugeSet(m.prometheus.consumptionBudgetUsage)
	}
}

func (m *MetricsCollectorAzureRmCosts) collectCostManagementMetrics(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription, costType, timeframe string, metric *prometheus.GaugeVec) {
	client := costmanagement.NewQueryClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	scope := fmt.Sprintf("/subscriptions/%s/", *subscription.SubscriptionID)

	params := costmanagement.QueryDefinition{}
	params.Type = to.StringPtr(costType)
	params.Timeframe = costmanagement.TimeframeType(timeframe)
	params.Dataset = &costmanagement.QueryDataset{}
	params.Dataset.Grouping = &[]costmanagement.QueryGrouping{
		{
			Name: to.StringPtr("ResourceGroupName"),
			Type: "Dimension",
		},
		{
			Name: to.StringPtr("ChargeType"),
			Type: "Dimension",
		},
	}
	params.Dataset.Granularity = "None"
	params.Dataset.Aggregation = map[string]*costmanagement.QueryAggregation{}

	params.Dataset.Aggregation["PreTaxCost"] = &costmanagement.QueryAggregation{
		Name:     to.StringPtr("PreTaxCost"),
		Function: to.StringPtr("Sum"),
	}
	params.Dataset.Sorting = &[]costmanagement.QuerySortingConfiguration{
		{Name: to.StringPtr("BillingMonth"), QuerySortingDirection: "ascending"},
	}

	list, err := client.Usage(ctx, scope, params)
	if err != nil {
		logger.Error(err)
		return
	}

	if list.Columns == nil || list.Rows == nil {
		// no result
		logger.Warnln("got invalid response (no columns or rows)")
		return
	}

	columnNumberCost := -1
	columnNumberResourceGroup := -1
	columnNumberCurrency := -1

	for num, col := range *list.Columns {
		if col.Name == nil {
			continue
		}

		switch strings.ToLower(*col.Name) {
		case "pretaxcost":
			columnNumberCost = num
		case "resourcegroupname":
			columnNumberResourceGroup = num
		case "currency":
			columnNumberCurrency = num
		}
	}

	if columnNumberCost == -1 || columnNumberResourceGroup == -1 || columnNumberCurrency == -1 {
		logger.Warnln("unable to detect columns")
		return
	}

	costMetric := prometheusCommon.NewMetricsList()
	for _, row := range *list.Rows {
		usage := float64(0)
		if v, ok := row[columnNumberCost].(float64); ok {
			usage = v
		}

		costMetric.Add(prometheus.Labels{
			"subscriptionID": to.String(subscription.SubscriptionID),
			"resourceGroup":  row[columnNumberResourceGroup].(string),
			"currency":       row[columnNumberCurrency].(string),
			"timeframe":      timeframe,
		}, usage)
	}

	callback <- func() {
		costMetric.GaugeSet(metric)
	}
}
