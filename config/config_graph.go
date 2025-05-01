package config

type (
	CollectorGraph struct {
		*CollectorBase

		Filter struct {
			Application      *string `json:"application"`
			ServicePrincipal *string `json:"servicePrincipal"`
		} `json:"filter"`
	}
)
