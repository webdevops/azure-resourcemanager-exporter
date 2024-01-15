package config

type (
	CollectorReservation struct {
		CollectorBase `yaml:",inline"`

		ResourceScope string `yaml:"resourceScope"`
		Granularity   string `yaml:"granularity"`
	}
)
