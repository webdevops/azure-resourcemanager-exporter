package config

import (
	"fmt"
	"strings"
	"time"
)

type (
	CollectorCosts struct {
		CollectorBase `yaml:",inline"`

		RequestDelay time.Duration `yaml:"requestDelay"`

		Queries []CollectorCostsQuery `yaml:"queries"`
	}

	CollectorCostsQuery struct {
		Name          string            `yaml:"name"`
		Help          *string           `yaml:"help"`
		Scopes        *[]string         `yaml:"scopes"`
		Subscriptions *[]string         `yaml:"subscriptions"`
		TimeFrames    []string          `yaml:"timeFrames"`
		Dimensions    []string          `yaml:"dimensions"`
		ValueField    string            `yaml:"valueField"`
		Labels        map[string]string `yaml:"labels"`

		config *configCollectorCostsQueryConfig
	}

	configCollectorCostsQueryConfig struct {
		Dimensions []configCollectorCostsQueryConfigDimension
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
		}
	}

	return q.config
}
