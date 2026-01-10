package config

import (
	"encoding/json"
)

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

func (rp *CollectorQuotaResourceProvider) UnmarshalJSON(data []byte) error {
	var (
		valString           string
		valResourceProvider struct {
			Provider   string `json:"provider"`
			ApiVersion string `json:"apiVersion"`
		}
	)

	// try string first
	if err := json.Unmarshal(data, &valString); err == nil {
		rp.Provider = valString
		rp.ApiVersion = "auto"
		return nil
	}

	// try full version
	err := json.Unmarshal(data, &valResourceProvider)
	if err != nil {
		return err
	}
	rp.Provider = valResourceProvider.Provider
	rp.ApiVersion = valResourceProvider.ApiVersion
	return nil
}
