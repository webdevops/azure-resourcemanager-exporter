package config

type (
	CollectorBudgets struct {
		CollectorBase `yaml:",inline"`

		Scopes []string `yaml:"scopes"`
	}
)
