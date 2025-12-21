package config

import (
	"encoding/json"
	"time"
)

type (
	Config struct {
		Azure      Azure `json:"azure"`
		Collectors struct {
			General        CollectorBase           `json:"general"`
			Resource       CollectorBase           `json:"resource"`
			Quota          CollectorBase           `json:"quota"`
			Advisor        CollectorAdvisor        `json:"advisor"`
			Defender       CollectorBase           `json:"defender"`
			ResourceHealth CollectorResourceHealth `json:"resourceHealth"`
			Iam            CollectorBase           `json:"iam"`
			Graph          CollectorGraph          `json:"graph"`
			Costs          CollectorCosts          `json:"costs"`
			Budgets        CollectorBudgets        `json:"budgets"`
			Reservation    CollectorReservation    `json:"reservation"`
			Portscan       CollectorPortscan       `json:"portscan"`
		} `json:"collectors"`
	}

	Azure struct {
		Subscriptions []string `json:"subscriptions"`
		Locations     []string `json:"locations"`

		ResourceTags      []string `json:"resourceTags"`
		ResourceGroupTags []string `json:"resourceGroupTags"`
	}

	CollectorBase struct {
		ScrapeTime *time.Duration `json:"scrapeTime"`
		// Cron *string
	}
)

func (c *CollectorBase) IsEnabled() bool {
	if c == nil {
		return false
	}

	return c.ScrapeTime != nil && c.ScrapeTime.Seconds() > 0
}

func (c *Config) GetJson() []byte {
	jsonBytes, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}
