package config

type (
	CollectorGraph struct {
		*CollectorBase `yaml:",inline"`

		Filter struct {
			Application      *string `json:"application"`
			ServicePrincipal *string `json:"servicePrincipal"`
		} `json:"filter"`
	}
)
