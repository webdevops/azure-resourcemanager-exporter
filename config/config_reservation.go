package config

type (
	CollectorReservation struct {
		*CollectorBase `yaml:",inline"`

		Scopes      []string `json:"scopes"`
		Granularity string   `json:"granularity"`
		FromDays    int      `json:"fromDays"`
	}
)
