package config

type (
	CollectorReservation struct {
		CollectorBase `yaml:",inline"`

		Scopes      []string `yaml:"scopes"`
		Granularity string   `yaml:"granularity"`
		FromDays    int      `yaml:"fromDays"`
	}
)
