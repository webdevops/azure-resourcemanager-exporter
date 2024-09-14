package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	armruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
	"go.uber.org/zap"

	"github.com/webdevops/azure-resourcemanager-exporter/config"
	metrics "github.com/webdevops/azure-resourcemanager-exporter/policy"
)

const (
	CostsQueryEnvVarPrefix = "COSTS_QUERY_"
)

type (
	MetricsCollectorAzureRmCosts struct {
		collector.Processor
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

func (m *MetricsCollectorAzureRmCosts) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	// ----------------------------------------------------
	// Costs (by Query)

	for _, query := range Config.Collectors.Costs.Queries {
		queryConfig := query.GetConfig()

		costLabels := []string{
			"scope",
			"subscriptionID",
			"currency",
			"timeframe",
			"granularity",
		}

		// add dimension labels
		for _, dimension := range queryConfig.Dimensions {
			switch dimension.Label {
			case "resourceGroup":
				// add additional resourceGroup labels
				costLabels = AzureResourceGroupTagManager.AddToPrometheusLabels(costLabels)
			case "resourceID":
				// add additional resourceGroup labels
				costLabels = AzureResourceTagManager.AddToPrometheusLabels(costLabels)
			}

			costLabels = append(costLabels, dimension.Label)
		}

		// add additional query labels
		for labelName := range query.Labels {
			costLabels = append(costLabels, labelName)
		}

		if query.Granularity == "Daily" || query.Granularity == "Monthly" {
			costLabels = append(costLabels, "date", "dateISO")
		}

		queryGaugeVec := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: query.GetMetricName(),
				Help: query.GetMetricHelp(),
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
	// run cost queries
	for _, row := range Config.Collectors.Costs.Queries {
		query := row

		exportType := armcostmanagement.ExportTypeActualCost
		if strings.EqualFold(query.ExportType, "AmortizedCost") {
			exportType = armcostmanagement.ExportTypeAmortizedCost
		}

		m.collectRunCostQuery(&query, exportType, callback)
	}
}

func (m *MetricsCollectorAzureRmCosts) collectRunCostQuery(query *config.CollectorCostsQuery, exportType armcostmanagement.ExportType, callback chan<- func()) {
	queryLogger := logger.With(zap.String("query", query.Name))
	for _, timeframe := range query.TimeFrames {
		timeframeLogger := queryLogger.With(zap.String("timeframe", timeframe))
		if query.Scopes != nil && len(*query.Scopes) > 0 {
			// using custom scope
			for _, scope := range *query.Scopes {
				m.collectCostManagementMetrics(
					timeframeLogger,
					m.Collector.GetMetricList(fmt.Sprintf(`query:%v`, query.Name)),
					scope,
					exportType,
					query,
					timeframe,
					nil,
				)
			}
		} else {
			// using subscription iterator
			iterator := AzureSubscriptionsIterator
			if query.Subscriptions != nil && len(*query.Subscriptions) > 0 {
				iterator = armclient.NewSubscriptionIterator(AzureClient, *query.Subscriptions...)
			}

			err := iterator.ForEach(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger) {
				subscriptionLogger := timeframeLogger.With(zap.String("subscriptionID", *subscription.SubscriptionID))
				m.collectCostManagementMetrics(
					subscriptionLogger,
					m.Collector.GetMetricList(fmt.Sprintf(`query:%v`, query.Name)),
					*subscription.ID,
					exportType,
					query,
					timeframe,
					subscription,
				)

			})
			if err != nil {
				m.Logger().Panic(err)
			}
		}
	}
}

func (m *MetricsCollectorAzureRmCosts) collectCostManagementMetrics(logger *zap.SugaredLogger, metricList *collector.MetricList, scope string, exportType armcostmanagement.ExportType, query *config.CollectorCostsQuery, timeframe string, subscription *armsubscriptions.Subscription) {
	logger.Infof(`fetching cost report for query "%v"`, query.Name)

	queryConfig := query.GetConfig()

	dimensionList := make([]*CostQueryConfigDimension, len(query.Dimensions))
	for i, dimension := range queryConfig.Dimensions {
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
	for i, row := range dimensionList {
		dimensionConfig := row
		queryGrouping[i] = &armcostmanagement.QueryGrouping{
			Name: &dimensionConfig.Name,
			Type: &dimensionConfig.Type,
		}
	}

	granularity := armcostmanagement.GranularityType(query.Granularity)
	timePeriod := armcostmanagement.QueryTimePeriod{}

	if query.TimePeriod != nil {
		if query.TimePeriod.From != nil {
			timePeriod.From = query.TimePeriod.From
		} else if query.TimePeriod.FromDuration != nil {
			now := time.Now()
			fromPeriod := now.Add(*query.TimePeriod.FromDuration)
			timePeriod.From = &fromPeriod
		}

		if query.TimePeriod.To != nil {
			timePeriod.To = query.TimePeriod.To
		} else if query.TimePeriod.ToDuration != nil {
			now := time.Now()
			toPeriod := now.Add(*query.TimePeriod.ToDuration)
			timePeriod.To = &toPeriod
		}
	}

	if timeframe == "Custom" && (timePeriod.From == nil || timePeriod.To == nil) {
		logger.Panic("If custom, then a specific time period must be provided.")
		return
	}

	timeframeType := armcostmanagement.TimeframeType(timeframe)

	aggregationFunction := armcostmanagement.FunctionTypeSum
	params := armcostmanagement.QueryDefinition{
		Dataset: &armcostmanagement.QueryDataset{
			Aggregation: map[string]*armcostmanagement.QueryAggregation{
				query.ValueField: {
					Name:     to.StringPtr(query.ValueField),
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

	if timeframe == "Custom" {
		params.TimePeriod = &timePeriod
	}

	result, err := m.sendCostQuery(m.Context(), logger, scope, params)
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
	columnNumberGranularityDate := -1
	costValueFieldName := strings.ToLower(query.ValueField)

	for num, col := range list.Columns {
		if col.Name == nil {
			continue
		}

		if query.Granularity == "Daily" && *col.Name == "UsageDate" {
			columnNumberGranularityDate = num
			continue
		} else if query.Granularity == "Monthly" && *col.Name == "BillingMonth" {
			columnNumberGranularityDate = num
			continue
		}

		switch stringToStringLower(*col.Name) {
		case costValueFieldName:
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
			"scope":          scope,
			"subscriptionID": "",
			"currency":       stringToStringLower(row[columnNumberCurrency].(string)),
			"timeframe":      timeframe,
			"granularity":    stringToStringLower(query.Granularity),
		}

		if subscription != nil {
			labels["subscriptionID"] = *subscription.SubscriptionID
		}

		if columnNumberGranularityDate != -1 {
			date := int64(0)
			dateISO := ""
			switch v := row[columnNumberGranularityDate].(type) {
			case float64:
				datetime, err := time.Parse("20060102", strconv.FormatFloat(v, 'g', 8, 64))
				if err != nil {
					logger.Errorf("Can't parse date %d", v)
				}
				date = datetime.Unix()
				dateISO = datetime.Format(time.RFC3339)
			case string:
				datetime, err := time.Parse("2006-01-02T00:00:00", v)
				if err != nil {
					logger.Errorf("Can't parse date %s", v)
				}
				date = datetime.Unix()
				dateISO = datetime.Format(time.RFC3339)
			}
			labels["date"] = strconv.FormatInt(date, 10)
			labels["dateISO"] = dateISO
		}

		for _, dimensionConfig := range dimensionList {
			labels[dimensionConfig.LabelName] = ""
			if row[dimensionConfig.ResultColumnNumber] != nil {
				labels[dimensionConfig.LabelName] = row[dimensionConfig.ResultColumnNumber].(string)

				switch dimensionConfig.LabelName {
				case "subscriptionName":
					if subscription != nil {
						labels[dimensionConfig.LabelName] = to.String(subscription.DisplayName)
					}
				case "resourceGroup":
					resourceId := ""
					resourceGroup := row[dimensionConfig.ResultColumnNumber].(string)
					if subscription != nil && resourceGroup != "" {
						// add resourceGroups labels using tag manager
						resourceId = fmt.Sprintf(
							"/subscriptions/%s/resourceGroups/%s",
							to.StringLower(subscription.SubscriptionID),
							resourceGroup,
						)
					}
					labels = AzureResourceGroupTagManager.AddResourceTagsToPrometheusLabels(m.Context(), labels, resourceId)
				case "resourceID":
					// add resource labels using tag manager
					labels = AzureResourceTagManager.AddResourceTagsToPrometheusLabels(m.Context(), labels, row[dimensionConfig.ResultColumnNumber].(string))
				}
			}
		}

		for labelName, labelValue := range query.Labels {
			labels[labelName] = labelValue
		}

		metricList.Add(labels, usage)
	}

	// avoid rate limit
	time.Sleep(Config.Collectors.Costs.RequestDelay)
}

func (m *MetricsCollectorAzureRmCosts) sendCostQuery(ctx context.Context, logger *zap.SugaredLogger, scope string, parameters armcostmanagement.QueryDefinition) (armcostmanagement.QueryClientUsageResponse, error) {
	clientOpts := AzureClient.NewArmClientOptions()

	// Initialize the client with appropriate retry options.
	clientOpts.Retry = policy.RetryOptions{
		MaxRetries:    3,
		RetryDelay:    30 * time.Second,
		MaxRetryDelay: 2 * time.Minute,
	}
	clientOpts.PerCallPolicies = append(clientOpts.PerCallPolicies, metrics.CostRateLimitPolicy{Logger: logger})

	client, err := armcostmanagement.NewQueryClient(AzureClient.GetCred(), clientOpts)
	if err != nil {
		logger.Panic(err.Error())
	}

	result, err := client.Usage(ctx, scope, parameters, nil)
	if err != nil {
		logger.Panic(err.Error())
	}

	// Set up the pipeline for paging.
	pl, err := armruntime.NewPipeline("azurerm-costs", gitTag, AzureClient.GetCred(), runtime.PipelineOptions{}, AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err.Error())
	}

	nextLink := result.Properties.NextLink
	for {
		if nextLink != nil && *nextLink != "" {
			err := func() error {
				req, err := runtime.NewRequest(ctx, http.MethodPost, *nextLink)
				if err != nil {
					return err
				}
				err = runtime.MarshalAsJSON(req, parameters)
				if err != nil {
					return err
				}

				resp, err := pl.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusTooManyRequests {
					retryAfterHeader := resp.Header.Get("X-Ms-Ratelimit-Microsoft.costmanagement-Entity-Retry-After")
					retryAfter, err := strconv.Atoi(retryAfterHeader)
					if err != nil {
						logger.Errorf("Unable to parse retry-after header: %v", retryAfterHeader)
						return fmt.Errorf("unable to parse retry-after header: %v", retryAfterHeader)
					}
					logger.Errorf("Received 429 Too Many Requests. Retrying after %d seconds. Headers: %v", retryAfter, resp.Header)
					time.Sleep(time.Duration(retryAfter) * time.Second)
					return fmt.Errorf("received 429 Too Many Requests, retrying after %d seconds", retryAfter)
				}

				if runtime.HasStatusCode(resp, http.StatusOK) {
					pagerResult := armcostmanagement.QueryClientUsageResponse{}
					if err := runtime.UnmarshalAsJSON(resp, &pagerResult); err == nil {
						result.Properties.Rows = append(result.Properties.Rows, pagerResult.Properties.Rows...)
						nextLink = pagerResult.Properties.NextLink
					} else {
						logger.Panic(err.Error())
					}
				} else {
					return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
				}

				return nil
			}()
			if err != nil {
				// If we encounter a rate limit error, retry after the specified delay.
				if strings.Contains(err.Error(), "received 429 Too Many Requests") {
					continue
				}
				return result, err
			}

		} else {
			break
		}
	}

	return result, err
}
