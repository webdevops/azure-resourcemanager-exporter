package config

import (
	"encoding/json"
	"time"
)

type (
	Config struct {
		Azure      Azure `yaml:"azure"`
		Collectors struct {
			General        CollectorBase           `yaml:"general"`
			Resource       CollectorBase           `yaml:"resource"`
			Quota          CollectorBase           `yaml:"quota"`
			Defender       CollectorBase           `yaml:"defender"`
			ResourceHealth CollectorResourceHealth `yaml:"resourceHealth"`
			Iam            CollectorBase           `yaml:"iam"`
			Graph          CollectorGraph          `yaml:"graph"`
			Costs          CollectorCosts          `yaml:"costs"`
			Budgets        CollectorBudgets        `yaml:"budgets"`
			Reservation    CollectorReservation    `yaml:"reservation"`
			Portscan       CollectorPortscan       `yaml:"portscan"`
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
)

func (c *CollectorBase) IsEnabled() bool {
	return c.ScrapeTime != nil && c.ScrapeTime.Seconds() > 0
}

func (c *Config) GetJson() []byte {
	jsonBytes, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}
