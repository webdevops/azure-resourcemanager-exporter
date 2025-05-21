package config

type (
	CollectorResourceHealth struct {
		*CollectorBase `yaml:",inline"`

		SummaryMaxLength int `json:"summaryMaxLength"`
	}
)
