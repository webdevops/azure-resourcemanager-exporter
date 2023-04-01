package config

import (
	"encoding/json"
	"time"
)

type (
	Config struct {
		Azure      Azure `yaml:"azure"`
		Collectors struct {
			Exporter       CollectorBase  `yaml:"exporter"`
			General        CollectorBase  `yaml:"general"`
			Resource       CollectorBase  `yaml:"resource"`
			Quota          CollectorBase  `yaml:"quota"`
			Defender       CollectorBase  `yaml:"defender"`
			ResourceHealth CollectorBase  `yaml:"resourceHealth"`
			Iam            CollectorBase  `yaml:"iam"`
			Graph          CollectorGraph `yaml:"graph"`
			Costs          CollectorCosts `yaml:"costs"`
			Portscan       CollectorBase  `yaml:"portscan"`
		} `yaml:"collectors"`
	}

	Azure struct {
		Subscriptions []string `yaml:"subscriptions"`
		Locations     []string `yaml:"locations"`

		ResourceTags      []string `yaml:"resourceTags"`
		ResourceGroupTags []string `yaml:"resourceGroupTags"`
	}

	CollectorBase struct {
		ScrapeTime *time.Duration `yaml:"scrapeTime"`
		// Cron *string
	}

	CollectorGraph struct {
		CollectorBase `yaml:",inline"`

		Filter struct {
			Application      *string `yaml:"application"`
			ServicePrincipal *string `yaml:"servicePrincipal"`
		} `yaml:"filter"`
	}
)

func (c *CollectorBase) IsEnabled() bool {
	return c.ScrapeTime != nil && c.ScrapeTime.Seconds() >= 0
}

func (c *Config) GetJson() []byte {
	jsonBytes, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}
