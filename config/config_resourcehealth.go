package config

type (
	CollectorResourceHealth struct {
		*CollectorBase

		SummaryMaxLength int `json:"summaryMaxLength"`
	}
)
