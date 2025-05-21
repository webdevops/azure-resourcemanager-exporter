package config

import (
	"fmt"
	"strings"
	"time"
)

type (
	CollectorCosts struct {
		*CollectorBase `yaml:",inline"`

		RequestDelay time.Duration `json:"requestDelay"`

		Queries []CollectorCostsQuery `json:"queries"`
	}

	CollectorCostsQuery struct {
		Name          string                         `json:"name"`
		Help          *string                        `json:"help"`
		Scopes        *[]string                      `json:"scopes"`
		Subscriptions *[]string                      `json:"subscriptions"`
		TimeFrames    []string                       `json:"timeFrames"`
		Dimensions    []string                       `json:"dimensions"`
		ExportType    string                         `json:"exportType"`
		Granularity   string                         `json:"granularity"`
		ValueField    string                         `json:"valueField"`
		Labels        map[string]string              `json:"labels"`
		TimePeriod    *CollectorCostsQueryTimePeriod `json:"timePeriod"`

		config *configCollectorCostsQueryConfig
	}
	CollectorCostsQueryTimePeriod struct {
		From         *time.Time     `json:"from"`
		FromDuration *time.Duration `json:"fromDuration"`
		To           *time.Time     `json:"to"`
		ToDuration   *time.Duration `json:"toDuration"`
	}

	configCollectorCostsQueryConfig struct {
		Dimensions []configCollectorCostsQueryConfigDimension
		ExportType string // Ajout du champ ExportType
	}

	configCollectorCostsQueryConfigDimension struct {
		Dimension string
		Label     string
	}
)

func (q *CollectorCostsQuery) GetMetricName() string {
	return fmt.Sprintf(`azurerm_costs_%v`, q.Name)
}

func (q *CollectorCostsQuery) GetMetricHelp() string {
	if q.Help != nil {
		return *q.Help
	} else {
		return fmt.Sprintf(`Azure ResourceManager costmanagement query with dimensions %v`, strings.Join(q.Dimensions, ", "))
	}
}

func (q *CollectorCostsQuery) GetConfig() *configCollectorCostsQueryConfig {
	if q.config == nil {
		q.config = &configCollectorCostsQueryConfig{
			Dimensions: []configCollectorCostsQueryConfigDimension{},
		}

		for _, dimension := range q.Dimensions {
			labelName := lowerFirst(prometheusLabelReplacerRegExp.ReplaceAllString(dimension, "_"))

			switch {
			case strings.EqualFold(dimension, "ResourceGroupName"):
				labelName = "resourceGroup"
			case strings.EqualFold(dimension, "ResourceId"):
				labelName = "resourceID"
			}

			q.config.Dimensions = append(
				q.config.Dimensions,
				configCollectorCostsQueryConfigDimension{
					Dimension: dimension,
					Label:     labelName,
				},
			)

			q.config.ExportType = q.ExportType
		}
	}

	return q.config
}
