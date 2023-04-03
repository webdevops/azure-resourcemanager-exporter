package config

type (
	CollectorResourceHealth struct {
		CollectorBase `yaml:",inline"`

		SummaryMaxLength uint `yaml:"summaryMaxLength"`
	}
)
