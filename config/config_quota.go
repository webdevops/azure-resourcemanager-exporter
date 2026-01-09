package config

type (
	CollectorQuota struct {
		*CollectorBase `yaml:",inline"`

		ResourceProviders []CollectorQuotaResourceProvider `json:"resourceProviders"`
	}

	CollectorQuotaResourceProvider struct {
		Provider   string `json:"provider"`
		ApiVersion string `json:"apiVersion"`
	}
)
