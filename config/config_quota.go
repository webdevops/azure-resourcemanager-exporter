package config

type (
	CollectorQuota struct {
		*CollectorBase `yaml:",inline"`

		ResourceProviders []string `json:"resourceProviders"`
	}
)
