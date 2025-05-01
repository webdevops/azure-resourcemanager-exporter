package config

type (
	CollectorReservation struct {
		*CollectorBase

		Scopes      []string `json:"scopes"`
		Granularity string   `json:"granularity"`
		FromDays    int      `json:"fromDays"`
	}
)
