package config

type (
	CollectorGraph struct {
		CollectorBase `yaml:",inline"`

		Filter struct {
			Application      *string `yaml:"application"`
			ServicePrincipal *string `yaml:"servicePrincipal"`
		} `yaml:"filter"`
	}
)
